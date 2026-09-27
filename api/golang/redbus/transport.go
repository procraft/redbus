package redbus

import (
	"context"
	"database/sql"

	"github.com/prokraft/redbus/api/golang/consumer"
	"github.com/prokraft/redbus/api/golang/outbox"
	"github.com/prokraft/redbus/api/golang/producer"
)

// transport is what Client needs from the bus; replaced in unit tests.
type transport interface {
	produce(ctx context.Context, topic string, message []byte, opts ...producer.OptionFn) error
	consume(ctx context.Context, topic, group string, handler consumer.Handler, opts ...consumer.OptionFn) error
	runFlusher(ctx context.Context, db *sql.DB, opts ...outbox.FlusherOption) error
	close() error
}

type grpcTransport struct {
	producer *producer.Producer
	consumer *consumer.Service
}

func newGRPCTransport(c *Client) (*grpcTransport, error) {
	p, err := producer.New(c.settings.Host, c.settings.Port)
	if err != nil {
		return nil, err
	}
	consumerOpts := []consumer.ServiceOptionFn{consumer.WithServiceLogger(c.log)}
	if c.unavailableTimeout > 0 {
		consumerOpts = append(consumerOpts, consumer.WithServiceUnavailableTimeout(c.unavailableTimeout))
	}
	return &grpcTransport{
		producer: p,
		consumer: consumer.New(c.settings.Host, c.settings.Port, consumerOpts...),
	}, nil
}

func (t *grpcTransport) produce(ctx context.Context, topic string, message []byte, opts ...producer.OptionFn) error {
	return t.producer.Produce(ctx, topic, message, opts...)
}

func (t *grpcTransport) consume(ctx context.Context, topic, group string, handler consumer.Handler, opts ...consumer.OptionFn) error {
	return t.consumer.ConsumeMessages(ctx, topic, group, handler, opts...)
}

func (t *grpcTransport) runFlusher(ctx context.Context, db *sql.DB, opts ...outbox.FlusherOption) error {
	return outbox.NewFlusher(db, t.producer.ProduceBatch, opts...).Run(ctx)
}

func (t *grpcTransport) close() error {
	return t.producer.Close()
}
