package consumer

import "time"

type OptionFn = func(c *Consumer)

func WithUnavailableTimeout(unavailableTimeout time.Duration) OptionFn {
	return func(c *Consumer) {
		c.unavailableTimeout = unavailableTimeout
	}
}

func WithConsumeTimeout(consumeTimeout time.Duration) OptionFn {
	return func(c *Consumer) {
		c.consumeTimeout = consumeTimeout
	}
}

func WithRepeatStrategyEven(maxAttempts int, intervalSec int) OptionFn {
	return func(c *Consumer) {
		c.repeatStrategy = &RepeatStrategy{
			maxAttempts:  maxAttempts,
			evenStrategy: &RepeatStrategyEven{intervalSec: intervalSec},
		}
	}
}

func WithRepeatStrategyProgressive(maxAttempts int, intervalSec int, multiplier float32) OptionFn {
	return func(c *Consumer) {
		c.repeatStrategy = &RepeatStrategy{
			maxAttempts:         maxAttempts,
			progressiveStrategy: &RepeatStrategyProgressive{intervalSec: intervalSec, multiplier: multiplier},
		}
	}
}
