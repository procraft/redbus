package example

import scala.concurrent.{ExecutionContext, Future}

object Producer extends App {
  println("producer")
}

class Producer()(implicit ec: ExecutionContext) {

}