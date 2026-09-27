package producer

import (
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
)

type OptionFn = func(c *pb.ProduceRequest)

func WithIdempotencyKey(key string) OptionFn {
	return func(r *pb.ProduceRequest) {
		r.IdempotencyKey = key
	}
}

func WithKey(key string) OptionFn {
	return func(r *pb.ProduceRequest) {
		r.Key = key
	}
}

// WithVersion sets the message version the consumer receives in consumer.Message.Version.
func WithVersion(version int64) OptionFn {
	return func(r *pb.ProduceRequest) {
		r.Version = version
	}
}

// WithTimestamp replaces the default "now" timestamp of the message.
func WithTimestamp(timestamp time.Time) OptionFn {
	return func(r *pb.ProduceRequest) {
		r.Timestamp = formatTimestamp(timestamp)
	}
}
