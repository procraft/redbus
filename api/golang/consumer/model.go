package consumer

import (
	"context"
	"time"
)

type Consumer struct {
	host               string
	port               int
	unavailableTimeout time.Duration
	consumeTimeout     time.Duration
	repeatStrategy     *RepeatStrategy
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
