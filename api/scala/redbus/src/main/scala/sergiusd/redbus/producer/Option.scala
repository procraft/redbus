package sergiusd.redbus.producer

import sergiusd.redbus.api.ProduceRequest

import java.util.UUID

object Option {

  type Fn = ProduceRequest => ProduceRequest

  def WithKeyUUIDv4(): Fn = {
    consumer => consumer.copy(key = UUID.randomUUID().toString)
  }

  def WithKey(key: String): Fn = {
    consumer => consumer.copy(key = key)
  }
}