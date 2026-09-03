package sergiusd.redbus.producer

import akka.actor.{Actor, ActorSystem, Props}
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
 */
class FlusherActor private[producer] (
  store: Flusher.Store,
  produce: api.ProduceRequest => Future[api.ProduceResponse],
  logger: String => Unit,
) extends Actor {
  import Flusher.ec

  def this(
    db: Database,
    produce: api.ProduceRequest => Future[api.ProduceResponse],
    logger: String => Unit = _ => (),
  ) = this(new Flusher.SlickStore(db), produce, logger)

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
        logger(s"Flush failed ($data), rows stay in outbox until the next pass: $e")
        self ! ProcessingFinished
    }
  }

  private def processMessages(data: String): Future[Unit] = {
    for {
      messages <- store.fetchAll()
      _ <- runSeq(messages) { message =>
        for {
          response <- produce(api.ProduceRequest(
            message.topic,
            message.options.key.getOrElse(""),
            ByteString.copyFrom(message.message),
            message.options.idempotencyKey.getOrElse(""),
            message.options.timestamp.getOrElse(""),
            message.options.version.getOrElse(message.id),
          ))
          _ <- if (response.ok) Future.unit else Future.failed(
            new IllegalStateException(s"Bus rejected message ${message.topic} / ${message.id}")
          )
          _ <- store.delete(message.id)
          _ = logger(s"Flushed message ${message.topic} / ${message.id}")
        } yield ()
      }
    } yield ()
  }

  private def runSeq[T, U](items: Iterable[T])(futureProvider: T => Future[U])(implicit ec: ExecutionContext): Future[List[U]] = {
    items.foldLeft(Future.successful[List[U]](Nil)) {
      (f, item) => f.flatMap {
        x => Future.unit.flatMap(_ => futureProvider(item).map(_ :: x))
      }
    } map (_.reverse)
  }
}

object Flusher {
  implicit val ec: ExecutionContext = ExecutionContext.fromExecutor(Executors.newSingleThreadExecutor())

  /** Default interval of the periodic outbox sweep. */
  val defaultSweepInterval: FiniteDuration = 30.seconds

  /** Outbox storage used by [[FlusherActor]]; rows are returned in `id` order. */
  trait Store {
    def fetchAll(): Future[Seq[PublishingMessage]]
    def delete(id: Long): Future[Int]
  }

  class SlickStore(db: Database) extends Store {
    override def fetchAll(): Future[Seq[PublishingMessage]] =
      db.run(PublishingMessages.sortBy(_.id).result)

    override def delete(id: Long): Future[Int] =
      db.run(PublishingMessages.filter(_.id === id).delete)
  }

  /**
   * Starts the outbox flusher: listens to `pg_notify('redbus_outbox')` and additionally sweeps
   * the table every `sweepInterval`, starting immediately, so rows left over from a restart or
   * a missed notification are still delivered.
   */
  def start(
    db: Database,
    produce: api.ProduceRequest => Future[api.ProduceResponse],
    logger: String => Unit = _ => (),
    sweepInterval: FiniteDuration = defaultSweepInterval,
  )(implicit as: ActorSystem): Unit = {
    val dispatcher = as.actorOf(Props(new FlusherActor(db, produce, logger)), "redbusFlusherActor")

    PostgresListener.listen(db) { id => dispatcher ! ProcessMessage(id) }

    as.scheduler.scheduleAtFixedRate(Duration.Zero, sweepInterval)(() => {
      dispatcher ! ProcessMessage("sweep")
    })(as.dispatcher)
  }
}
