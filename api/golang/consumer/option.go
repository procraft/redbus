package consumer

import "time"

type ServiceOptionFn = func(c *Service)

func WithServiceUnavailableTimeout(unavailableTimeout time.Duration) ServiceOptionFn {
	return func(c *Service) {
		c.unavailableTimeout = unavailableTimeout
	}
}

type OptionFn = func(c *Listener)

func WithConsumeTimeout(consumeTimeout time.Duration) OptionFn {
	return func(l *Listener) {
		l.consumeTimeout = consumeTimeout
	}
}

func WithRepeatStrategyEven(maxAttempts int, intervalSec int) OptionFn {
	return func(l *Listener) {
		l.repeatStrategy = &RepeatStrategy{
			maxAttempts:  maxAttempts,
			evenStrategy: &RepeatStrategyEven{intervalSec: intervalSec},
		}
	}
}

func WithRepeatStrategyProgressive(maxAttempts int, intervalSec int, multiplier float32) OptionFn {
	return func(l *Listener) {
		l.repeatStrategy = &RepeatStrategy{
			maxAttempts:         maxAttempts,
			progressiveStrategy: &RepeatStrategyProgressive{intervalSec: intervalSec, multiplier: multiplier},
		}
	}
}

func WithBatchSize(batchSize int) OptionFn {
	return func(l *Listener) {
		l.batchSize = batchSize
	}
}
