package example

import akka.actor.ActorSystem
import sergiusd.redbus
import sergiusd.redbus.producer
import slick.jdbc.PostgresProfile.api._

import scala.concurrent.{Await, ExecutionContext, Future, Promise}
import io.github.cdimascio.dotenv.Dotenv

import scala.concurrent.duration.{Duration, DurationInt, FiniteDuration}
import scala.util.Try

object ProducerOutbox extends App {
  private val dotenv = Dotenv.configure().directory(".").load()
  private val db = Database.forURL(
    url = dotenv.get("DB_URL"),
    user = dotenv.get("DB_USER"),
    password = dotenv.get("DB_PASSWORD"),
    driver = "org.postgresql.Driver"
  )

  private implicit val actorSystem: ActorSystem = ActorSystem("ExampleActorSystem")
  private implicit val ec: ExecutionContext = actorSystem.dispatcher

  if (args.length != 2) {
    println("""Usage: sbt "runMain example.Producer topic-1 \\"Message text\\""""")
    System.exit(1)
  }

  private val topic = args(0)
  private val message = args(1)
  private val key = if (args.length > 2) args(2) else ""

  println("Producer / start")
  val client = sergiusd.redbus.Client("localhost", 50005, println(_))
  var options: Seq[redbus.producer.Option.Fn] = Nil
  if (key.nonEmpty) {
    options :+= redbus.producer.Option.WithKey(key)
  }

  private val produceTickerFuture: Future[Unit] = {

    def tick(n: Int): Future[Unit] = {
      for {
        affected <- db.run(producer.Producer.produceDba(topic, s"$message ($n)".getBytes, options: _*))
      } yield {
        println(s"Producer | tick: $n ($affected record affected)")
      }
    }

    for {
      _ <- runSeq(Seq.range(1, 5), 1.second)(tick)
      _ = println("All message sent to database, wait  some time for produce")
      _ <- delay(20.seconds)
      _ = client.close()
    } yield ()
  }

  // start flusher in background
  Future {
    client.startProducerDbaFlusher(db)
  }

  Await.result(produceTickerFuture, Duration.Inf)
  actorSystem.terminate()

  // helpers

  private def runSeq[T, U](items: Iterable[T], delayDuration: FiniteDuration)(futureProvider: T => Future[U]): Future[List[U]] = {
    items.foldLeft(Future.successful[List[U]](Nil)) {
      (f, item) => f.flatMap {
        x => (if (x.isEmpty) Future.unit else delay(delayDuration)).flatMap(_ => futureProvider(item).map(_ :: x))
      }
    } map (_.reverse)
  }

  private def delay(duration: FiniteDuration): Future[Unit] = {
    val pr = Promise[Unit]()
    actorSystem.scheduler.scheduleOnce(duration) {
      pr.complete(Try(()))
    }
    pr.future
  }
}