package sergiusd.redbus.producer

import java.sql.Connection
import slick.jdbc.PostgresProfile.backend.Database
import org.apache.pekko.actor.{Actor, ActorLogging, ActorRef, ActorSystem, Props}
import org.postgresql.PGConnection

import scala.concurrent.duration._
import scala.util.{Try, Using}
import scala.util.control.NonFatal

case object CheckNotifications

class PostgresListenerActor private[producer] (
  db: Database,
  callback: String => Unit,
  onSubscribed: () => Unit,
) extends Actor with ActorLogging {
  def this(db: Database, callback: String => Unit) = this(db, callback, () => ())

  private var connection: scala.Option[(Connection, PGConnection)] = None
  private var retryAt: scala.Option[Deadline] = None

  private def connect(): PGConnection = {
    val conn = db.source.createConnection()
    try {
      val pgConn = conn.unwrap(classOf[PGConnection])
      Using.resource(conn.createStatement())(_.execute(s"LISTEN ${PostgresListener.channel}"))
      connection = Some((conn, pgConn))
      pgConn
    } catch {
      case NonFatal(error) =>
        Try(conn.close())
        throw error
    }
  }

  private def disconnect(): Unit = {
    connection.foreach { case (conn, _) => Try(conn.close()) }
    connection = None
  }

  override def postStop(): Unit = disconnect()

  override def receive: Receive = {
    case CheckNotifications if retryAt.forall(_.isOverdue()) =>
      try {
        val subscribing = connection.isEmpty
        val pgConn = connection.fold(connect())(_._2)
        val notifications = pgConn.getNotifications()
        if (subscribing) {
          if (retryAt.nonEmpty) log.info("RED Bus LISTEN restored")
          retryAt = None
          onSubscribed()
        }
        if (notifications != null) {
          notifications.foreach(n => callback(n.getParameter))
        }
      } catch {
        case NonFatal(error) =>
          disconnect()
          if (retryAt.isEmpty) log.warning("RED Bus LISTEN failed; reconnecting: {}", error.getMessage)
          retryAt = Some(1.second.fromNow)
      }
    case CheckNotifications => ()
  }
}

object PostgresListener {
  val channel = "redbus_outbox"
  private val interval = 100.millis

  def listen(db: Database)(callback: String => Unit)(implicit system: ActorSystem): ActorRef =
    listen(db, () => ())(callback)

  private[producer] def listen(
    db: Database,
    onSubscribed: () => Unit,
  )(callback: String => Unit)(implicit system: ActorSystem): ActorRef = {
    val actor = system.actorOf(Props(new PostgresListenerActor(db, callback, onSubscribed)), "redbusPostgresListener")

    system.scheduler.scheduleAtFixedRate(0.millis, interval)(() => {
      actor ! CheckNotifications
    })(system.dispatcher)

    actor
  }
}