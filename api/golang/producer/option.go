package producer

import (
	"github.com/google/uuid"
	"github.com/prokraft/redbus/api/golang/pb"
)

type OptionFn = func(c *pb.ProduceRequest)

func WithKeyUUIDv4() OptionFn {
	return func(r *pb.ProduceRequest) {
		r.Key = uuid.New().String()
	}
}

func WithKey(key string) OptionFn {
	return func(r *pb.ProduceRequest) {
		r.Key = key
	}
}
