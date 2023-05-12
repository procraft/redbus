package consumer

import "time"

type ServiceOptionFn = func(c *Service)

func WithServiceUnavailableTimeout(unavailableTimeout time.Duration) ServiceOptionFn {
	return func(c *Service) {
		c.unavailableTimeout = unavailableTimeout
	}
}

type ListenerOptionFn = func(c *Listener)

func WithConsumeTimeout(consumeTimeout time.Duration) ListenerOptionFn {
	return func(l *Listener) {
		l.consumeTimeout = consumeTimeout
	}
}

func WithRepeatStrategyEven(maxAttempts int, intervalSec int) ListenerOptionFn {
	return func(l *Listener) {
		l.repeatStrategy = &RepeatStrategy{
			maxAttempts:  maxAttempts,
			evenStrategy: &RepeatStrategyEven{intervalSec: intervalSec},
		}
	}
}

func WithRepeatStrategyProgressive(maxAttempts int, intervalSec int, multiplier float32) ListenerOptionFn {
	return func(l *Listener) {
		l.repeatStrategy = &RepeatStrategy{
			maxAttempts:         maxAttempts,
			progressiveStrategy: &RepeatStrategyProgressive{intervalSec: intervalSec, multiplier: multiplier},
		}
	}
}

func WithBatchSize(batchSize int) ListenerOptionFn {
	return func(l *Listener) {
		l.batchSize = batchSize
	}
}
