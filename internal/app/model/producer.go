package model

import (
	"context"
	"time"
)

type ProduceData struct {
	Key            string
	Message        []byte
	Version        int64
	IdempotencyKey string
	Timestamp      *time.Time
}

type ProduceMessage struct {
	Key     string
	Message []byte
	Headers map[string]string
}

type IProducer interface {
	Produce(ctx context.Context, key string, message []byte, headers map[string]string) error
	ProduceBatch(ctx context.Context, messages []ProduceMessage) error
	Close() error
}
