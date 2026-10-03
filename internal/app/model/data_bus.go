package model

type Stat struct {
	ConsumeTopicCount int
	ConsumerCount     int
	RepeatAllCount    int
	RepeatFailedCount int
	// RepeatDeferredCount is part of RepeatAllCount and never part of RepeatFailedCount.
	RepeatDeferredCount int
}

const Version = "version"
const IdempotencyKeyHeader = "idempotencyKey"
const TimestampHeader = "timestamp"
