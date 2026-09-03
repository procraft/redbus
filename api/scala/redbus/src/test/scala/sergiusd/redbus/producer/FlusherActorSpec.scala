package sergiusd.redbus.producer

import akka.actor.{ActorSystem, Props}
import akka.testkit.TestKit
import org.scalatest.BeforeAndAfterAll
import org.scalatest.concurrent.Eventually
import org.scalatest.matchers.should.Matchers
import org.scalatest.time.{Millis, Seconds, Span}
import org.scalatest.wordspec.AnyWordSpecLike
import sergiusd.redbus.api

import java.util.concurrent.atomic.AtomicInteger
import scala.concurrent.{Future, Promise}

class FlusherActorSpec
  extends TestKit(ActorSystem("FlusherActorSpec"))
  with AnyWordSpecLike
  with Matchers
  with Eventually
  with BeforeAndAfterAll {

  implicit override val patienceConfig: PatienceConfig =
    PatienceConfig(timeout = Span(5, Seconds), interval = Span(50, Millis))

  override def afterAll(): Unit = TestKit.shutdownActorSystem(system)

  private class InMemoryStore(initial: Seq[PublishingMessage]) extends Flusher.Store {
    @volatile var rows: Seq[PublishingMessage] = initial
    val fetches = new AtomicInteger(0)

    override def fetchAll(): Future[Seq[PublishingMessage]] = {
      fetches.incrementAndGet()
      Future.successful(rows.sortBy(_.id))
    }

    override def delete(id: Long): Future[Int] = {
      rows = rows.filterNot(_.id == id)
      Future.successful(1)
    }
  }

  private def message(id: Long): PublishingMessage =
    PublishingMessage("topic", s"payload-$id".getBytes, PublishingMessage.Options.empty, id)

  "FlusherActor" should {

    "run another pass after the current one when a trigger arrives while in progress" in {
      val store = new InMemoryStore(Seq(message(1)))
      val firstProduce = Promise[api.ProduceResponse]()
      val produced = new AtomicInteger(0)
      val produce: api.ProduceRequest => Future[api.ProduceResponse] = _ => {
        if (produced.incrementAndGet() == 1) firstProduce.future
        else Future.successful(api.ProduceResponse(ok = true))
      }
      val actor = system.actorOf(Props(new FlusherActor(store, produce, _ => ())))

      actor ! ProcessMessage("1")
      eventually(produced.get() shouldBe 1)

      // A row inserted while the first pass is blocked on produce, plus its notification.
      store.rows = store.rows :+ message(2)
      actor ! ProcessMessage("2")
      store.fetches.get() shouldBe 1

      firstProduce.success(api.ProduceResponse(ok = true))

      eventually {
        store.fetches.get() shouldBe 2
        store.rows shouldBe empty
      }
      produced.get() shouldBe 2
    }

    "keep the row and stay alive when produce fails" in {
      val store = new InMemoryStore(Seq(message(1)))
      val attempts = new AtomicInteger(0)
      val produce: api.ProduceRequest => Future[api.ProduceResponse] = _ => {
        if (attempts.incrementAndGet() == 1) Future.failed(new RuntimeException("bus unavailable"))
        else Future.successful(api.ProduceResponse(ok = true))
      }
      val logged = new AtomicInteger(0)
      val logger: String => Unit = _ => { logged.incrementAndGet(); () }
      val actor = system.actorOf(Props(new FlusherActor(store, produce, logger)))

      actor ! ProcessMessage("1")
      eventually(attempts.get() shouldBe 1)
      eventually(logged.get() should be >= 1)
      store.rows.map(_.id) shouldBe Seq(1L)

      actor ! ProcessMessage("sweep")
      eventually {
        attempts.get() shouldBe 2
        store.rows shouldBe empty
      }
    }
  }
}
