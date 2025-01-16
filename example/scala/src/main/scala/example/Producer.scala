package example

import akka.actor.ActorSystem
import sergiusd.redbus

import scala.concurrent.duration.Duration
import scala.concurrent.{Await, ExecutionContext}

object Producer extends App {
  private val actorSystem: ActorSystem = ActorSystem("ExampleActorSystem")
  implicit private val ec: ExecutionContext = actorSystem.dispatcher

  if (args.length != 2) {
    println("""Usage: sbt "runMain example.Producer topic-1 \\"Message text\\""""")
    System.exit(1)
  }

  private val topic = args(0)
  private val message = args(1)
  private val key = if (args.length > 2) args(2) else ""

  private val produceFuture = {
    println("Producer / start")
    val client = sergiusd.redbus.Client("localhost", 50005)
    var options: Seq[redbus.producer.Option.Fn] = Nil
    if (key.nonEmpty) {
      options :+= redbus.producer.Option.WithKey(key)
    }
    client.produce(topic, message.getBytes, options: _*).map { ok =>
      client.close()
      println(s"Producer / finish: ${if (ok) "Ok" else "Fail"}")
    }
  }

  Await.result(produceFuture, Duration.Inf)
  actorSystem.terminate()
}