package sergiusd.redbus.producer

import play.api.libs.json.{Json, OFormat}
import slick.lifted.Tag

import java.time.ZonedDateTime
import sergiusd.redbus.producer.PostgresDriver.api._

case class PublishingMessage(
  topic: String,
  message: Array[Byte],
  options: PublishingMessage.Options,
  id: Long = 0,
)

object PublishingMessage {

  case class Options(
    key: Option[String],
    idempotencyKey: Option[String],
    timestamp: Option[String],
  )

  object Options {
    val empty: Options = Options(
      key = None,
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
  private def createdAt = column[ZonedDateTime]("created_at")
  def id = column[Long]("id", O.PrimaryKey, O.AutoInc)

  def * = (topic, message, options, createdAt).<>(
    { case (t, m, o, _) => PublishingMessage(t, m, o) },
    { pm: PublishingMessage => Some((pm.topic, pm.message, pm.options, ZonedDateTime.now)) },
  )
}

object PublishingMessages extends TableQuery(new PublishingMessages(_))