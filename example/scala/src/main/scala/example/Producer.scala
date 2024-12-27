package example

import akka.actor.ActorSystem
import sergiusd.redbus.producer

import scala.concurrent.ExecutionContext

object Producer extends App {
  private val actorSystem: ActorSystem = ActorSystem("ExampleActorSystem")
  implicit private val ec: ExecutionContext = actorSystem.dispatcher

  println("Producer / start")
  private val client = sergiusd.redbus.Client("localhost", 50005)
  client.produce("topic-1", "message-1".getBytes, producer.Option.WithKeyUUIDv4())
  client.close()
  println("Producer / finish")
}