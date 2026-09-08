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
	"github.com/prokraft/redbus/internal/pkg/stream"
)

func (b *DataBus) CreateConsumer(ctx context.Context, kafkaHost []string, credentials *credential.Conf, topic model.TopicName, group model.GroupName, id model.ConsumerId, batchSize int) (model.IConsumer, error) {
	options := []consumer.Option{}
	if batchSize != 0 {
		options = append(options, consumer.WithBatchSize(batchSize))
	}
	if credentials != nil {
		options = append(options, consumer.WithCredentials(credentials))
	}
	c, err := consumer.New(ctx, kafkaHost, topic, group, id, options...)
	connectionResult := "success"
	if err != nil {
		connectionResult = "error"
	}
	b.metrics.ObserveConsumerConnection(string(topic), string(group), connectionResult)
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
	limits model.ConsumeLimits,
	abort stream.AbortFn,
	handler func(ctx context.Context, list model.MessageList) error,
	cancel context.CancelFunc,
) error {
	b.startConsumer(ctx, c, srv, repeatStrategy, abort, limits)
	err := b.processConsumer(ctx, c, handler, cancel)
	b.finishConsumer(ctx, c, err)
	// Ошибку обработки возвращаем наружу: gRPC закроет стрим с ошибкой, и клиент узнает,
	// что его больше не обслуживают. Молчаливое завершение оставляло клиента в состоянии
	// "якобы подключён" без onError и onCompleted.
	return err
}

func (b *DataBus) startConsumer(
	ctx context.Context,
	c model.IConsumer,
	srv pb.RedbusService_ConsumeServer,
	repeatStrategy *model.RepeatStrategy,
	abort stream.AbortFn,
	limits model.ConsumeLimits,
) {
	logger.Consumer(ctx, c, "Start consuming")
	b.connStore.AddConsumer(c, srv, repeatStrategy, abort, limits)
	b.metrics.AddConsumer(string(c.GetTopic()), string(c.GetGroup()), string(c.GetID()), c.GetState().String())
}

func (b *DataBus) finishConsumer(ctx context.Context, c model.IConsumer, err error) {
	b.metrics.RemoveConsumer(string(c.GetTopic()), string(c.GetGroup()), string(c.GetID()))
	b.connStore.RemoveConsumer(c)
	logger.Consumer(ctx, c, "Finish consuming, error: %v", err)
}

func (b *DataBus) processConsumer(
	ctx context.Context,
	c model.IConsumer,
	handler func(ctx context.Context, list model.MessageList) error,
	cancel context.CancelFunc,
) error {
	// Буфер на 1: горутина не должна блокироваться, если ctx закрыли снаружи и читать некому.
	errCh := make(chan error, 1)

	go func() {
		defer cancel()
		defer func() {
			logger.Consumer(ctx, c, "Consume kafka stop")
			if _, err := c.Close(); err != nil {
				logger.Consumer(ctx, c, "Can't stop kafka example error: %v\n", err)
			}
		}()
		errCh <- b.consumeLoop(ctx, c, handler)
	}()

	<-ctx.Done()

	select {
	case err := <-errCh:
		return err
	default:
		// ctx закрыт снаружи (клиент отключился или сервис останавливается) — своей ошибки нет.
		return nil
	}
}

// consumeLoop читает kafka и отдаёт батчи в handler, переподключаясь при ошибках.
// Возвращает nil, если работа завершена штатно, и ошибку обработки — если стрим нужно закрыть.
func (b *DataBus) consumeLoop(
	ctx context.Context,
	c model.IConsumer,
	handler func(ctx context.Context, list model.MessageList) error,
) error {
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

			// Переподключаемся при любой ошибке. Reader kafka-go после неудачной обработки
			// продолжает с текущей позиции чтения, а не с последнего коммита: если оставить
			// его как есть, незакоммиченные сообщения будут пропущены, а lag останется навсегда.
			logger.Consumer(ctx, c, "Reconnecting kafka consumer...")
			b.metrics.ObserveKafkaReconnect(string(c.GetTopic()), string(c.GetGroup()), kafkaErrorReason(consumeErr))
			if err := c.Reconnect(ctx); err != nil {
				logger.Consumer(ctx, c, "Failed to reconnect kafka consumer: %v", err)
				consumeErr = err
				if ctx.Err() != nil {
					return nil
				}
				continue
			}
		}

		logger.Consumer(ctx, c, "Consume kafka starting...")
		c.SetState(model.ConsumerStateConnected)
		b.metrics.ChangeConsumerState(string(c.GetTopic()), string(c.GetGroup()), string(c.GetID()), model.ConsumerStateConnected.String())
		consumeErr = c.Consume(ctx, func(ctx context.Context, list model.MessageList) error { return handler(ctx, list) })
		c.SetState(model.ConsumerStateReconnecting)
		b.metrics.ChangeConsumerState(string(c.GetTopic()), string(c.GetGroup()), string(c.GetID()), model.ConsumerStateReconnecting.String())

		// Ошибка обработки на стороне клиента: стрим рассинхронизирован или клиент не отвечает,
		// продолжать чтение нельзя — закрываем стрим, чтобы клиент переподключился.
		if errors.Is(consumeErr, model.ErrHandler) {
			return consumeErr
		}
		// on done finish
		if errors.Is(consumeErr, context.Canceled) || consumeErr == nil {
			return nil
		}
	}
}

func kafkaErrorReason(err error) string {
	switch {
	case consumer.IsAuthorizationError(err):
		return "authorization"
	case consumer.IsRebalanceError(err):
		return "rebalance"
	default:
		return "other"
	}
}
