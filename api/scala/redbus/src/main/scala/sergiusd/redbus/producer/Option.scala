package sergiusd.redbus.producer

import sergiusd.redbus.api.ProduceRequest

import java.time.ZonedDateTime

object Option {

  type Fn = ProduceRequest => ProduceRequest

  def WithKey(key: String): Fn = {
    consumer => consumer.copy(key = key)
  }

  def WithIdempotencyKey(key: String): Fn = {
    consumer => consumer.copy(idempotencyKey = key)
  }

  def WithTimestamp(timestamp: ZonedDateTime): Fn = {
    consumer => consumer.copy(timestamp = timestamp.toString)
  }
}