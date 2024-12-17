package example

import akka.actor.ActorSystem

import scala.concurrent.ExecutionContext

object Producer extends App {
  private val actorSystem: ActorSystem = ActorSystem("ExampleActorSystem")
  implicit private val ec: ExecutionContext = actorSystem.dispatcher

  println("Producer / start")
  private val client = sergiusd.redbus.Client("localhost", 50005)
  client.produce("topic-1", "message-1".getBytes, withKeyUUIDv4 = true)
  client.close()
  println("Producer / finish")
}