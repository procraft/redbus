package sergiusd.redbus.producer

import play.api.libs.json.{Json, OFormat}
import slick.lifted.Tag

import java.time.Instant
import sergiusd.redbus.PostgresDriver.api._

import java.sql.Timestamp

case class PublishingMessage(
  topic: String,
  message: Array[Byte],
  options: PublishingMessage.Options,
  id: Long = 0,
)

object PublishingMessage {

  case class Options(
    key: Option[String],
    version: Option[Long],
    idempotencyKey: Option[String],
    timestamp: Option[String],
  )

  object Options {
    val empty: Options = Options(
      key = None,
      version = None,
      idempotencyKey = None,
      timestamp = None,
    )

    implicit val fmt: OFormat[Options] = Json.format
  }

}

class PublishingMessages(tag: Tag) extends Table[PublishingMessage](tag, Some("public"), "redbus_outbox") {
  def topic = column[String]("topic")
  def message = column[Array[Byte]]("message")
  def options = column[PublishingMessage.Options]("options")
  def createdAt = column[Timestamp]("created_at")
  def id = column[Long]("id", O.PrimaryKey, O.AutoInc)

  def * = (topic, message, options, createdAt, id).<>(
      { case (t: String, m: Array[Byte], o: PublishingMessage.Options, _, id: Long) => PublishingMessage(t, m, o, id) },
      { pm: PublishingMessage => Some((pm.topic, pm.message, pm.options, Timestamp.from(Instant.now), pm.id)) },
    )
}

object PublishingMessages extends TableQuery(new PublishingMessages(_))