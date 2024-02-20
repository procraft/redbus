package example

object Producer extends App {
  println("Producer / start")
  private val client = sergiusd.redbus.Client("localhost", 50005)
  client.produce("topic-1", "message-1".getBytes)
  client.close()
  println("Producer / finish")
}