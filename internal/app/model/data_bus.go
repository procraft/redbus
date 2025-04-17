package model

type Stat struct {
	ConsumeTopicCount int
	ConsumerCount     int
	RepeatAllCount    int
	RepeatFailedCount int
}

const Version = "version"
const IdempotencyKeyHeader = "idempotencyKey"
const TimestampHeader = "timestamp"
