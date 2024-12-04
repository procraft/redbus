package consumer

import (
	"context"
	"errors"
	"fmt"
	kpkg "github.com/prokraft/redbus/internal/app/model"
	"time"

	"github.com/prokraft/redbus/internal/pkg/kafka/credential"

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
	batchSize   int
}

func New(ctx context.Context, hosts []string, topic, group, id string, partition int, options ...Option) (*Consumer, error) {
	var c Consumer

	c.hosts = hosts
	c.id = id
	c.topic = topic
	c.group = group
	c.conf = conf{
		log:       false,
		batchSize: 1,
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

func (c *Consumer) Consume(ctx context.Context, processor func(ctx context.Context, list kpkg.MessageList) error) error {
	batchSize := c.conf.batchSize
	for {
		var mList []kafka.Message
		var err error
		var m kafka.Message
		if batchSize > 1 {
			m, err = c.reader.FetchMessage(ctx)
			if err == nil {
				mList = append(mList, m)

				waitTimeout := time.Millisecond * 100
				for {
					ctx, cancel := context.WithTimeout(ctx, waitTimeout)
					m, err = c.reader.FetchMessage(ctx)
					cancel()
					if err == nil {
						mList = append(mList, m)
						if len(mList) == batchSize {
							break
						}
					}
					if errors.Is(err, context.DeadlineExceeded) {
						err = nil
						break
					}
				}
			}
		} else {
			m, err = c.reader.FetchMessage(ctx)
			if err == nil {
				mList = []kafka.Message{m}
			}
		}
		if err != nil {
			return fmt.Errorf("Failed to read kafka message: %w\n", err)
		}

		if err := c.processAndCommit(ctx, mList, processor); err != nil {
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

func (c *Consumer) processAndCommit(ctx context.Context, mList []kafka.Message, processor func(ctx context.Context, list kpkg.MessageList) error) error {
	fn := func() error {
		topic := c.topic
		partition := mList[0].Partition
		offset := mList[len(mList)-1].Offset
		list := make(kpkg.MessageList, 0, len(mList))
		for _, m := range mList {
			list = append(list, kpkg.Message{
				Id:    fmt.Sprintf("%v/%v", partition, m.Offset),
				Key:   m.Key,
				Value: m.Value,
			})
		}

		if c.conf.log {
			fmt.Printf("Receive %d kafka message at topic/partition/offset %v/%v/%v: %v\n", len(mList), topic, partition, offset, list)
		}

		if err := processor(ctx, list); err != nil {
			return fmt.Errorf("Can't process kafka event: %w\n", err)
		}

		return nil
	}

	if err := fn(); err != nil {
		fmt.Printf("failed to process messages: %v\n", err)
		return err
	}

	if err := c.reader.CommitMessages(ctx, mList...); err != nil {
		fmt.Printf("failed to commit messages: %v\n", err)
		return err
	}

	return nil
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
