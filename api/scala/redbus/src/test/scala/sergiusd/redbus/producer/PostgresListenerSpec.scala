package sergiusd.redbus.producer

import org.apache.pekko.actor.ActorSystem
import org.apache.pekko.testkit.{TestKit, TestProbe}
import org.postgresql.{PGConnection, PGNotification}
import org.postgresql.core.Notification
import org.postgresql.ds.PGSimpleDataSource
import org.scalatest.BeforeAndAfterAll
import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpecLike
import slick.jdbc.PostgresProfile.api._

import java.lang.reflect.Proxy
import java.sql.{Connection, SQLException, Statement}
import java.util.concurrent.atomic.AtomicInteger
import scala.concurrent.duration._

class PostgresListenerSpec
  extends TestKit(ActorSystem("PostgresListenerSpec"))
  with AnyWordSpecLike
  with Matchers
  with BeforeAndAfterAll {

  override def afterAll(): Unit = TestKit.shutdownActorSystem(system)

  private def proxy[A](iface: Class[A])(call: PartialFunction[(String, Array[AnyRef]), AnyRef]): A =
    iface.cast(Proxy.newProxyInstance(
      iface.getClassLoader,
      Array[Class[_]](iface),
      (_, method, args) =>
        call.applyOrElse((method.getName, args), (_: (String, Array[AnyRef])) =>
          throw new UnsupportedOperationException(method.getName)),
    ))

  private class Session(failListen: Boolean, poll: () => Array[PGNotification]) {
    val closed = new AtomicInteger()
    private val subscribed = new AtomicInteger()

    private val statement = proxy(classOf[Statement]) {
      case ("execute", args) if args(0) == s"LISTEN ${PostgresListener.channel}" =>
        if (failListen) throw new SQLException("LISTEN failed", "08006")
        subscribed.incrementAndGet()
        java.lang.Boolean.FALSE
      case ("close", _) => null
    }

    private val pg = proxy(classOf[PGConnection]) {
      case ("getNotifications", _) if subscribed.get() == 1 => poll()
    }

    val connection: Connection = proxy(classOf[Connection]) {
      case ("createStatement", _) => statement
      case ("unwrap", args) if args(0) == classOf[PGConnection] => pg
      case ("close", _) =>
        closed.incrementAndGet()
        null
    }
  }

  private def notification(id: String): Array[PGNotification] =
    Array(new Notification(PostgresListener.channel, 1, id))

  "PostgresListener" should {
    "signal one catch-up sweep per successful LISTEN, including the first subscription" in {
      val events = TestProbe()
      val firstPolls = new AtomicInteger()
      val first = new Session(false, () =>
        if (firstPolls.getAndIncrement() == 0) notification("21")
        else throw new SQLException("connection lost", "08006"),
      )
      val failed = new Session(true, () => null)
      val lastPolls = new AtomicInteger()
      val last = new Session(false, () =>
        if (lastPolls.getAndIncrement() == 0) notification("22") else null,
      )
      val sessions = Iterator(first, failed, last)
      val source = new PGSimpleDataSource {
        override def getConnection: Connection = sessions.next().connection
      }
      val db = Database.forDataSource(source, maxConnections = None)
      val actor = PostgresListener.listen(db, () => events.ref ! "subscribed") { message =>
        events.ref ! message
      }
      try {
        events.expectMsg("subscribed")
        events.expectMsg("21")
        events.expectMsg(6.seconds, "subscribed")
        events.expectMsg("22")
        events.expectNoMessage(250.millis)
        first.closed.get() shouldBe 1
        failed.closed.get() shouldBe 1
      } finally {
        events.watch(actor)
        system.stop(actor)
        events.expectTerminated(actor)
        db.close()
      }
      last.closed.get() shouldBe 1
    }
  }
}
