package sergiusd.redbus.consumer

import slick.jdbc.JdbcBackend
import slick.jdbc.PostgresProfile.backend.Database
import scala.concurrent.duration.FiniteDuration

object Option {

  type Fn = Model.Listener => Model.Listener

  case class EventKey(
    topic: String,
    group: String,
    idempotencyKey: Model.MessageIdempotencyKey,
    timestamp: Model.MessageTimestamp,
  )

  def WithConsumeTimeout(consumeTimeout: FiniteDuration): Fn = {
    consumer => consumer.copy(consumeTimeout = consumeTimeout)
  }

  def WithUnavailableTimeout(unavailableTimeout: FiniteDuration): Fn = {
    consumer => consumer.copy(unavailableTimeout = unavailableTimeout)
  }


  def WithRepeatStrategyEven(maxAttempts: Int, intervalSec: Int): Fn = {
    consumer => consumer.copy(repeatStrategy = Some(new Model.RepeatStrategy(
      maxAttempts =  maxAttempts,
      evenStrategy = Some(new Model.RepeatStrategyEven(intervalSec = intervalSec)),
    )))
  }

  def WithRepeatStrategyProgressive(maxAttempts: Int, intervalSec: Int, multiplier: Float): Fn = {
    consumer => consumer.copy(repeatStrategy = Some(new Model.RepeatStrategy(
      maxAttempts = maxAttempts,
      progressiveStrategy = Some(new Model.RepeatStrategyProgressive(intervalSec = intervalSec, multiplier = multiplier)),
    )))
  }

  def WithBatchSize(batchSize: Int): Fn = {
    consumer => consumer.copy(batchSize = batchSize)
  }

  /**
   * Inbox dedup in separate steps: skip a message already marked in `redbus_inbox`, run the
   * processor, then write the mark after success. A crash between the processor's commit and the
   * mark, or a concurrent redelivery, can process the message twice. Replaces
   * [[WithTransactionalInbox]] when both are given (the last option wins).
   */
  def WithOnlyOnceProcessor(db: Database): Fn = WithInbox(db, InboxMode.OnlyOnce)

  /**
   * Transactional inbox: skip a message already marked in `redbus_inbox` (a cheap pre-check), but
   * never write the mark. Instead the processor receives `MessageMeta.claimDba` and must run it as
   * the first step of the same transaction as its business writes; on `false` it skips the business
   * logic and completes successfully. The mark uses the same table and key as
   * [[WithOnlyOnceProcessor]], so the modes are interchangeable. Replaces [[WithOnlyOnceProcessor]]
   * when both are given (the last option wins).
   */
  def WithTransactionalInbox(db: Database): Fn = WithInbox(db, InboxMode.Transactional)

  /**
   * Selects the inbox dedup mode of the consumer in one option (see [[InboxMode]]). Accepts the
   * database of any Slick `JdbcProfile`, including an application's own `PostgresProfile` subclass.
   * Replaces any inbox option given before it.
   */
  def WithInbox(db: JdbcBackend#JdbcDatabaseDef, mode: InboxMode): Fn = {
    val inboxDb = sergiusd.redbus.JdbcDatabases.postgres(db)
    mode match {
      case InboxMode.OnlyOnce =>
        consumer => consumer.copy(checkEventProcessedDatabase = Some(inboxDb), transactionalInbox = false)
      case InboxMode.Transactional =>
        consumer => consumer.copy(checkEventProcessedDatabase = Some(inboxDb), transactionalInbox = true)
      case InboxMode.Disabled =>
        consumer => consumer.copy(checkEventProcessedDatabase = None, transactionalInbox = false)
    }
  }

  private[redbus] def withLogger(logger: String => Unit): Fn = {
    consumer => consumer.copy(logger = logger)
  }
}