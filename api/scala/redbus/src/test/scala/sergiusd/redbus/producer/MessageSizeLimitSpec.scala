package sergiusd.redbus.producer

import org.scalatest.BeforeAndAfterAll
import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec
import sergiusd.redbus.Client
import slick.dbio.{DBIOAction, FailureAction}

import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext}

class MessageSizeLimitSpec extends AnyWordSpec with Matchers with BeforeAndAfterAll {

  implicit private val ec: ExecutionContext = ExecutionContext.global

  private val bus = new HangingBus
  bus.hanging = false

  override def afterAll(): Unit = bus.close()

  private def payload(size: Int): Array[Byte] = Array.fill(size)(1.toByte)

  private def tooLargeOf(action: DBIOAction[_, _, _]): MessageTooLargeException = action match {
    case FailureAction(e: MessageTooLargeException) => e
    case other => fail(s"expected DBIO.failed(MessageTooLargeException), got $other")
  }

  "Producer.defaultMaxMessageBytes" should {
    "be 256 KiB" in {
      Producer.defaultMaxMessageBytes shouldBe 262144
    }
  }

  "Producer.tooLarge" should {
    "accept a payload of exactly the limit and reject one byte more" in {
      Producer.tooLarge("topic", payload(10), 10) shouldBe None
      Producer.tooLarge("topic", payload(11), 10) shouldBe Some(MessageTooLargeException("topic", 11, 10))
    }

    "reject a non-positive limit" in {
      an[IllegalArgumentException] should be thrownBy Producer.tooLarge("topic", payload(1), 0)
    }
  }

  "Producer.produceWithLimits" should {
    "send a payload of exactly the limit" in {
      val calls = bus.produceCalls.get()
      Await.result(Producer.produceWithLimits(bus.stub, 5.seconds, 10, "topic", payload(10)), 10.seconds) shouldBe true
      bus.produceCalls.get() shouldBe calls + 1
    }

    "fail a larger payload without calling the bus" in {
      val calls = bus.produceCalls.get()
      val e = the[MessageTooLargeException] thrownBy Await.result(
        Producer.produceWithLimits(bus.stub, 5.seconds, 10, "topic", payload(11)), 10.seconds
      )
      e shouldBe MessageTooLargeException("topic", 11, 10)
      e.getMessage should include("topic topic is 11 bytes, above the limit of 10 bytes")
      bus.produceCalls.get() shouldBe calls
    }

    "apply the default limit through produceWithTimeout" in {
      val calls = bus.produceCalls.get()
      val limit = Producer.defaultMaxMessageBytes
      Await.result(Producer.produceWithTimeout(bus.stub, 5.seconds, "topic", payload(limit)), 10.seconds) shouldBe true
      the[MessageTooLargeException] thrownBy Await.result(
        Producer.produceWithTimeout(bus.stub, 5.seconds, "topic", payload(limit + 1)), 10.seconds
      ) shouldBe MessageTooLargeException("topic", limit + 1, limit)
      bus.produceCalls.get() shouldBe calls + 1
    }
  }

  "Producer.produceDba" should {
    "insert a payload of exactly the default limit" in {
      Producer.produceDba("topic", payload(Producer.defaultMaxMessageBytes)) should not be a[FailureAction]
    }

    "fail the action, without an insert, for one byte more" in {
      val limit = Producer.defaultMaxMessageBytes
      tooLargeOf(Producer.produceDba("topic", payload(limit + 1))) shouldBe
        MessageTooLargeException("topic", limit + 1, limit)
    }

    "apply an explicit limit" in {
      Producer.produceDbaWithLimit("topic", payload(10), 10) should not be a[FailureAction]
      tooLargeOf(Producer.produceDbaWithLimit("topic", payload(11), 10)) shouldBe MessageTooLargeException("topic", 11, 10)
    }
  }

  "Client" should {
    "apply its maxMessageBytes to the outbox without connecting" in {
      val client = Client("localhost", 1, maxMessageBytes = 10)
      client.produceDba("topic", payload(10)) should not be a[FailureAction]
      tooLargeOf(client.produceDba("topic", payload(11))) shouldBe MessageTooLargeException("topic", 11, 10)
    }

    "reject a non-positive maxMessageBytes" in {
      an[IllegalArgumentException] should be thrownBy Client("localhost", 1, maxMessageBytes = 0)
    }
  }
}
