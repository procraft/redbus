package databus

import (
	"context"
	"fmt"
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
	return b.ProduceBatch(ctx, topic, []model.ProduceData{{
		Key:            key,
		Message:        message,
		Version:        version,
		IdempotencyKey: idempotencyKey,
		Timestamp:      timestamp,
	}})
}

func (b *DataBus) ProduceBatch(ctx context.Context, topic model.TopicName, messages []model.ProduceData) error {
	startedAt := time.Now()
	result := "success"
	defer func() {
		b.metrics.ObserveProduce(string(topic), result, len(messages), time.Since(startedAt))
	}()
	if len(messages) == 0 {
		result = "error"
		return fmt.Errorf("produce batch for topic %q is empty", topic)
	}
	log.Printf("Handle produce batch to topic %v: %d messages", topic, len(messages))
	p, err := b.connStore.GetProducer(ctx, topic)
	if err != nil {
		result = "error"
		return err
	}
	batch := make([]model.ProduceMessage, 0, len(messages))
	for _, message := range messages {
		batch = append(batch, model.ProduceMessage{
			Key:     message.Key,
			Message: message.Message,
			Headers: produceHeaders(message),
		})
	}
	if err := p.ProduceBatch(ctx, batch); err != nil {
		result = "error"
		return err
	}
	logger.Produce(ctx, topic, "Produce %d messages to kafka", len(messages))
	return nil
}

func produceHeaders(message model.ProduceData) map[string]string {
	headers := make(map[string]string, 3)
	if message.Version != 0 {
		headers[model.Version] = strconv.FormatInt(message.Version, 10)
	}
	if message.IdempotencyKey != "" {
		headers[model.IdempotencyKeyHeader] = message.IdempotencyKey
	}
	if message.Timestamp != nil {
		headers[model.TimestampHeader] = message.Timestamp.Format(time.RFC3339)
	}
	return headers
}
