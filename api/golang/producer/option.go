package producer

import (
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
