package sergiusd.redbus.consumer

import slick.jdbc.PostgresProfile.backend.Database

import scala.concurrent.{ExecutionContext, Future}

/**
 * Inbox dedup around one processor call. Kept apart from the gRPC stream so both inbox modes are
 * unit tested without a bus or a database.
 */
private[redbus] object InboxProcessing {

  trait Store {
    def isProcessed(group: String, topic: String, idempotencyKey: String): Future[Boolean]
    def setProcessed(group: String, topic: String, idempotencyKey: String): Future[Unit]
  }

  final class SlickStore(db: Database)(implicit ec: ExecutionContext) extends Store {
    override def isProcessed(group: String, topic: String, idempotencyKey: String): Future[Boolean] =
      db.run(IncomeMessages.isProcessed(group, topic, idempotencyKey))

    override def setProcessed(group: String, topic: String, idempotencyKey: String): Future[Unit] =
      db.run(IncomeMessages.setProcessed(group, topic, idempotencyKey)).map(_ => ())
  }

  /**
   * @param store         inbox store; `None` disables dedup entirely
   * @param transactional `true` hands the claim to the processor instead of writing the mark
   * @param meta          evaluated only when the message is going to be processed
   */
  def process(
    store: Option[Store],
    transactional: Boolean,
    group: String,
    topic: String,
    idempotencyKey: String,
    data: Array[Byte],
    meta: => Model.MessageMeta,
    processor: Model.Processor,
    log: String => Unit,
  )(implicit ec: ExecutionContext): Future[Either[Throwable, Unit]] = {
    val inbox = store.filter(_ => idempotencyKey.nonEmpty)
    for {
      isProcessed <- inbox match {
        case Some(s) => s.isProcessed(group, topic, idempotencyKey)
        case None => Future.successful(false)
      }
      result <- if (isProcessed) {
        log(s"Skip already processed message $group / $topic / $idempotencyKey")
        Future.successful(Right(()))
      } else {
        val claimDba = if (transactional && inbox.isDefined) {
          Some(IncomeMessages.claim(group, topic, idempotencyKey))
        } else None
        for {
          result <- processor(data, meta.copy(claimDba = claimDba))
            .map(_ => Right(()))
            .recover(e => Left(e))
          _ <- (result, inbox) match {
            case (Right(_), Some(s)) if !transactional => s.setProcessed(group, topic, idempotencyKey)
            case _ => Future.unit
          }
        } yield result
      }
    } yield result
  }
}
