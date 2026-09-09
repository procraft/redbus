package sergiusd.redbus.consumer

import sergiusd.redbus.api.ConsumeRequest

private[consumer] object ResultConverter {

  def toPB(id: String, result: Either[Throwable, Unit]): ConsumeRequest.Result = result match {
    case Right(_) => ConsumeRequest.Result(ok = true, id = id)
    case Left(e: RetryLaterException) =>
      ConsumeRequest.Result(
        ok = false,
        message = e.getMessage,
        id = id,
        preserveAttempt = true,
        retryAfterSec = durationSeconds(e.delay),
      )
    case Left(e) => ConsumeRequest.Result(ok = false, message = e.getMessage, id = id)
  }

  private def durationSeconds(delay: scala.concurrent.duration.FiniteDuration): Int = {
    if (delay.length <= 0) 0
    else {
      val seconds = math.ceil(delay.toNanos.toDouble / 1000000000d)
      math.min(seconds, Int.MaxValue.toDouble).toInt
    }
  }
}
