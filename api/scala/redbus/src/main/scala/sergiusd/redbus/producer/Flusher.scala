package sergiusd.redbus.producer

import akka.actor.{Actor, ActorSystem, Props}
import com.google.protobuf.ByteString
import slick.jdbc.PostgresProfile.api._
import sergiusd.redbus.api

import java.util.concurrent.Executors
import scala.concurrent.{ExecutionContext, Future}

private case class ProcessMessage(data: String)

class FlusherActor(
  db: slick.jdbc.PostgresProfile.backend.Database,
  produce: api.ProduceRequest => Future[api.ProduceResponse],
  logger: String => Unit = _ => (),
) extends Actor {
  import Flusher.ec
  private var inProgress = false

  override def receive: Receive = {
    case ProcessMessage(data) =>
      if (!inProgress) {
        inProgress = true
        processMessages(data).andThen(_ => inProgress = false)
      }
    case x => logger(s"Unknown message $x")
  }

  private def processMessages(data: String): Future[Unit] = {
    for {
      messages <- db.run(PublishingMessages.result)
      _ <- runSeq(messages) { message =>
        for {
          _ <- produce(api.ProduceRequest(
            message.topic,
            message.options.key.getOrElse(""),
            ByteString.copyFrom(message.message),
            message.options.idempotencyKey.getOrElse(""),
            message.options.timestamp.getOrElse(""),
            message.options.version.getOrElse(message.id),
          ))
          _ <- db.run(PublishingMessages.filter(_.id === message.id).delete)
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

  def start(
    db: slick.jdbc.PostgresProfile.backend.Database,
    produce: api.ProduceRequest => Future[api.ProduceResponse],
    logger: String => Unit = _ => (),
  )(implicit as: ActorSystem): Unit = {
    val dispatcher = as.actorOf(Props(new FlusherActor(db, produce, logger)), "redbusFlusherActor")

    PostgresListener.listen(db) { id => dispatcher ! ProcessMessage(id) }
  }
}