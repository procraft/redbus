package model

import "context"

type IConsumer interface {
	GetHosts() []string
	GetTopic() string
	GetGroup() string
	GetID() string
	Consume(ctx context.Context, processor func(ctx context.Context, k, v []byte, id string) error) error
	Close() error
}
