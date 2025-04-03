package sergiusd.redbus.producer

import java.sql.Connection
import slick.jdbc.PostgresProfile.backend.Database
import akka.actor.{Actor, ActorRef, ActorSystem, Props}
import org.postgresql.PGConnection

import scala.concurrent.duration._

case object CheckNotifications

class PostgresListenerActor(db: Database, callback: String => Unit) extends Actor {
  private val conn: Connection = db.source.createConnection()
  private val pgConn: PGConnection = conn.unwrap(classOf[PGConnection])

  override def preStart(): Unit = {
    val stmt = conn.createStatement()
    stmt.execute(s"LISTEN ${PostgresListener.channel}")
    stmt.close()
  }

  override def receive: Receive = {
    case CheckNotifications =>
      val notifications = pgConn.getNotifications()
      if (notifications != null) {
        notifications.foreach(n => callback(n.getParameter))
      }
  }
}

object PostgresListener {
  val channel = "redbus_outbox"
  private val interval = 100.millis

  def listen(db: Database)(callback: String => Unit)(implicit system: ActorSystem): ActorRef = {
    val actor = system.actorOf(Props(new PostgresListenerActor(db, callback)), "redbusPostgresListener")

    system.scheduler.scheduleAtFixedRate(0.millis, interval)(() => {
      actor ! CheckNotifications
    })(system.dispatcher)

    actor
  }
}