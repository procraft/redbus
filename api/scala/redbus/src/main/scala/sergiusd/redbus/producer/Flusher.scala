package sergiusd.redbus.producer

import org.apache.pekko.actor.{Actor, ActorSystem, Props}
import com.google.protobuf.ByteString
import slick.jdbc.PostgresProfile.api._
import sergiusd.redbus.api

import java.util.concurrent.Executors
import scala.concurrent.duration._
import scala.concurrent.{ExecutionContext, Future}
import scala.util.{Failure, Success}

private case class ProcessMessage(data: String)
private case object ProcessingFinished

/**
 * Drains the `redbus_outbox` table into the bus.
 *
 * A pass is triggered either by a `pg_notify` from the outbox trigger or by the periodic sweep
 * scheduled in [[Flusher.start]]. Only one pass runs at a time; a trigger that arrives while a
 * pass is in progress is remembered (`pending`) and starts another pass right after the current
 * one finishes, so no notification is lost.
 *
 * Order is kept only within a topic. A pass repeatedly fetches up to `batchSize` rows in `id` order,
 * leaving out topics that failed earlier in the pass, and sends the rows of each topic as one batch
 * (topics in the order of their first row), deleting each batch after it is confirmed. A failed topic
 * (produce failure, `ok = false`, delete failure or mismatch) keeps its rows and is excluded for the
 * rest of the pass, so one rejected topic does not stop the others; it is retried on the next
 * trigger or sweep. A failed fetch ends the pass.
 *
 * `logger` gets routine diagnostics (each flushed batch); `errorLogger` gets a report with the
 * cause for every failed topic and failed fetch, at most once per `errorLogInterval` for each of
 * them; the next report after a quiet interval states how many were suppressed.
 */
class FlusherActor private[producer] (
  store: Flusher.Store,
  produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
  logger: String => Unit,
  errorLogger: Flusher.ErrorLogger,
  batchSize: Int,
  errorLogInterval: FiniteDuration,
  clockNanos: () => Long,
) extends Actor {
  import Flusher.ec
  require(batchSize > 0, "batchSize must be positive")

  private[producer] def this(
    store: Flusher.Store,
    produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
    logger: String => Unit,
    errorLogger: Flusher.ErrorLogger,
    batchSize: Int,
  ) = this(store, produceBatch, logger, errorLogger, batchSize, Flusher.defaultErrorLogInterval, () => System.nanoTime())

  private[producer] def this(
    store: Flusher.Store,
    produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
    logger: String => Unit,
    batchSize: Int,
  ) = this(store, produceBatch, logger, Flusher.errorsToLogger(logger), batchSize, Flusher.defaultErrorLogInterval,
    () => System.nanoTime())

  def this(
    db: Database,
    produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
    logger: String => Unit = _ => (),
    batchSize: Int = Flusher.defaultBatchSize,
  ) = this(new Flusher.SlickStore(db), produceBatch, logger, Flusher.errorsToLogger(logger), batchSize,
    Flusher.defaultErrorLogInterval, () => System.nanoTime())

  private val errors = new Flusher.ErrorThrottle(errorLogInterval, clockNanos)

  private var inProgress = false
  private var pending = false

  override def receive: Receive = {
    case ProcessMessage(data) =>
      if (inProgress) {
        pending = true
      } else {
        startProcessing(data)
      }
    case ProcessingFinished =>
      inProgress = false
      if (pending) {
        pending = false
        startProcessing("pending")
      }
    case x => logger(s"Unknown message $x")
  }

  // Actor state is touched only from `receive`: the future completion reports back via `self`.
  private def startProcessing(data: String): Unit = {
    inProgress = true
    processMessages(data, Set.empty).onComplete {
      case Success(_) =>
        self ! ProcessingFinished
      case Failure(e) =>
        reportError("", s"Flush failed ($data), rows stay in outbox until the next pass", e)
        self ! ProcessingFinished
    }
  }

  private def reportError(key: String, message: String, cause: Throwable): Unit =
    errors.report(key) { suppressed =>
      val suffix = if (suppressed > 0) s" ($suppressed more suppressed since the previous report)" else ""
      errorLogger(message + suffix, cause)
    }

  // Every step either deletes rows or adds each topic of its selection to `failed`, and a failed
  // topic is never selected again in the pass, so the pass always ends.
  private def processMessages(data: String, failed: Set[String]): Future[Unit] = {
    store.fetchBatch(batchSize, failed).flatMap { fetched =>
      if (fetched.isEmpty) Future.unit
      else {
        val groups = fetched.groupBy(_.topic).toSeq.sortBy(_._2.head.id)
        groups.foldLeft(Future.successful(failed)) { case (acc, (topic, messages)) =>
          acc.flatMap { failedSoFar =>
            flushBatch(topic, messages).transform {
              case Success(_) => Success(failedSoFar)
              case Failure(e) =>
                reportError(topic, s"Flush failed ($data) for topic $topic, its rows stay in outbox until the next pass", e)
                Success(failedSoFar + topic)
            }
          }
        }.flatMap(processMessages(data, _))
      }
    }
  }

  private def flushBatch(topic: String, messages: Seq[PublishingMessage]): Future[Unit] = {
    val ids = messages.map(_.id)
    val request = api.ProduceBatchRequest(
      topic = topic,
      messageList = messages.map(message => api.ProduceBatchMessage(
        message.options.key.getOrElse(""),
        ByteString.copyFrom(message.message),
        message.options.idempotencyKey.getOrElse(""),
        message.options.timestamp.getOrElse(""),
        message.options.version.getOrElse(message.id),
      )),
    )
    for {
      response <- produceBatch(request)
      _ <- if (response.ok) Future.unit else Future.failed(
        new IllegalStateException(s"Bus rejected batch $topic / ${ids.mkString(",")}")
      )
      deleted <- store.deleteBatch(ids)
      _ <- if (deleted == ids.size) Future.unit else Future.failed(
        new IllegalStateException(s"Deleted $deleted of ${ids.size} flushed outbox rows")
      )
    } yield logger(s"Flushed batch $topic / ${ids.mkString(",")}")
  }
}

object Flusher {
  implicit val ec: ExecutionContext = ExecutionContext.fromExecutor(Executors.newSingleThreadExecutor())

  /** Default interval of the periodic outbox sweep. */
  val defaultSweepInterval: FiniteDuration = 30.seconds
  /** Default maximum number of outbox rows fetched in one query and batch request. */
  val defaultBatchSize: Int = 100
  /** Minimum interval between two error reports for one topic (or for failed fetches). */
  val defaultErrorLogInterval: FiniteDuration = 1.minute

  /** Receives a failed flush pass: a message and its cause. */
  type ErrorLogger = (String, Throwable) => Unit

  /**
   * Error sink used when none is given: the report goes to the plain `logger` with the cause
   * appended, as before error sinks existed.
   */
  def errorsToLogger(logger: String => Unit): ErrorLogger = (message, cause) => logger(s"$message: $cause")

  /**
   * Lets one report per key through every `interval` and counts the ones suppressed in between.
   * A steady insert rate triggers a pass per `pg_notify`, so an unthrottled error sink would repeat
   * the same failure on every pass.
   */
  private[producer] final class ErrorThrottle(interval: FiniteDuration, clockNanos: () => Long) {
    private var lastReported = Map.empty[String, Long]
    private var suppressed = Map.empty[String, Int]

    def report(key: String)(emit: Int => Unit): Unit = {
      val emitted = synchronized {
        val now = clockNanos()
        lastReported.get(key) match {
          case Some(last) if now - last < interval.toNanos =>
            suppressed = suppressed.updated(key, suppressed.getOrElse(key, 0) + 1)
            None
          case _ =>
            val count = suppressed.getOrElse(key, 0)
            lastReported = lastReported.updated(key, now)
            suppressed = suppressed - key
            Some(count)
        }
      }
      emitted.foreach(emit)
    }
  }

  /**
   * Outbox storage used by [[FlusherActor]]. `fetchBatch` returns up to `batchSize` rows whose topic
   * is not in `failedTopics`, in `id` order.
   */
  trait Store {
    def fetchBatch(batchSize: Int, failedTopics: Set[String]): Future[Seq[PublishingMessage]]
    def deleteBatch(ids: Seq[Long]): Future[Int]
  }

  class SlickStore(db: Database) extends Store {
    // Without failed topics this is the plain `ORDER BY id LIMIT n` over the primary key.
    override def fetchBatch(batchSize: Int, failedTopics: Set[String]): Future[Seq[PublishingMessage]] = {
      val candidates =
        if (failedTopics.isEmpty) PublishingMessages
        else PublishingMessages.filterNot(_.topic.inSetBind(failedTopics))
      db.run(candidates.sortBy(_.id).take(batchSize).result)
    }

    override def deleteBatch(ids: Seq[Long]): Future[Int] =
      db.run(PublishingMessages.filter(_.id.inSetBind(ids)).delete)
  }

  /**
   * Starts the outbox flusher: listens to `pg_notify('redbus_outbox')` and additionally sweeps
   * the table every `sweepInterval`, starting immediately, so rows left over from a restart or
   * a missed notification are still delivered.
   *
   * @param errorLogger receives every failed topic and failed fetch with its cause, at most once per
   *                    [[defaultErrorLogInterval]] for each; without it the report goes to `logger`,
   *                    the same as before this parameter existed
   */
  def start(
    db: Database,
    produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
    logger: String => Unit = _ => (),
    sweepInterval: FiniteDuration = defaultSweepInterval,
    batchSize: Int = defaultBatchSize,
    errorLogger: scala.Option[ErrorLogger] = None,
  )(implicit as: ActorSystem): Unit = {
    require(batchSize > 0, "batchSize must be positive")
    val errors = errorLogger.getOrElse(errorsToLogger(logger))
    val dispatcher = as.actorOf(
      Props(new FlusherActor(new SlickStore(db), produceBatch, logger, errors, batchSize)),
      "redbusFlusherActor",
    )

    PostgresListener.listen(db) { id => dispatcher ! ProcessMessage(id) }

    as.scheduler.scheduleAtFixedRate(Duration.Zero, sweepInterval)(() => {
      dispatcher ! ProcessMessage("sweep")
    })(as.dispatcher)
  }
}
