package sergiusd.redbus.consumer

import java.util.concurrent.atomic.AtomicBoolean

import scala.concurrent.{ExecutionContext, Future, Promise, TimeoutException}
import scala.util.control.NonFatal

/** Arbitrates between a final handler failure and the processing deadline for one delivery. */
private[consumer] final class LastAttemptFailure(
  notify: (Array[Byte], Model.MessageMeta, Throwable) => Future[Unit],
  logError: Throwable => Unit,
)(implicit ec: ExecutionContext) {
  // Some(error) means the handler failed before the deadline. None means processing timed out first.
  private val handlerFailure = Promise[Option[Throwable]]()
  private val notificationFinished = new AtomicBoolean(false)

  def wrap(processor: Model.Processor): Model.Processor = (data, meta) => {
    val processing = try processor(data, meta) catch { case NonFatal(e) => Future.failed(e) }
    processing.recoverWith {
      case NonFatal(error) if !error.isInstanceOf[RetryLaterException] &&
        handlerFailure.trySuccess(Some(error)) && !notificationFinished.get() =>
        val notification = try notify(data, meta, error) catch { case NonFatal(e) => Future.failed(e) }
        notification.transformWith { result =>
          if (notificationFinished.compareAndSet(false, true)) {
            result.failed.foreach(logError)
          }
          Future.failed(error)
        }
    }
  }

  def onTimeout(): Future[Either[Throwable, Unit]] = {
    handlerFailure.trySuccess(None)
    handlerFailure.future.flatMap {
      case Some(error) =>
        if (notificationFinished.compareAndSet(false, true)) {
          logError(new TimeoutException("Callback exceeded the processing deadline"))
        }
        Future.successful(Left(error))
      case None => Future.failed(new TimeoutException("Future timed out"))
    }
  }
}
