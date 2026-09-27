// Package inbox implements consumer-side deduplication over the client's redbus_inbox table
// (schema: api/inbox.sql). It shares the table and key format with the Scala SDK, so marks written
// by either SDK or either mode are visible to the others.
package inbox

import (
	"context"
	"database/sql"
	"fmt"
)

// Mode selects the inbox dedup of a consumer.
type Mode int

const (
	// Disabled: no inbox, every delivery reaches the handler.
	Disabled Mode = iota
	// OnlyOnce: skip marked messages and write the mark after the handler succeeds. The mark is
	// written separately from the handler's own writes, so a crash in between or a concurrent
	// redelivery can process a message twice.
	OnlyOnce
	// Transactional: skip marked messages, but never write the mark; the handler receives a
	// ClaimFunc and runs it as the first step of its own transaction (see Guard).
	Transactional
)

func (m Mode) String() string {
	switch m {
	case Disabled:
		return "disabled"
	case OnlyOnce:
		return "only-once"
	case Transactional:
		return "transactional"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}

// Execer is satisfied by *sql.DB, *sql.Tx and *sql.Conn.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// DB is what the SDK needs for the pre-check and the only-once mark; *sql.DB satisfies it.
type DB interface {
	Execer
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ClaimFunc inserts the processed mark of one message inside tx and reports whether this call
// inserted it. false means another delivery already processed the message.
type ClaimFunc func(ctx context.Context, tx Execer) (bool, error)

// Key is the redbus_inbox key of a message; the message id stands in for an empty idempotency key
// on the consumer side.
func Key(group, topic, idempotencyKey string) string {
	return group + "|" + topic + "|" + idempotencyKey
}

// created_at is written explicitly: client tables are not guaranteed to carry the column default
// from api/inbox.sql.
const (
	claimSQL     = `INSERT INTO public.redbus_inbox ("key", created_at) VALUES ($1, now()) ON CONFLICT ("key") DO NOTHING`
	processedSQL = `SELECT EXISTS (SELECT 1 FROM public.redbus_inbox WHERE "key" = $1)`
)

// Claim is the transactional-inbox claim: INSERT … ON CONFLICT DO NOTHING, true only when this
// call inserted the row. Run it as the first step of the transaction that holds the business
// writes: a concurrent claim of the same key waits for that transaction and then gets false, and a
// rollback releases the key together with the business writes.
func Claim(ctx context.Context, tx Execer, group, topic, idempotencyKey string) (bool, error) {
	res, err := tx.ExecContext(ctx, claimSQL, Key(group, topic, idempotencyKey))
	if err != nil {
		return false, fmt.Errorf("redbus inbox claim: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("redbus inbox claim: %w", err)
	}
	return n == 1, nil
}

// ClaimFor binds Claim to one message.
func ClaimFor(group, topic, idempotencyKey string) ClaimFunc {
	return func(ctx context.Context, tx Execer) (bool, error) {
		return Claim(ctx, tx, group, topic, idempotencyKey)
	}
}

// IsProcessed reports whether the message is already marked.
func IsProcessed(ctx context.Context, db DB, group, topic, idempotencyKey string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, processedSQL, Key(group, topic, idempotencyKey)).Scan(&exists); err != nil {
		return false, fmt.Errorf("redbus inbox check: %w", err)
	}
	return exists, nil
}

// SetProcessed writes the mark; an existing mark is kept.
func SetProcessed(ctx context.Context, db Execer, group, topic, idempotencyKey string) error {
	if _, err := Claim(ctx, db, group, topic, idempotencyKey); err != nil {
		return err
	}
	return nil
}

// Guard runs fn behind the transactional-inbox claim inside tx and reports whether fn ran. On a
// lost claim (the message was already processed) fn is skipped and Guard returns (false, nil):
// commit the transaction and complete the message successfully. Without a claim (nil: another
// inbox mode, or a call path that does not come from the bus) fn always runs.
func Guard(ctx context.Context, tx Execer, claim ClaimFunc, fn func(ctx context.Context) error) (bool, error) {
	if claim != nil {
		claimed, err := claim(ctx, tx)
		if err != nil || !claimed {
			return false, err
		}
	}
	return true, fn(ctx)
}
