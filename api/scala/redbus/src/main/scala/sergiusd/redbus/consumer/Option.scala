package sergiusd.redbus.consumer

import scala.concurrent.duration.FiniteDuration

object Option {

  type ListenerOptionFn = Model.Listener => Model.Listener

  def WithConsumeTimeout(consumeTimeout: FiniteDuration): ListenerOptionFn = {
    listener => listener.copy(consumeTimeout = consumeTimeout)
  }

  def WithUnavailableTimeout(unavailableTimeout: FiniteDuration): ListenerOptionFn = {
    listener => listener.copy(unavailableTimeout = unavailableTimeout)
  }


  def WithRepeatStrategyEven(maxAttempts: Int, intervalSec: Int): ListenerOptionFn = {
    listener => listener.copy(repeatStrategy = Some(new Model.RepeatStrategy(
      maxAttempts =  maxAttempts,
      evenStrategy = Some(new Model.RepeatStrategyEven(intervalSec = intervalSec)),
    )))
  }

  def WithRepeatStrategyProgressive(maxAttempts: Int, intervalSec: Int, multiplier: Float): ListenerOptionFn = {
    listener => listener.copy(repeatStrategy = Some(new Model.RepeatStrategy(
      maxAttempts = maxAttempts,
      progressiveStrategy = Some(new Model.RepeatStrategyProgressive(intervalSec = intervalSec, multiplier = multiplier)),
    )))
  }

  def WithBatchSize(batchSize: Int): ListenerOptionFn = {
    listener => listener.copy(batchSize = batchSize)
  }

}