package sergiusd.redbus

import com.typesafe.config.Config
import sergiusd.redbus.producer.{Flusher, Producer}

import scala.concurrent.duration._

/**
 * Connection and switches of a bus client. The bus is used at all only when at least one side is
 * enabled; see [[ProtoClient]] for what a disabled side does.
 *
 * @param outboxBatchSize     maximum outbox rows per flusher query and batch request
 * @param produceTimeout      deadline of one direct produce call
 * @param produceBatchTimeout deadline of one batch request of the outbox flusher
 * @param maxMessageBytes     limit of one message payload (the serialized message) for direct
 *                            produce and the outbox; a longer one fails with
 *                            `producer.MessageTooLargeException` and is neither sent nor written
 */
final case class RedbusSettings(
  host: String,
  port: Int,
  producerEnabled: Boolean,
  consumerEnabled: Boolean,
  outboxBatchSize: Int = Flusher.defaultBatchSize,
  produceTimeout: FiniteDuration = Producer.defaultProduceTimeout,
  produceBatchTimeout: FiniteDuration = Producer.defaultProduceBatchTimeout,
  maxMessageBytes: Int = Producer.defaultMaxMessageBytes,
) {
  require(outboxBatchSize > 0, "outboxBatchSize must be positive")
  require(produceTimeout.length > 0, "produceTimeout must be positive")
  require(produceBatchTimeout.length > 0, "produceBatchTimeout must be positive")
  require(maxMessageBytes > 0, "maxMessageBytes must be positive")

  def enabled: Boolean = producerEnabled || consumerEnabled
}

object RedbusSettings {

  /**
   * Reads the settings from the bus section of an application config (pass the section itself, for
   * example `config.getConfig("app.redbus")`): `host`, `port`, `producerEnabled`, `consumerEnabled`
   * and the optional `outboxBatchSize`, `produceTimeout` and `produceBatchTimeout` (durations, for
   * example `30s`) and `maxMessageBytes` (a number of bytes or a HOCON size, for example `512K`).
   */
  def fromConfig(c: Config): RedbusSettings = RedbusSettings(
    host = c.getString("host"),
    port = c.getInt("port"),
    producerEnabled = c.getBoolean("producerEnabled"),
    consumerEnabled = c.getBoolean("consumerEnabled"),
    outboxBatchSize = if (c.hasPath("outboxBatchSize")) c.getInt("outboxBatchSize") else Flusher.defaultBatchSize,
    produceTimeout = duration(c, "produceTimeout", Producer.defaultProduceTimeout),
    produceBatchTimeout = duration(c, "produceBatchTimeout", Producer.defaultProduceBatchTimeout),
    maxMessageBytes = bytes(c, "maxMessageBytes", Producer.defaultMaxMessageBytes),
  )

  private def duration(c: Config, path: String, default: FiniteDuration): FiniteDuration =
    if (c.hasPath(path)) c.getDuration(path).toMillis.millis else default

  private def bytes(c: Config, path: String, default: Int): Int =
    if (c.hasPath(path)) {
      val value: Long = c.getBytes(path)
      require(value <= Int.MaxValue, s"$path must not exceed ${Int.MaxValue} bytes")
      value.toInt
    } else default
}
