package sergiusd.redbus.consumer

import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec

import scala.concurrent.duration._

class ResultConverterSpec extends AnyWordSpec with Matchers {

  "ResultConverter" should {
    "mark a retry-later result as deferred and round its delay up to seconds" in {
      val result = ResultConverter.toPB(
        "message-id",
        Left(RetryLaterException(new RuntimeException("provider throttled"), 1500.millis)),
      )

      result.ok shouldBe false
      result.message shouldBe "provider throttled"
      result.retryLater shouldBe true
      result.retryAfterSec shouldBe 2
    }

    "keep ordinary failures backward compatible" in {
      val result = ResultConverter.toPB("message-id", Left(new RuntimeException("failed")))

      result.ok shouldBe false
      result.retryLater shouldBe false
      result.retryAfterSec shouldBe 0
    }
  }
}
