package consumer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sergiusd/redbus/internal/pkg/kafka/credential"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	conf   conf
	hosts  []string
	id     string
	topic  string
	group  string
	reader *kafka.Reader
}

type conf struct {
	log         bool
	credentials *credential.Conf
}

func New(ctx context.Context, hosts []string, topic, group, id string, partition int, options ...Option) (*Consumer, error) {
	var c Consumer

	c.hosts = hosts
	c.id = id
	c.topic = topic
	c.group = group
	c.conf = conf{
		log: false,
	}
	for _, o := range options {
		o(&c.conf)
	}

	readerConf := kafka.ReaderConfig{
		Brokers:   hosts,
		GroupID:   group,
		Topic:     topic,
		Partition: partition,
		MinBytes:  1,    // 1B
		MaxBytes:  10e6, // 10MB
	}

	if c.conf.credentials != nil {
		saslConfig, tlsConfig, err := (*c.conf.credentials).GetSaslAndTls(ctx)
		if err != nil {
			return nil, err
		}
		if saslConfig != nil {
			readerConf.Dialer = &kafka.Dialer{
				Timeout:       10 * time.Second,
				DualStack:     true,
				SASLMechanism: *saslConfig,
				TLS:           tlsConfig,
			}
		}
	}

	c.reader = kafka.NewReader(readerConf)

	return &c, nil
}

func (c *Consumer) GetHosts() []string {
	return c.hosts
}

func (c *Consumer) GetTopic() string {
	return c.topic
}

func (c *Consumer) GetGroup() string {
	return c.group
}

func (c *Consumer) GetID() string {
	return c.id
}

func (c *Consumer) Consume(ctx context.Context, processor func(ctx context.Context, k, v []byte, id string) error) error {
	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return fmt.Errorf("Failed to read kafka message: %w\n", err)
		}
		log.Printf("Receive message from kafka: %v", m.Value)

		if err := c.processAndCommit(ctx, m, processor); err != nil {
			return err
		}

		if ctx.Err() == context.DeadlineExceeded {
			return ctx.Err()
		}
		if ctx.Err() == context.Canceled {
			return nil
		}
	}
}

func (c *Consumer) processAndCommit(ctx context.Context, m kafka.Message, processor func(ctx context.Context, k, v []byte, id string) error) error {
	fn := func() error {
		k := m.Key
		v := m.Value

		if c.conf.log {
			fmt.Printf("Receive kafka message at topic/partition/offset %v/%v/%v: [%s] %s\n", m.Topic, m.Partition, m.Offset, string(k), v)
		}

		id := fmt.Sprintf("%v/%v", m.Partition, m.Offset)
		if err := processor(ctx, k, v, id); err != nil {
			return fmt.Errorf("Can't process kafka event: %w\n", err)
		}

		return nil
	}

	if err := fn(); err != nil {
		fmt.Printf("failed to process messages: %v\n", err)
		return err
	}

	if err := c.reader.CommitMessages(ctx, m); err != nil {
		fmt.Printf("failed to commit messages: %v\n", err)
		return err
	}

	return nil
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
