package sergiusd.redbus.producer

import scala.concurrent.duration.FiniteDuration

/**
 * The bus did not answer a `Produce` / `ProduceBatch` call within `timeout` (gRPC
 * `DEADLINE_EXCEEDED`). The outcome is unknown: the bus may still have written the message(s) to
 * Kafka, so a retry can produce a duplicate. `cause` keeps the original gRPC status.
 */
final case class ProduceTimeoutException(topic: String, timeout: FiniteDuration, cause: Throwable)
  extends RuntimeException(s"Redbus produce to topic $topic got no answer within $timeout; the outcome is unknown", cause)
