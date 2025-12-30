package databus

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/consumer"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

var errHandler = errors.New("error in handler")

func (b *DataBus) CreateConsumer(ctx context.Context, kafkaHost []string, credentials *credential.Conf, topic model.TopicName, group model.GroupName, id model.ConsumerId, batchSize int) (model.IConsumer, error) {
	options := []consumer.Option{}
	if batchSize != 0 {
		options = append(options, consumer.WithBatchSize(batchSize))
	}
	if credentials != nil {
		options = append(options, consumer.WithCredentials(credentials))
	}
	c, err := consumer.New(ctx, kafkaHost, topic, group, id, options...)
	connMsg := fmt.Sprintf("%s with credentials %s", strings.Join(kafkaHost, ", "), credentials)
	if err != nil {
		logger.Consumer(ctx, c, "Failed connect to kafka %s: %v", connMsg, err)
	} else {
		logger.Consumer(ctx, c, "Success connect to kafka %s", connMsg)
	}
	return c, err
}

func (b *DataBus) FindRepeatStrategy(topic model.TopicName, group model.GroupName, id model.ConsumerId) *model.RepeatStrategy {
	return b.connStore.FindRepeatStrategy(topic, group, id)
}

func (b *DataBus) Consume(
	ctx context.Context,
	c model.IConsumer,
	srv pb.RedbusService_ConsumeServer,
	repeatStrategy *model.RepeatStrategy,
	handler func(ctx context.Context, list model.MessageList) error,
	cancel context.CancelFunc,
) error {
	b.startConsumer(ctx, c, srv, repeatStrategy)
	err := b.processConsumer(ctx, c, handler, cancel)
	b.finishConsumer(ctx, c, err)
	return nil
}

func (b *DataBus) startConsumer(ctx context.Context, c model.IConsumer, srv pb.RedbusService_ConsumeServer, repeatStrategy *model.RepeatStrategy) {
	logger.Consumer(ctx, c, "Start consuming")
	b.connStore.AddConsumer(c, srv, repeatStrategy)
}

func (b *DataBus) finishConsumer(ctx context.Context, c model.IConsumer, err error) {
	b.connStore.RemoveConsumer(c)
	logger.Consumer(ctx, c, "Finish consuming, error: %v", err)
}

func (b *DataBus) processConsumer(
	ctx context.Context,
	c model.IConsumer,
	handler func(ctx context.Context, list model.MessageList) error,
	cancel context.CancelFunc,
) error {

	go func() {
		defer func() {
			logger.Consumer(ctx, c, "Consume kafka stop")
			if _, err := c.Close(); err != nil {
				logger.Consumer(ctx, c, "Can't stop kafka example error: %v\n", err)
			}
		}()

		var attempt int
		var consumeErr error
		var lastRebalanceTime time.Time
		const minRebalanceDelay = 10 * time.Second // Минимальная задержка после ребалансировки

		for {
			attempt++
			if attempt != 1 {
				// Для ошибок ребалансировки нужна дополнительная задержка
				if consumer.IsRebalanceError(consumeErr) {
					now := time.Now()
					// Если прошло недостаточно времени с последней ребалансировки, ждем дольше
					if now.Sub(lastRebalanceTime) < minRebalanceDelay {
						waitTime := minRebalanceDelay - now.Sub(lastRebalanceTime)
						logger.Consumer(ctx, c, "Rebalance error detected, waiting %v before reconnect...", waitTime)
						time.Sleep(waitTime)
					}
					lastRebalanceTime = time.Now()
					logger.Consumer(ctx, c, "Rebalance error: %v, reconnecting after %v...", consumeErr, b.conf.Kafka.FailTimeout)
				} else if consumer.IsAuthorizationError(consumeErr) {
					logger.Consumer(ctx, c, "Authorization error: %v, %v waiting...", consumeErr, b.conf.Kafka.FailTimeout)
				} else {
					logger.Consumer(ctx, c, "Consume kafka error: %v, %v waiting...", consumeErr, b.conf.Kafka.FailTimeout)
				}
				time.Sleep(b.conf.Kafka.FailTimeout.Duration)
			}

			// Переподключаемся только при ошибках авторизации или ребалансировки
			// При других ошибках reader может остаться валидным
			if attempt != 1 && (consumer.IsRebalanceError(consumeErr) || consumer.IsAuthorizationError(consumeErr)) {
				logger.Consumer(ctx, c, "Reconnecting kafka consumer...")
				if err := c.Reconnect(ctx); err != nil {
					logger.Consumer(ctx, c, "Failed to reconnect kafka consumer: %v", err)
					consumeErr = err
					continue
				}
			}

			logger.Consumer(ctx, c, "Consume kafka starting...")
			c.SetState(model.ConsumerStateConnected)
			consumeErr = c.Consume(ctx, func(ctx context.Context, list model.MessageList) error { return handler(ctx, list) })
			c.SetState(model.ConsumerStateReconnecting)

			// handler error
			if errors.Is(consumeErr, errHandler) {
				cancel()
				return
			}
			// on done finish
			if errors.Is(consumeErr, context.Canceled) || consumeErr == nil {
				return
			}
		}
	}()

	<-ctx.Done()

	return nil
}
