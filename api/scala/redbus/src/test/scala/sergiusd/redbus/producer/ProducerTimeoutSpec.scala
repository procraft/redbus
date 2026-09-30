package sergiusd.redbus.producer

import io.grpc.Status
import org.scalatest.BeforeAndAfterAll
import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec
import sergiusd.redbus.api

import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext}

class ProducerTimeoutSpec extends AnyWordSpec with Matchers with BeforeAndAfterAll {

  implicit private val ec: ExecutionContext = ExecutionContext.global

  private val bus = new HangingBus

  override def afterAll(): Unit = bus.close()

  "Producer" should {

    "fail a produce the bus does not answer within the timeout" in {
      val result = Producer.produceWithTimeout(bus.stub, 300.millis, "topic", "payload".getBytes)

      val e = the[ProduceTimeoutException] thrownBy Await.result(result, 10.seconds)
      e.topic shouldBe "topic"
      e.timeout shouldBe 300.millis
      Status.fromThrowable(e.cause).getCode shouldBe Status.Code.DEADLINE_EXCEEDED
      bus.produceCalls.get() shouldBe 1
    }

    "fail a batch the bus does not answer within the timeout" in {
      val request = api.ProduceBatchRequest(topic = "batch-topic", messageList = Seq(api.ProduceBatchMessage()))

      val e = the[ProduceTimeoutException] thrownBy Await.result(
        Producer.produceBatch(bus.stub, request, 300.millis), 10.seconds
      )
      e.topic shouldBe "batch-topic"
    }

    "reject a non-positive timeout" in {
      an[IllegalArgumentException] should be thrownBy {
        Producer.produceWithTimeout(bus.stub, Duration.Zero, "topic", Array.emptyByteArray)
      }
    }

    "keep other gRPC failures as they are" in {
      val closed = new HangingBus
      closed.close()

      val e = intercept[Throwable](Await.result(
        Producer.produceWithTimeout(closed.stub, 5.seconds, "topic", Array.emptyByteArray), 10.seconds
      ))
      e should not be a[ProduceTimeoutException]
    }
  }
}
