package sergiusd.redbus.producer

import sergiusd.redbus.api.ProduceRequest

import java.time.ZonedDateTime

object Option {

  type Fn = ProduceRequest => ProduceRequest

  def WithKey(key: String): Fn = {
    consumer => consumer.copy(key = key)
  }

  def WithVersion(version: Long): Fn = {
    consumer => consumer.copy(version = version)
  }

  def WithIdempotencyKey(key: String): Fn = {
    consumer => consumer.copy(idempotencyKey = key)
  }

  def WithTimestamp(timestamp: ZonedDateTime): Fn = {
    consumer => consumer.copy(timestamp = timestamp.toOffsetDateTime.toString)
  }
}