package sergiusd.redbus.consumer

import sergiusd.redbus.api.ConsumeRequest
import slick.dbio.{DBIOAction, Effect, NoStream}
import slick.jdbc.PostgresProfile.backend.Database
import java.time.ZonedDateTime
import scala.concurrent.Future
import scala.concurrent.duration.FiniteDuration

object Model {

  private type MessageData = Array[Byte]
  /**
   * @param claimDba set only in the `Option.WithTransactionalInbox` mode: the inbox claim for this
   *                 message ([[IncomeMessages.claim]]). The processor runs it as the first step of its
   *                 own transaction; on `false` it skips the business logic and completes successfully.
   * @param lastAttempt an ordinary failure would exhaust the bus retry budget
   * @param idempotencyKey the producer's key, or the bus message ID when the producer supplied none
   */
  case class MessageMeta(
    version: Option[MessageVersion] = None,
    timestamp: Option[MessageTimestamp] = None,
    claimDba: Option[InboxClaim] = None,
    lastAttempt: Boolean = false,
    idempotencyKey: Option[MessageIdempotencyKey] = None,
  )

  type InboxClaim = DBIOAction[Boolean, NoStream, Effect.Write]

  private type MessageVersion = Long
  type MessageIdempotencyKey = String
  type MessageTimestamp = ZonedDateTime

  type Processor = (MessageData, MessageMeta) => Future[Unit]
  type StopHook = (() => Future[_]) => Unit

  case class Listener(
    consumeTimeout: FiniteDuration,
    repeatStrategy: Option[RepeatStrategy] = None,
    batchSize: Int,
    unavailableTimeout: FiniteDuration,
    logger: String => Unit = _ => (),
    checkEventProcessedDatabase: Option[Database] = None,
    transactionalInbox: Boolean = false,
    onLastAttemptFailure: Option[(Array[Byte], MessageMeta, Throwable) => Future[Unit]] = None,
    errorLogger: Option[(String, Throwable) => Unit] = None,
  )

  class RepeatStrategy(
    maxAttempts: Int,
    evenStrategy: Option[RepeatStrategyEven] = None,
    progressiveStrategy: Option[RepeatStrategyProgressive] = None,
  ) {
    def toPB: ConsumeRequest.Connect.RepeatStrategy = {
      ConsumeRequest.Connect.RepeatStrategy(
        maxAttempts,
        evenStrategy.map(x =>
          ConsumeRequest.Connect.RepeatStrategy.EvenConfig(
            intervalSec = x.intervalSec,
          )
        ),
        progressiveStrategy.map(x =>
          ConsumeRequest.Connect.RepeatStrategy.ProgressiveConfig(
            intervalSec = x.intervalSec,
            multiplier = x.multiplier,
          )
        ),
      )
    }
  }

  case class RepeatStrategyEven(
    intervalSec: Int,
  )

  case class RepeatStrategyProgressive(
    intervalSec: Int,
    multiplier: Float,
  )

}
