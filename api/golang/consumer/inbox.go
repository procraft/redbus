package consumer

import (
	"context"
	"log/slog"

	"github.com/prokraft/redbus/api/golang/inbox"
)

// inboxStore is the database side of the inbox modes; replaced in unit tests.
type inboxStore interface {
	isProcessed(ctx context.Context, group, topic, idempotencyKey string) (bool, error)
	setProcessed(ctx context.Context, group, topic, idempotencyKey string) error
}

type sqlInboxStore struct {
	db inbox.DB
}

func (s sqlInboxStore) isProcessed(ctx context.Context, group, topic, idempotencyKey string) (bool, error) {
	return inbox.IsProcessed(ctx, s.db, group, topic, idempotencyKey)
}

func (s sqlInboxStore) setProcessed(ctx context.Context, group, topic, idempotencyKey string) error {
	return inbox.SetProcessed(ctx, s.db, group, topic, idempotencyKey)
}

// processWithInbox runs handler behind the inbox dedup of the given mode; store nil disables it.
// The message id stands in for an empty idempotency key.
func processWithInbox(
	ctx context.Context,
	store inboxStore,
	mode inbox.Mode,
	group, topic string,
	msg Message,
	handler Handler,
	log *slog.Logger,
) error {
	key := msg.IdempotencyKey
	if key == "" {
		key = msg.ID
	}
	if store == nil || mode == inbox.Disabled || key == "" {
		return handler(ctx, msg)
	}
	processed, err := store.isProcessed(ctx, group, topic, key)
	if err != nil {
		return err
	}
	if processed {
		log.Info("redbus: skip already processed message", "group", group, "topic", topic, "idempotencyKey", key)
		return nil
	}
	if mode == inbox.Transactional {
		msg.Claim = inbox.ClaimFor(group, topic, key)
		return handler(ctx, msg)
	}
	if err := handler(ctx, msg); err != nil {
		return err
	}
	// The handler has already succeeded: failing the message now would only process it again.
	if err := store.setProcessed(ctx, group, topic, key); err != nil {
		log.Error("redbus: can't mark message as processed", "group", group, "topic", topic, "idempotencyKey", key, "error", err)
	}
	return nil
}
