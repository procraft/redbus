package databus

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

func (b *DataBus) Produce(
	ctx context.Context,
	topic model.TopicName,
	key string,
	message []byte,
	version int64,
	idempotencyKey string,
	timestamp *time.Time,
) error {
	startedAt := time.Now()
	result := "success"
	defer func() {
		b.metrics.ObserveProduce(string(topic), result, time.Since(startedAt))
	}()
	log.Printf("Handle produce to topic %v: %v / %v", topic, key, message)
	p, err := b.connStore.GetProducer(ctx, topic)
	if err != nil {
		result = "error"
		return err
	}
	headers := make(map[string]string, 2)
	if version != 0 {
		headers[model.Version] = strconv.FormatInt(version, 10)
	}
	if idempotencyKey != "" {
		headers[model.IdempotencyKeyHeader] = idempotencyKey
	}
	if timestamp != nil {
		headers[model.TimestampHeader] = (*timestamp).Format(time.RFC3339)
	}
	if err := p.Produce(ctx, key, message, headers); err != nil {
		result = "error"
		return err
	}
	logger.Produce(ctx, topic, "Produce to kafka: %s", message)
	return err
}
