package model

import "context"

type IProducer interface {
	Produce(ctx context.Context, key string, message []byte, headers map[string]string) error
	Close() error
}
