package consumer

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	host               string
	port               int
	unavailableTimeout time.Duration
}

type Listener struct {
	consumeTimeout time.Duration
	repeatStrategy *RepeatStrategy
	batchSize      int
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

type ConsumeProcessor = func(ctx context.Context, data []byte) error

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
