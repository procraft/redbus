package sergiusd.redbus.consumer

import sergiusd.redbus.PostgresDriver

import java.time.Instant
import PostgresDriver.api._
import slick.dbio.Effect

import java.sql.Timestamp
import scala.concurrent.ExecutionContext

case class IncomeMessage(
  key: String,
  createdAt: Timestamp = Timestamp.from(Instant.now),
)

class IncomeMessages private[consumer] (tag: Tag, schemaName: String) extends Table[IncomeMessage](tag, Some(schemaName), "redbus_inbox") {
  def this(tag: Tag) = this(tag, IncomeMessages.DefaultSchema)

  def key = column[String]("key")
  def createdAt = column[Timestamp]("created_at")

  def * = (key, createdAt).<>((IncomeMessage.apply _).tupled, IncomeMessage.unapply)
}

object IncomeMessages extends TableQuery(new IncomeMessages(_)) {
  private[consumer] final val DefaultSchema = "public"

  def setProcessed(group: String, topic: String, idempotencyKey: String): DBIOAction[Int, NoStream, Effect.Write] =
    setProcessedIn(DefaultSchema, group, topic, idempotencyKey)

  /**
   * Transactional-inbox claim: inserts the processed mark with the same key as [[setProcessed]]
   * and yields `true` only when this call inserted the row (`INSERT … ON CONFLICT DO NOTHING`).
   * Run it as the first step of the transaction that holds the business writes: a concurrent claim
   * of the same key waits for that transaction and then yields `false`, and a rollback releases the
   * key together with the business writes. `false` means another delivery already processed the
   * message, so the caller skips its business logic and reports success.
   *
   * `created_at` is written explicitly, like [[setProcessed]] does: client tables are not guaranteed
   * to carry the `DEFAULT` from `api/inbox.sql`.
   */
  def claim(group: String, topic: String, idempotencyKey: String): DBIOAction[Boolean, NoStream, Effect.Write] =
    claimIn(DefaultSchema, group, topic, idempotencyKey)

  def isProcessed(group: String, topic: String, idempotencyKey: String): DBIOAction[Boolean, NoStream, Effect.Read] =
    isProcessedIn(DefaultSchema, group, topic, idempotencyKey)

  // The schema parameter exists only so the PostgreSQL spec can work in a throwaway schema.

  private[consumer] def setProcessedIn(
    schema: String,
    group: String,
    topic: String,
    idempotencyKey: String,
  ): DBIOAction[Int, NoStream, Effect.Write] =
    in(schema) += IncomeMessage(gecCompositeKey(group, topic, idempotencyKey))

  private[consumer] def claimIn(
    schema: String,
    group: String,
    topic: String,
    idempotencyKey: String,
  ): DBIOAction[Boolean, NoStream, Effect.Write] = {
    val key = gecCompositeKey(group, topic, idempotencyKey)
    sqlu"""INSERT INTO #$schema.redbus_inbox ("key", created_at) VALUES ($key, now()) ON CONFLICT ("key") DO NOTHING"""
      .map(_ == 1)(ExecutionContext.parasitic)
  }

  private[consumer] def isProcessedIn(
    schema: String,
    group: String,
    topic: String,
    idempotencyKey: String,
  ): DBIOAction[Boolean, NoStream, Effect.Read] = {
    val key = gecCompositeKey(group, topic, idempotencyKey)
    in(schema).filter(x => x.key === key).map(_ => ()).exists.result
  }

  private def in(schema: String): TableQuery[IncomeMessages] =
    if (schema == DefaultSchema) this else TableQuery(new IncomeMessages(_, schema))

  private def gecCompositeKey(group: String, topic: String, idempotencyKey: String): String = {
    s"$group|$topic|$idempotencyKey"
  }
}
