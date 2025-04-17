package example

import akka.actor.ActorSystem

import java.nio.charset.StandardCharsets
import scala.concurrent.{Await, ExecutionContext, Future}
import sun.misc.Signal
import sergiusd.redbus

import scala.concurrent.duration.Duration

object Consumer extends App {

  private val actorSystem: ActorSystem = ActorSystem("ExampleActorSystem")
  implicit private val ec: ExecutionContext = actorSystem.dispatcher

  if (args.length != 2) {
    println("""Usage: sbt "runMain example.Consumer topic-1 group-1"""")
    System.exit(1)
  }

  private val topic = args(0)
  private val group = args(1)

  private var stopHook: Option[() => Future[_]] = None
  Signal.handle(new Signal("TERM"), (signal: Signal) => {
    println(s"Received signal: ${signal.getName}")
    shutdown()
  })

  private def processor(bytes: Array[Byte], meta: redbus.consumer.Model.MessageMeta): Future[Unit] = {
    val items = Seq(
      Some(new String(bytes, StandardCharsets.UTF_8)),
      meta.version.map(x => s"version: $x"),
      meta.timestamp.map(x => s"timestamp: $x"),
    ).flatten
    println(s"Consumer / message: ${items.mkString(", ")}")
    Future.unit
  }

  private def addStopHook(hook: () => Future[_]): Unit = {
    stopHook = Some(hook)
  }

  private def shutdown(): Unit = {
    println("Performing cleanup before shutdown...")
    stopHook.getOrElse(() => Future.unit)().map { _ =>
      client.close()
      System.exit(0)
    }
  }

  println("Consumer / start")
  private val client = redbus.Client("localhost", 50005, println(_))
  private val consumeFuture = client.consume(
    topic, group, processor, addStopHook,
    redbus.consumer.Option.WithBatchSize(3),
  ).map(_ =>
    println("Consumer / finish")
  )

  Await.result(consumeFuture, Duration.Inf)
  actorSystem.terminate()
}