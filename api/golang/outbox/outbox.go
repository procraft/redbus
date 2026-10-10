// Package outbox implements the transactional outbox over the client's redbus_outbox table
// (schema: api/outbox.sql): Write inserts a message inside the caller's transaction and Flusher
// delivers committed rows to the bus. Rows use the same layout and options JSON as the Scala SDK.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/procraft/redbus/api/golang/producer"
)

// Execer is satisfied by *sql.DB, *sql.Tx and *sql.Conn.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// options is the JSONB "options" column. Absent fields are omitted, the same as the Scala SDK
// writes them.
type options struct {
	Key            string `json:"key,omitempty"`
	Version        int64  `json:"version,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
}

const insertSQL = `INSERT INTO public.redbus_outbox (topic, message, options, created_at) VALUES ($1, $2, $3, now())`

// Write inserts the message into redbus_outbox through tx, so it is committed together with the
// caller's own writes; a running Flusher then delivers it. The request is prepared exactly like
// producer.Produce: a random idempotency key and the current timestamp unless options set them.
// The payload limit is producer.DefaultMaxMessageBytes; see WriteWithLimit.
func Write(ctx context.Context, tx Execer, topic string, message []byte, opts ...producer.OptionFn) error {
	return WriteWithLimit(ctx, tx, topic, message, producer.DefaultMaxMessageBytes, opts...)
}

// WriteWithLimit is Write with an explicit payload limit (zero or less means
// producer.DefaultMaxMessageBytes). A longer payload fails with *producer.MessageTooLargeError
// before any statement runs, so no row is written; the caller must roll back its transaction.
func WriteWithLimit(ctx context.Context, tx Execer, topic string, message []byte, maxMessageBytes int, opts ...producer.OptionFn) error {
	if err := producer.CheckMessageSize(topic, message, maxMessageBytes); err != nil {
		return err
	}
	req := producer.NewRequest(topic, message, opts...)
	o, err := json.Marshal(options{
		Key:            req.Key,
		Version:        req.Version,
		IdempotencyKey: req.IdempotencyKey,
		Timestamp:      req.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("redbus outbox: %w", err)
	}
	if message == nil {
		message = []byte{}
	}
	if _, err := tx.ExecContext(ctx, insertSQL, req.Topic, message, string(o)); err != nil {
		return fmt.Errorf("redbus outbox write: %w", err)
	}
	return nil
}
