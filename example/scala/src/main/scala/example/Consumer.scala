package example

import akka.actor.ActorSystem

import java.nio.charset.StandardCharsets
import scala.concurrent.{Await, ExecutionContext, Future}
import sun.misc.Signal

import scala.concurrent.duration.Duration

object Consumer extends App {

  private val actorSystem: ActorSystem = ActorSystem("ExampleActorSystem")
  implicit private val ec: ExecutionContext = actorSystem.dispatcher

  private var stopHook: Option[() => Future[_]] = None
  Signal.handle(new Signal("TERM"), (signal: Signal) => {
    println(s"Received signal: ${signal.getName}")
    shutdown()
  })

  private def processor(bytes: Array[Byte]): Future[Either[String, Unit]] = {
    println(s"Consumer / message: ${new String(bytes, StandardCharsets.UTF_8)}")
    Future.successful(Right(()))
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
  private val client = sergiusd.redbus.Client("localhost", 50005)
  private val consumeFuture = client.consume("topic-1", "group-1", processor, addStopHook).map(_ =>
    println("Consumer / finish")
  )
  Await.result(consumeFuture, Duration.Inf)

  actorSystem.terminate()
}