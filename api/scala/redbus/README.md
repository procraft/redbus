## Redbus service scala API and SDK.

### Actor runtime: Pekko

Since **0.3.0** the SDK runs on [Apache Pekko](https://pekko.apache.org) (`org.apache.pekko`
`pekko-actor` 1.0.3) instead of Akka. Pekko is the actor runtime Play 3.0 ships, so the SDK now
lines up with a Play 3 host application. The switch is visible in the public API:
`Client.startProducerDbaFlusher` takes an implicit `org.apache.pekko.actor.ActorSystem`, so a
caller that still runs on Akka cannot compile against 0.3.0.

Two lines are supported:

| Line | Actor runtime | Host | Scala |
|---|---|---|---|
| `0.2.x` (last: `0.2.8`) | Akka 2.6.20 | Play 2.9 | 2.13 |
| `0.3.x` | Pekko 1.0.3 | Play 3.0 | 2.13 |

They are not interchangeable — pick the one matching the host application, and branch fixes for
Akka consumers from the commit that last released `0.2.8` instead of from the Pekko line.

The SDK builds its own actor systems with the default configuration (`ConfigFactory.load()`), so it
picks up the host application's `pekko { … }` section. Pekko 1.0.3 defines no `akka` keys at all and
silently ignores a leftover `akka { … }` block, so a host that has not renamed its configuration
just falls back to Pekko defaults — no error, no warning. Keep host-wide settings such as
`pekko.actor.provider` or a resized `pekko.actor.default-dispatcher` in mind: they now apply to the
SDK's systems too.

### Compile

Run to build.

```shell
sbt compile
```

### Test

```shell
sbt test
```

### Temporary processing failures

Return a failed future with `consumer.RetryLaterException(cause, delay)` when an external provider
asks the consumer to wait (for example, a rate limit). A compatible Redbus server schedules the next
delivery after the requested delay without consuming a retry attempt. Servers older than this SDK
ignore the additive result fields and use their configured repeat strategy.

### Transactional outbox flusher

`producer.Producer.produceDba` writes the message into the client's `redbus_outbox` table
(schema: `api/outbox.sql`) inside the caller's transaction. `Client.startProducerDbaFlusher(db)`
starts the flusher that delivers those rows to the bus: it reacts to the `pg_notify('redbus_outbox')`
sent by the table trigger and, in addition, sweeps the table on a fixed schedule — immediately at
start and then every `sweepInterval` (default 30 seconds, `startProducerDbaFlusher(db, sweepInterval)`).
The sweep delivers rows left over from a restart or a missed notification. The flusher fetches at
most `batchSize` rows (default 100) in `id` order, publishes the same-topic prefix with one confirmed
batch request, and deletes all confirmed ids in one database operation. It immediately fetches the
next bounded batch until the table is empty. A publish failure keeps the whole batch in the table and
it is retried on the next pass. Existing positional calls remain compatible; configure the limit with
`startProducerDbaFlusher(db, sweepInterval, batchSize)` or the named `batchSize` argument.

### Publish

Run to update maven package.

```shell
set -a
. ../../../.env
set +a
sbt --batch publish
```
