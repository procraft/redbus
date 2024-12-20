package sergiusd.redbus.consumer

import scala.concurrent.duration.FiniteDuration

object Option {

  type Fn = Model.Listener => Model.Listener

  def WithConsumeTimeout(consumeTimeout: FiniteDuration): Fn = {
    consumer => consumer.copy(consumeTimeout = consumeTimeout)
  }

  def WithUnavailableTimeout(unavailableTimeout: FiniteDuration): Fn = {
    consumer => consumer.copy(unavailableTimeout = unavailableTimeout)
  }


  def WithRepeatStrategyEven(maxAttempts: Int, intervalSec: Int): Fn = {
    consumer => consumer.copy(repeatStrategy = Some(new Model.RepeatStrategy(
      maxAttempts =  maxAttempts,
      evenStrategy = Some(new Model.RepeatStrategyEven(intervalSec = intervalSec)),
    )))
  }

  def WithRepeatStrategyProgressive(maxAttempts: Int, intervalSec: Int, multiplier: Float): Fn = {
    consumer => consumer.copy(repeatStrategy = Some(new Model.RepeatStrategy(
      maxAttempts = maxAttempts,
      progressiveStrategy = Some(new Model.RepeatStrategyProgressive(intervalSec = intervalSec, multiplier = multiplier)),
    )))
  }

  def WithBatchSize(batchSize: Int): Fn = {
    consumer => consumer.copy(batchSize = batchSize)
  }

}