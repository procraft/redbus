package model

import "context"

type IProducer interface {
	Produce(ctx context.Context, keyAndMessage ...[]byte) error
	Close() error
}
