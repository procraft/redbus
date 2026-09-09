package sergiusd.redbus.consumer

import scala.concurrent.duration.FiniteDuration

/** A temporary processing failure which should be retried after `delay` without consuming an attempt. */
final case class RetryLaterException(cause: Throwable, delay: FiniteDuration)
  extends RuntimeException(scala.Option(cause).fold("retry later")(_.getMessage), cause)
