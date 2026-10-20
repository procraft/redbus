package redbus

import (
	"errors"
	"fmt"

	"github.com/procraft/redbus/api/golang/outbox"
	"github.com/procraft/redbus/api/golang/producer"
)

// Settings are the connection and switches of a bus client. The bus is used at all only when at
// least one side is enabled; see Client for what a disabled side does. The env tags are relative:
// use them with a prefix, e.g. caarlos0/env with envPrefix:"REDBUS_".
type Settings struct {
	Host            string `json:"host" env:"HOST"`
	Port            int    `json:"port" env:"PORT"`
	ProducerEnabled bool   `json:"producerEnabled" env:"PRODUCER_ENABLED"`
	ConsumerEnabled bool   `json:"consumerEnabled" env:"CONSUMER_ENABLED"`
	// OutboxBatchSize is the maximum outbox rows per flusher query and batch request; 0 means
	// outbox.DefaultBatchSize.
	OutboxBatchSize int `json:"outboxBatchSize,omitempty" env:"OUTBOX_BATCH_SIZE"`
	// MaxMessageBytes is the payload limit of direct produce and the outbox; 0 means
	// producer.DefaultMaxMessageBytes (256 KiB). A longer payload fails with
	// *producer.MessageTooLargeError and is neither sent nor written.
	MaxMessageBytes int `json:"maxMessageBytes,omitempty" env:"MAX_MESSAGE_BYTES"`
}

func (s Settings) Enabled() bool {
	return s.ProducerEnabled || s.ConsumerEnabled
}

func (s Settings) batchSize() int {
	if s.OutboxBatchSize == 0 {
		return outbox.DefaultBatchSize
	}
	return s.OutboxBatchSize
}

func (s Settings) maxMessageBytes() int {
	if s.MaxMessageBytes == 0 {
		return producer.DefaultMaxMessageBytes
	}
	return s.MaxMessageBytes
}

// Validate checks the settings; a fully disabled client needs no address.
func (s Settings) Validate() error {
	var errs []error
	if s.OutboxBatchSize < 0 {
		errs = append(errs, fmt.Errorf("outboxBatchSize must not be negative, got %d", s.OutboxBatchSize))
	}
	if s.MaxMessageBytes < 0 {
		errs = append(errs, fmt.Errorf("maxMessageBytes must not be negative, got %d", s.MaxMessageBytes))
	}
	if s.Enabled() {
		if s.Port <= 0 {
			errs = append(errs, fmt.Errorf("port must be positive, got %d", s.Port))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("redbus settings: %w", err)
	}
	return nil
}
