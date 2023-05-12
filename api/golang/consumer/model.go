package consumer

import (
	"context"
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

type ConsumeProcessor = func(ctx context.Context, data []byte, id string) error

type ProcessResult struct {
	id  string
	err error
}
