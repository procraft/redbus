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
 * one finishes, so no notification is lost. A produce failure ends the pass, keeps the row in
 * the table and is retried on the next trigger or sweep.
 *
 * `logger` gets routine diagnostics (each flushed batch); `errorLogger` gets one report with the
 * cause for every failed pass (fetch, produce, `ok = false`, delete mismatch).
 */
class FlusherActor private[producer] (
  store: Flusher.Store,
  produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
  logger: String => Unit,
  errorLogger: Flusher.ErrorLogger,
  batchSize: Int,
) extends Actor {
  import Flusher.ec
  require(batchSize > 0, "batchSize must be positive")

  private[producer] def this(
    store: Flusher.Store,
    produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
    logger: String => Unit,
    batchSize: Int,
  ) = this(store, produceBatch, logger, Flusher.errorsToLogger(logger), batchSize)

  def this(
    db: Database,
    produceBatch: api.ProduceBatchRequest => Future[api.ProduceBatchResponse],
    logger: String => Unit = _ => (),
    batchSize: Int = Flusher.defaultBatchSize,
  ) = this(new Flusher.SlickStore(db), produceBatch, logger, Flusher.errorsToLogger(logger), batchSize)

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
    processMessages(data).onComplete {
      case Success(_) =>
        self ! ProcessingFinished
      case Failure(e) =>
        errorLogger(s"Flush failed ($data), rows stay in outbox until the next pass", e)
        self ! ProcessingFinished
    }
  }

  private def processMessages(data: String): Future[Unit] = {
    store.fetchBatch(batchSize).flatMap { fetched =>
      fetched.headOption match {
        case None => Future.unit
        case Some(first) =>
          val messages = fetched.takeWhile(_.topic == first.topic)
          val ids = messages.map(_.id)
          val request = api.ProduceBatchRequest(
            topic = first.topic,
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
              new IllegalStateException(s"Bus rejected batch ${first.topic} / ${ids.mkString(",")}")
            )
            deleted <- store.deleteBatch(ids)
            _ <- if (deleted == ids.size) Future.unit else Future.failed(
              new IllegalStateException(s"Deleted $deleted of ${ids.size} flushed outbox rows")
            )
            _ = logger(s"Flushed batch ${first.topic} / ${ids.mkString(",")}")
            _ <- processMessages(data)
          } yield ()
      }
    }
  }
}

object Flusher {
  implicit val ec: ExecutionContext = ExecutionContext.fromExecutor(Executors.newSingleThreadExecutor())

  /** Default interval of the periodic outbox sweep. */
  val defaultSweepInterval: FiniteDuration = 30.seconds
  /** Default maximum number of outbox rows fetched in one query and batch request. */
  val defaultBatchSize: Int = 100

  /** Receives a failed flush pass: a message and its cause. */
  type ErrorLogger = (String, Throwable) => Unit

  /**
   * Error sink used when none is given: the report goes to the plain `logger` with the cause
   * appended, as before error sinks existed.
   */
  def errorsToLogger(logger: String => Unit): ErrorLogger = (message, cause) => logger(s"$message: $cause")

  /** Outbox storage used by [[FlusherActor]]; rows are returned in `id` order. */
  trait Store {
    def fetchBatch(batchSize: Int): Future[Seq[PublishingMessage]]
    def deleteBatch(ids: Seq[Long]): Future[Int]
  }

  class SlickStore(db: Database) extends Store {
    override def fetchBatch(batchSize: Int): Future[Seq[PublishingMessage]] =
      db.run(PublishingMessages.sortBy(_.id).take(batchSize).result)

    override def deleteBatch(ids: Seq[Long]): Future[Int] =
      db.run(PublishingMessages.filter(_.id.inSetBind(ids)).delete)
  }

  /**
   * Starts the outbox flusher: listens to `pg_notify('redbus_outbox')` and additionally sweeps
   * the table every `sweepInterval`, starting immediately, so rows left over from a restart or
   * a missed notification are still delivered.
   *
   * @param errorLogger receives every failed pass with its cause; without it the report goes to
   *                    `logger`, the same as before this parameter existed
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
