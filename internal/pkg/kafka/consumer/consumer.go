package consumer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	kpkg "github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	conf      conf
	hosts     []string
	id        kpkg.ConsumerId
	topic     kpkg.TopicName
	group     kpkg.GroupName
	state     int32
	offsetMap kpkg.PartitionOffsetMap
	offsetMu  sync.RWMutex
	reader    *kafka.Reader
	mu        sync.Mutex
}

type conf struct {
	log         bool
	credentials *credential.Conf
	batchSize   int
}

func New(
	ctx context.Context,
	hosts []string,
	topic kpkg.TopicName,
	group kpkg.GroupName,
	id kpkg.ConsumerId,
	options ...Option,
) (*Consumer, error) {
	var c Consumer

	c.hosts = hosts
	c.id = id
	c.topic = topic
	c.group = group
	c.state = int32(kpkg.ConsumerStateConnecting)
	c.offsetMap = make(kpkg.PartitionOffsetMap)
	c.conf = conf{
		log:       false,
		batchSize: 1,
	}
	for _, o := range options {
		o(&c.conf)
	}

	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Consumer) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if closed, _ := c.Close(); closed {
		// Даем время для корректного выхода из группы
		time.Sleep(2 * time.Second)
	}

	if err := c.connect(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Consumer) connect(ctx context.Context) error {
	groupID := fmt.Sprintf("%s-%s", string(c.group), string(c.topic))

	readerConf := kafka.ReaderConfig{
		Brokers:  c.hosts,
		GroupID:  groupID,
		Topic:    string(c.topic),
		MinBytes: 1,    // 1B
		MaxBytes: 10e6, // 10MB
		// Увеличиваем таймауты для предотвращения ребалансировок
		// SessionTimeout - максимальное время между heartbeat'ами перед исключением из группы
		// HeartbeatInterval - интервал отправки heartbeat'ов
		// Эти значения должны быть настроены так, чтобы heartbeat отправлялся чаще, чем session timeout
		// По умолчанию в kafka-go: SessionTimeout = 10s, HeartbeatInterval = 3s
		// Увеличиваем для более стабильной работы при сетевых задержках
		SessionTimeout:    30 * time.Second,
		HeartbeatInterval: 10 * time.Second,
		// CommitInterval - интервал автоматического коммита офсетов
		// 0 означает, что коммит происходит вручную через CommitMessages
		CommitInterval: 0,
	}

	readerConf.Dialer = &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	if err := c.conf.credentials.UpdateDialer(ctx, readerConf.Dialer); err != nil {
		return err
	}

	c.reader = kafka.NewReader(readerConf)
	return nil
}

func (c *Consumer) GetHosts() []string {
	return c.hosts
}

func (c *Consumer) GetTopic() kpkg.TopicName {
	return c.topic
}

func (c *Consumer) GetGroup() kpkg.GroupName {
	return c.group
}

func (c *Consumer) setOffset(messageList []kafka.Message) {
	offsetMap := make(map[kpkg.PartitionN]kpkg.Offset, len(messageList))
	for _, message := range messageList {
		offsetMap[kpkg.PartitionN(message.Partition)] = kpkg.Offset(message.Offset)
	}
	c.offsetMu.Lock()
	defer c.offsetMu.Unlock()
	for partition, offset := range offsetMap {
		c.offsetMap[partition] = offset
	}
}

func (c *Consumer) GetOffsetMap() map[kpkg.PartitionN]kpkg.Offset {
	c.offsetMu.RLock()
	defer c.offsetMu.RUnlock()
	return c.offsetMap
}

func (c *Consumer) GetID() kpkg.ConsumerId {
	return c.id
}

func (c *Consumer) GetState() kpkg.ConsumerState {
	return kpkg.ConsumerState(atomic.LoadInt32(&c.state))
}

func (c *Consumer) SetState(state kpkg.ConsumerState) {
	atomic.StoreInt32(&c.state, int32(state))
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

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}

		c.setOffset(mList)
	}
}

func (c *Consumer) processAndCommit(ctx context.Context, mList []kafka.Message, processor func(ctx context.Context, list kpkg.MessageList) error) error {
	fn := func() error {
		topic := c.topic
		partition := mList[0].Partition
		offset := mList[len(mList)-1].Offset
		list := make(kpkg.MessageList, 0, len(mList))
		for _, m := range mList {
			headers := make(map[string]string, len(m.Headers))
			for _, h := range m.Headers {
				headers[h.Key] = string(h.Value)
			}
			list = append(list, kpkg.Message{
				Id:      fmt.Sprintf("%v/%v", partition, m.Offset),
				Key:     m.Key,
				Value:   m.Value,
				Headers: headers,
			})
		}

		if c.conf.log {
			fmt.Printf("Receive %d kafka message at topic/partition/offsetMap %v/%v/%v: %v\n", len(mList), topic, partition, offset, list)
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

func (c *Consumer) Lock() {
	c.mu.Lock()
}

func (c *Consumer) Unlock() {
	c.mu.Unlock()
}

func (c *Consumer) Close() (bool, error) {
	if c.reader == nil {
		return false, nil
	}
	err := c.reader.Close()
	c.reader = nil
	return true, err
}

func IsAuthorizationError(err error) bool {
	// Проверяем, является ли ошибка типом kafka.Error
	var kafkaErr kafka.Error
	if errors.As(err, &kafkaErr) {
		// Код 29 - TopicAuthorizationFailed
		return int(kafkaErr) == 29
	}
	// Проверяем текст ошибки на случай, если это обернутая ошибка
	errStr := err.Error()
	return strings.Contains(errStr, "[29]") ||
		strings.Contains(errStr, "Topic Authorization Failed") ||
		strings.Contains(errStr, "not authorized to access")
}

func IsRebalanceError(err error) bool {
	// Проверяем, является ли ошибка типом kafka.Error
	var kafkaErr kafka.Error
	if errors.As(err, &kafkaErr) {
		// Код 27 - RebalanceInProgress
		return int(kafkaErr) == 27
	}
	// Проверяем текст ошибки на случай, если это обернутая ошибка
	errStr := err.Error()
	return strings.Contains(errStr, "[27]") ||
		strings.Contains(errStr, "Rebalance In Progress") ||
		strings.Contains(errStr, "rebalancing the group")
}
