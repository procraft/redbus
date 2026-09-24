package sergiusd.redbus.consumer

import slick.dbio.{DBIOAction, Effect, NoStream}

import scala.concurrent.ExecutionContext

object Inbox {

  /**
   * Runs `fn` behind the transactional-inbox claim; compose the result into the transaction that
   * holds the business writes. The claim runs first: on `false` the message was already processed,
   * so `onSkip` is called and `fn` is skipped (the processor then completes successfully). Without a
   * claim (`None`: another inbox mode, or a call path that does not come from the bus) `fn` always
   * runs.
   */
  def guard[E <: Effect](claim: scala.Option[Model.InboxClaim], onSkip: => Unit = ())(
    fn: => DBIOAction[Unit, NoStream, E]
  )(implicit ec: ExecutionContext): DBIOAction[Unit, NoStream, E with Effect.Write] = claim match {
    case None => fn
    case Some(c) =>
      c.flatMap {
        case true => fn
        case false =>
          onSkip
          DBIOAction.successful(())
      }
  }
}
