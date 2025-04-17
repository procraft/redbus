package sergiusd.redbus.consumer

import sergiusd.redbus.PostgresDriver

import java.time.Instant
import PostgresDriver.api._
import slick.dbio.Effect

import java.sql.Timestamp

case class IncomeMessage(
  key: String,
  createdAt: Timestamp = Timestamp.from(Instant.now),
)

class IncomeMessages(tag: Tag) extends Table[IncomeMessage](tag, Some("public"), "redbus_inbox") {
  def key = column[String]("key")
  def createdAt = column[Timestamp]("created_at")

  def * = (key, createdAt).<>((IncomeMessage.apply _).tupled, IncomeMessage.unapply)
}

object IncomeMessages extends TableQuery(new IncomeMessages(_)) {
  def setProcessed(group: String, topic: String, idempotencyKey: String): DBIOAction[Int, NoStream, Effect.Write] = {
    IncomeMessages += IncomeMessage(gecCompositeKey(group, topic, idempotencyKey))
  }

  def isProcessed(group: String, topic: String, idempotencyKey: String): DBIOAction[Boolean, PostgresDriver.api.NoStream, Effect.Read] = {
    val key = gecCompositeKey(group, topic, idempotencyKey)
    IncomeMessages.filter(x => x.key === key).map(_ => ()).exists.result
  }

  private def gecCompositeKey(group: String, topic: String, idempotencyKey: String): String = {
    s"$group|$topic|$idempotencyKey"
  }
}
