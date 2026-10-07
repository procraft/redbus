## Redbus service Go SDK

The Go SDK lives in the main module: `github.com/procraft/redbus/api/golang/...`. It covers the same
needs as the Scala SDK (`api/scala/redbus`): direct and batch produce, the transactional outbox
with its flusher, consumers with repeat strategies, deferred retries and inbox dedup, and a typed
client for protobuf messages.

Install with `go get github.com/procraft/redbus@v0.1.0`; the module requires Go 1.27 or newer.

| Package | Purpose |
|---|---|
| `redbus` | Recommended entry point: `Client` configured by `Settings`, protobuf helpers |
| `producer` | Low-level gRPC producer: `Produce`, `ProduceBatch`, options |
| `consumer` | Low-level consumer: `Consume` (payload) and `ConsumeMessages` (`Message` with metadata) |
| `outbox` | `Write` into `redbus_outbox` inside a transaction, `Flusher` that delivers the rows |
| `inbox` | Dedup over `redbus_inbox`: modes, `Claim`, `Guard` |

The database API is `database/sql` (`*sql.DB`, `*sql.Tx`). A pgx pool works through
`github.com/jackc/pgx/v5/stdlib` (`stdlib.OpenDBFromPool`). Client tables: `api/inbox.sql`,
`api/outbox.sql`; they are shared with the Scala SDK, so a service can switch SDKs without migration.

### Typed client

```go
import (
	"github.com/procraft/redbus/api/golang/consumer"
	"github.com/procraft/redbus/api/golang/inbox"
	"github.com/procraft/redbus/api/golang/producer"
	"github.com/procraft/redbus/api/golang/redbus"
)

bus, err := redbus.New(
	redbus.Settings{Host: "redbus", Port: 50005, ProducerEnabled: true, ConsumerEnabled: true},
	redbus.WithDB(db),                  // holds redbus_inbox / redbus_outbox
	redbus.WithListenDSN(dsn),          // optional: react to pg_notify, not only to the sweep
	redbus.WithLogger(slog.Default()),
)
defer bus.Close()                       // stops the flusher; cancel consumers first

bus.StartFlusher(ctx)                   // background until ctx is cancelled or Close; repeated calls do nothing
bus.ProduceProto(ctx, "topic", msg, producer.WithIdempotencyKey(key))  // (sent bool, err error)
bus.ProduceProtoTx(ctx, tx, "topic", msg)                              // outbox row inside tx

err = redbus.ConsumeProto(ctx, bus, "topic", "group", inbox.Transactional,
	func(ctx context.Context, req *pb.Request, msg consumer.Message) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := inbox.Guard(ctx, tx, msg.Claim, func(ctx context.Context) error {
			return businessWrites(ctx, tx, req)
		}); err != nil {
			return err
		}
		return tx.Commit()
	},
	consumer.WithBatchSize(10),
)                                       // blocks until ctx is cancelled
```

- `Settings` has JSON tags (`host`, `port`, `producerEnabled`, `consumerEnabled`,
  `outboxBatchSize`) and relative env tags for `caarlos0/env` with a prefix such as `REDBUS_`.
- A disabled side is a no-op: `Produce*` returns `false, nil`, `Produce*Tx` writes no row,
  `Consume`/`ConsumeProto` return `nil` at once, `StartFlusher` starts nothing. The gRPC connection
  is created only when a side is enabled.
- A payload that does not decode as the requested message is logged (`redbus.receive.invalid`,
  `action=drop`) and acknowledged, because a retry cannot fix it.
- The inbox option is placed before the caller's consumer options, so a later `consumer.WithInbox`
  wins.

### Consumer

`Service.ConsumeMessages(ctx, topic, group, handler, options...)` blocks until `ctx` is cancelled,
reconnecting after failures (`WithServiceUnavailableTimeout`, default 60 s). It does not wait for
a handler that ignores its `ctx`: such a handler may still run after a timeout or after the return. Messages of a batch
are handled concurrently; a handler error, panic or `WithConsumeTimeout` (default 60 s) fails that
message only. `Message` carries `ID`, `Data`, `IdempotencyKey`, `Version`, `Timestamp` and, in the
transactional inbox mode, `Claim`. Options: `WithBatchSize`, `WithRepeatStrategyEven`,
`WithRepeatStrategyProgressive`, `WithInbox`. `Consume` is the payload-only form.

Return `consumer.NewRetryLaterError(err, delay)` for a temporary external condition (rate limit):
a compatible bus schedules the next delivery after `delay` without consuming an attempt.

### Inbox dedup

`inbox.Mode` is `Disabled`, `OnlyOnce` or `Transactional`; the key is `group|topic|idempotencyKey`
(the message id replaces an empty key), the same as in the Scala SDK.

- `OnlyOnce` — skip a marked message, run the handler, mark after success. The mark is a separate
  write: a crash in between or a concurrent redelivery can process a message twice. A failed mark
  after a successful handler is logged and the message is still acknowledged.
- `Transactional` — the same pre-check, but the SDK never writes the mark. Run `msg.Claim` as the
  first step of the handler's own transaction (`inbox.Guard` does it): `false` means the message
  was already processed, so skip the business logic and return `nil`. A concurrent claim of the
  same key waits for the first transaction; a rollback releases the key.

Side effects outside the database are not covered by the claim and need their own idempotency.

### Transactional outbox

`outbox.Write(ctx, tx, topic, message, producer options...)` inserts a row inside the caller's
transaction. `outbox.NewFlusher(db, producer.ProduceBatch, options...).Run(ctx)` delivers rows: at
start, every `WithSweepInterval` (default 30 s) and, with `WithListenDSN`, on
`pg_notify('redbus_outbox')`. One pass at a time; each step selects at most `WithBatchSize` rows
(default 100) in id order `FOR UPDATE`, publishes the rows of each topic with one `ProduceBatch`
request and deletes the published rows in the same transaction, until the table is empty. Row
locks serialise flushers of several replicas. A failed or timed out publish is ambiguous and may
duplicate on retry, so consumers must stay idempotent.

**Order is guaranteed only within a topic.** A topic whose publish fails (bus down, Kafka
`TOPIC_AUTHORIZATION_FAILED`, …) keeps its rows and is excluded from the selection for the rest of
the pass; the other topics are still delivered, and the failed topic is retried on the next trigger.
Each failed topic is logged at error level at most once per `WithErrorLogInterval` (default one
minute); the next report carries the number of suppressed ones in `suppressed`.

- The transaction and its row locks are held while the batch is published (up to
  `WithPublishTimeout`, default 30 s, per topic in the selection), so the flusher occupies one connection of the pool, and a
  replica waiting for the locks occupies another. Give `WithDB` a pool with room for that, or a
  separate small pool.
- Id order is insertion order, not commit order: a transaction that inserted a row earlier but
  committed later has its row published after rows committed before it.
- A shutdown after a successful publish still commits the delete, so the batch is not sent again.

### Verify

```shell
go test ./api/golang/...
REDBUS_PG_TEST_URL='postgres://localhost:5432/postgres?sslmode=disable' go test -count=1 ./api/golang/...
```

The second command also runs the PostgreSQL tests; each creates and drops its own database, so
the user needs `CREATEDB`.
