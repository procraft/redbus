package databus

import (
	"context"
	"log"

	"github.com/prokraft/redbus/internal/pkg/logger"
)

func (b *DataBus) Produce(ctx context.Context, topic, key string, message []byte) error {
	log.Printf("Handle produce to topic %v: %v / %v", topic, key, message)
	p, err := b.connStore.GetProducer(ctx, topic)
	if err != nil {
		return err
	}
	if err := p.Produce(ctx, []byte(key), message); err != nil {
		return err
	}
	logger.Produce(ctx, topic, "Produce to kafka: %s", message)
	return err
}
