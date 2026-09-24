package sergiusd.redbus

import com.typesafe.config.Config
import sergiusd.redbus.producer.Flusher

/**
 * Connection and switches of a bus client. The bus is used at all only when at least one side is
 * enabled; see [[ProtoClient]] for what a disabled side does.
 *
 * @param outboxBatchSize maximum outbox rows per flusher query and batch request
 */
final case class RedbusSettings(
  host: String,
  port: Int,
  producerEnabled: Boolean,
  consumerEnabled: Boolean,
  outboxBatchSize: Int = Flusher.defaultBatchSize,
) {
  require(outboxBatchSize > 0, "outboxBatchSize must be positive")

  def enabled: Boolean = producerEnabled || consumerEnabled
}

object RedbusSettings {

  /**
   * Reads the settings from the bus section of an application config (pass the section itself, for
   * example `config.getConfig("app.redbus")`): `host`, `port`, `producerEnabled`, `consumerEnabled`
   * and the optional `outboxBatchSize`.
   */
  def fromConfig(c: Config): RedbusSettings = RedbusSettings(
    host = c.getString("host"),
    port = c.getInt("port"),
    producerEnabled = c.getBoolean("producerEnabled"),
    consumerEnabled = c.getBoolean("consumerEnabled"),
    outboxBatchSize = if (c.hasPath("outboxBatchSize")) c.getInt("outboxBatchSize") else Flusher.defaultBatchSize,
  )
}
