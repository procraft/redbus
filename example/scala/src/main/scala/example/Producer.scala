package example

object Producer extends App {
  private val client = sergiusd.redbus.Client("localhost", 50005)
  client.produce("test-1", "message-1".getBytes)
}