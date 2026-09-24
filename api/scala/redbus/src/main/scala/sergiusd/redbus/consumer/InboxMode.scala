package sergiusd.redbus.consumer

/**
 * Inbox dedup of a consumer, selected with [[Option.WithInbox]]. One value per consumer, so the
 * mutually exclusive modes cannot be combined by accident.
 */
sealed trait InboxMode

object InboxMode {
  /** Skip marked messages and write the mark after the processor succeeds ([[Option.WithOnlyOnceProcessor]]). */
  case object OnlyOnce extends InboxMode

  /** Skip marked messages and hand `MessageMeta.claimDba` to the processor ([[Option.WithTransactionalInbox]]). */
  case object Transactional extends InboxMode

  /** No inbox: every delivery reaches the processor. */
  case object Disabled extends InboxMode
}
