package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/procraft/redbus/api/golang/inbox"
)

type Service struct {
	host               string
	port               int
	unavailableTimeout time.Duration
	log                *slog.Logger
}

type Listener struct {
	consumeTimeout time.Duration
	repeatStrategy *RepeatStrategy
	batchSize      int
	inboxDB        inbox.DB
	inboxMode      inbox.Mode
}

type RepeatStrategy struct {
	maxAttempts         int
	evenStrategy        *RepeatStrategyEven
	progressiveStrategy *RepeatStrategyProgressive
}

type RepeatStrategyEven struct {
	intervalSec int
}

type RepeatStrategyProgressive struct {
	intervalSec int
	multiplier  float32
}

// ConsumeProcessor is the payload-only handler of Consume.
type ConsumeProcessor = func(ctx context.Context, data []byte) error

// Message is one delivery passed to a Handler.
type Message struct {
	// ID is the bus id of the delivery.
	ID   string
	Data []byte
	// IdempotencyKey is the producer's key; empty when the producer sent none.
	IdempotencyKey string
	// Version is the producer's version, 0 when absent.
	Version int64
	// Timestamp is the producer's timestamp, zero when absent or unparsable.
	Timestamp time.Time
	// Claim is set only in the inbox.Transactional mode: run it as the first step of the handler's
	// own transaction (see inbox.Guard); on false skip the business logic and return nil.
	Claim inbox.ClaimFunc
}

// Handler processes one message. A returned error (or a panic, or the consume timeout) fails the
// message and the bus retries it by the repeat strategy; return NewRetryLaterError for a temporary
// condition that should not consume an attempt.
type Handler = func(ctx context.Context, msg Message) error

type ProcessResult struct {
	id  string
	err error
}

// RetryLaterError asks Redbus to retry a temporary failure after Delay without consuming an
// attempt. Return it from a ConsumeProcessor for rate limits and other recoverable outages.
type RetryLaterError struct {
	Err   error
	Delay time.Duration
}

func (e *RetryLaterError) Error() string {
	if e.Err == nil {
		return "retry later"
	}
	return e.Err.Error()
}

func (e *RetryLaterError) Unwrap() error {
	return e.Err
}

func NewRetryLaterError(err error, delay time.Duration) error {
	if err == nil {
		err = fmt.Errorf("retry later")
	}
	return &RetryLaterError{Err: err, Delay: delay}
}
