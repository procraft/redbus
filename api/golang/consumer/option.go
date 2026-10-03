package consumer

import (
	"log/slog"
	"time"

	"github.com/procraft/redbus/api/golang/inbox"
)

type ServiceOptionFn = func(c *Service)

func WithServiceUnavailableTimeout(unavailableTimeout time.Duration) ServiceOptionFn {
	return func(c *Service) {
		c.unavailableTimeout = unavailableTimeout
	}
}

// WithServiceLogger sets the logger of the consumer (default slog.Default()).
func WithServiceLogger(log *slog.Logger) ServiceOptionFn {
	return func(c *Service) {
		c.log = log
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

// WithInbox selects the inbox dedup mode over the client's redbus_inbox table in db (see
// inbox.Mode). It replaces any inbox option given before it; inbox.Disabled ignores db.
func WithInbox(db inbox.DB, mode inbox.Mode) OptionFn {
	return func(l *Listener) {
		l.inboxDB = db
		l.inboxMode = mode
		if mode == inbox.Disabled {
			l.inboxDB = nil
		}
	}
}
