package databus

import (
	"context"
	"errors"
	"fmt"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"io"
	"strings"
	"time"

	"github.com/prokraft/redbus/internal/pkg/logger"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/consumer"
)

var errHandler = errors.New("error in handler")

func (b *DataBus) CreateConsumerConnection(ctx context.Context, kafkaHost []string, credentials *credential.Conf, topic, group, id string, batchSize int) (model.IConsumer, error) {
	options := []consumer.Option{}
	if batchSize != 0 {
		options = append(options, consumer.WithBatchSize(batchSize))
	}
	if credentials != nil {
		options = append(options, consumer.WithCredentials(credentials))
	}
	c, err := consumer.New(ctx, kafkaHost, topic, group, id, 0, options...)
	connMsg := fmt.Sprintf("%s with credentials %s", strings.Join(kafkaHost, ", "), credentials)
	if err != nil {
		logger.Consumer(ctx, c, "Failed connect to kafka %s: %v", connMsg, err)
	} else {
		logger.Consumer(ctx, c, "Success connect to kafka %s", connMsg)
	}
	return c, err
}

func (b *DataBus) FindRepeatStrategy(topic, group, id string) *model.RepeatStrategy {
	return b.connStore.FindRepeatStrategy(topic, group, id)
}

func (b *DataBus) Consume(
	ctx context.Context,
	srv pb.RedbusService_ConsumeServer,
	topic, group, id string,
	repeatStrategy *model.RepeatStrategy,
	c model.IConsumer,
	handler func(ctx context.Context, list model.MessageList) error,
	cancel context.CancelFunc,
) error {
	b.startConsumer(ctx, srv, topic, group, id, repeatStrategy, c)
	err := b.consumeProcess(ctx, cancel, c, handler)
	b.finishConsumer(ctx, topic, group, id, c, err)
	return nil
}

func (b *DataBus) startConsumer(ctx context.Context, srv pb.RedbusService_ConsumeServer, topic, group, id string, repeatStrategy *model.RepeatStrategy, c model.IConsumer) {
	logger.Consumer(ctx, c, "Start consuming")
	b.connStore.AddConsumer(srv, topic, group, id, repeatStrategy, c)
}

func (b *DataBus) finishConsumer(ctx context.Context, topic, group, id string, c model.IConsumer, err error) {
	b.connStore.RemoveConsumer(topic, group, id)
	logger.Consumer(ctx, c, "Finish consuming, error: %v", err)
}

func (b *DataBus) consumeProcess(
	ctx context.Context,
	cancel context.CancelFunc,
	c model.IConsumer,
	handler func(ctx context.Context, list model.MessageList) error,
) error {

	go func() {
		defer func() {
			logger.Consumer(ctx, c, "Consume kafka stop")
			if err := c.Close(); err != nil {
				logger.Consumer(ctx, c, "Can't stop kafka example error: %v\n", err)
			}
		}()

		var attempt int
		var consumeErr error
		for {
			attempt++
			if attempt != 1 {
				logger.Consumer(ctx, c, "Consume kafka error: %v, %v waiting...", consumeErr, b.conf.Kafka.FailTimeout)
				time.Sleep(b.conf.Kafka.FailTimeout.Duration)
			}
			logger.Consumer(ctx, c, "Consume kafka starting...")
			consumeErr = c.Consume(ctx, func(ctx context.Context, list model.MessageList) error { return handler(ctx, list) })
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

func (b *DataBus) consumerSend(ctx context.Context, c *consumer.Consumer, srv pb.RedbusService_ConsumeServer, data *pb.ConsumeResponse) (bool, error) {
	err := srv.Send(data)
	if err == io.EOF {
		return false, err
	}
	if err != nil {
		logger.Consumer(ctx, c, "Can't send to example client: %v", err)
		return true, err
	}
	return true, nil
}

func (b *DataBus) consumerRecv(ctx context.Context, c *consumer.Consumer, srv pb.RedbusService_ConsumeServer) (bool, *pb.ConsumeRequest, error) {
	rest, err := srv.Recv()
	if err == io.EOF {
		return false, nil, nil
	}
	if err != nil {
		logger.Consumer(ctx, c, "Can't receive from example client: %v", err)
		return true, nil, err
	}
	return true, rest, nil
}
