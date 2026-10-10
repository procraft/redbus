package sergiusd.redbus.producer

/**
 * The payload of a message to `topic` is `sizeBytes` long, above the client's `maxBytes` limit. The
 * message was neither sent to the bus nor written to the outbox; nothing was truncated. The size is
 * that of the payload bytes passed to `produce` / `produceDba` (for a typed client, the serialized
 * message), which is exactly what the bus would write to Kafka as the record value.
 */
final case class MessageTooLargeException(topic: String, sizeBytes: Int, maxBytes: Int)
  extends IllegalArgumentException(
    s"Redbus message to topic $topic is $sizeBytes bytes, above the limit of $maxBytes bytes; it was not sent"
  )
