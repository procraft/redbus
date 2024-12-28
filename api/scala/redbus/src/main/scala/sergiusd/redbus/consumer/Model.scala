package sergiusd.redbus.consumer

import sergiusd.redbus.api.ConsumeRequest
import sergiusd.redbus.consumer.Option.EventKey

import scala.concurrent.Future
import scala.concurrent.duration.FiniteDuration

object Model {

  type Processor = (String, Array[Byte]) => Future[Either[String, Unit]]
  type StopHook = (() => Future[_]) => Unit

  case class Listener(
    consumeTimeout: FiniteDuration,
    repeatStrategy: Option[RepeatStrategy] = None,
    batchSize: Int,
    unavailableTimeout: FiniteDuration,
    logger: String => Unit = _ => (),
    isEventProcessedFn: Option[EventKey => Future[Boolean]] = None,
    setEventProcessedFn: Option[EventKey => Future[_]] = None,
  )

  class RepeatStrategy(
    maxAttempts: Int,
    evenStrategy: Option[RepeatStrategyEven] = None,
    progressiveStrategy: Option[RepeatStrategyProgressive] = None,
  ) {
    def toPB: ConsumeRequest.Connect.RepeatStrategy = {
      ConsumeRequest.Connect.RepeatStrategy(
        maxAttempts,
        evenStrategy.map(x =>
          ConsumeRequest.Connect.RepeatStrategy.EvenConfig(
            intervalSec = x.intervalSec,
          )
        ),
        progressiveStrategy.map(x =>
          ConsumeRequest.Connect.RepeatStrategy.ProgressiveConfig(
            intervalSec = x.intervalSec,
            multiplier = x.multiplier,
          )
        ),
      )
    }
  }

  case class RepeatStrategyEven(
    intervalSec: Int,
  )

  case class RepeatStrategyProgressive(
    intervalSec: Int,
    multiplier: Float,
  )

}