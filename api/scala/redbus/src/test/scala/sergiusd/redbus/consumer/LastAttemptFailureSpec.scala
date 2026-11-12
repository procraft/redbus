package sergiusd.redbus.consumer

import org.scalatest.matchers.should.Matchers
import org.scalatest.wordspec.AnyWordSpec

import java.util.concurrent.atomic.AtomicInteger
import scala.concurrent.duration._
import scala.concurrent.{Await, ExecutionContext, Future, Promise, TimeoutException}

class LastAttemptFailureSpec extends AnyWordSpec with Matchers {
  implicit private val ec: ExecutionContext = ExecutionContext.global

  private val failure = new IllegalArgumentException("handler failed")
  private val meta = Model.MessageMeta(lastAttempt = true, idempotencyKey = Some("command"))
  private val bytes = Array.emptyByteArray

  private def await[T](future: Future[T]): T = Await.result(future, 3.seconds)

  "A final attempt failure" should {
    "preserve the original error when the callback fails" in {
      val called = Promise[(Model.MessageMeta, Throwable)]()
      val logged = Promise[Throwable]()
      val callbackFailure = new IllegalStateException("callback failed")
      val hook = new LastAttemptFailure((_, metadata, error) => {
        called.success(metadata -> error)
        Future.failed(callbackFailure)
      }, error => { logged.success(error); () })

      await(hook.wrap((_, _) => Future.failed(failure))(bytes, meta).failed) shouldBe failure
      await(called.future) shouldBe (meta -> failure)
      await(logged.future) shouldBe callbackFailure
    }

    "notify on a synchronous ordinary handler failure" in {
      val called = Promise[Throwable]()
      val hook = new LastAttemptFailure((_, _, error) => {
        called.success(error)
        Future.unit
      }, _ => ())

      await(hook.wrap((_, _) => throw failure)(bytes, meta).failed) shouldBe failure
      await(called.future) shouldBe failure
    }

    "skip the callback for success and a deferred result" in {
      val deferred = RetryLaterException(failure, 2.seconds)
      val calls = new AtomicInteger(0)
      val hook = new LastAttemptFailure((_, _, _) => {
        calls.incrementAndGet()
        Future.unit
      }, _ => ())

      await(hook.wrap((_, _) => Future.unit)(bytes, meta)) shouldBe (())
      await(hook.wrap((_, _) => Future.failed(deferred))(bytes, meta).failed) shouldBe deferred
      await(hook.wrap((_, _) => throw deferred)(bytes, meta).failed) shouldBe deferred
      calls.get() shouldBe 0
    }

    "return the handler error at the deadline while the callback is still running" in {
      val started = Promise[Unit]()
      val callback = Promise[Unit]()
      val logged = Promise[Throwable]()
      val logs = new AtomicInteger(0)
      val hook = new LastAttemptFailure((_, _, _) => {
        started.success(())
        callback.future
      }, error => {
        logs.incrementAndGet()
        logged.trySuccess(error)
        ()
      })

      val result = hook.wrap((_, _) => Future.failed(failure))(bytes, meta)
      await(started.future)
      await(hook.onTimeout()) shouldBe Left(failure)
      await(logged.future) shouldBe a[TimeoutException]
      callback.failure(new IllegalStateException("late callback error"))
      await(result.failed) shouldBe failure
      logs.get() shouldBe 1
    }

    "skip a callback when the handler fails after the deadline" in {
      val hook = new LastAttemptFailure((_, _, _) => fail("late callback"), _ => ())
      val processing = Promise[Unit]()
      val result = hook.wrap((_, _) => processing.future)(bytes, meta)

      await(hook.onTimeout().failed) shouldBe a[TimeoutException]
      processing.failure(failure)
      await(result.failed) shouldBe failure
    }
  }
}
