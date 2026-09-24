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
| `0.3.x` (last: `0.3.0`) | Pekko 1.0.3 | Play 3.0 | 2.13 |
| `0.4.x` | Pekko 1.0.3 | Play 3.0 | 2.13, 3.3 LTS |

The Akka and Pekko lines are not interchangeable — pick the one matching the host application, and
branch fixes for Akka consumers from the commit that last released `0.2.8` instead of from the
Pekko line. Starting with `0.4.0`, the Pekko SDK is published for both Scala 2.13 and Scala 3.3 LTS
from the same sources and protobuf definition. Use the normal `%%` dependency syntax so sbt selects
`redbus_2.13` or `redbus_3` for the host Scala version.

`0.4.0` also moves generated messages from ScalaPB 0.10.11 to the ScalaPB 0.11 line; `0.4.1`
aligns the compiler and runtime on 0.11.20 and uses the shaded gRPC Netty transport. The wire format
and the handwritten Redbus client API stay unchanged, but ScalaPB 0.11 no longer generates the legacy
`Message.Builder`/`HasBuilder` JVM API. Consumers using case-class constructors and `copy` are
unaffected; a consumer that called those generated builder classes must migrate before raising its
pin. The old `redbus_2.13:0.3.0` artifact remains available for unchanged consumers.

The SDK builds its own actor systems with the default configuration (`ConfigFactory.load()`), so it
picks up the host application's `pekko { … }` section. Pekko 1.0.3 defines no `akka` keys at all and
silently ignores a leftover `akka { … }` block, so a host that has not renamed its configuration
just falls back to Pekko defaults — no error, no warning. Keep host-wide settings such as
`pekko.actor.provider` or a resized `pekko.actor.default-dispatcher` in mind: they now apply to the
SDK's systems too.

### Typed client (recommended entry point)

Since **0.4.4**, `ProtoClient` covers what every service used to write in its own `RedbusClient`
wrapper: settings, the enabled/disabled switches, ScalaPB encoding and decoding, the outbox flusher
and the inbox mode. The lower-level `Client` stays available and unchanged.

```scala
import sergiusd.redbus.{ProtoClient, RedbusSettings}
import sergiusd.redbus.consumer.{Inbox, InboxMode}

val bus = ProtoClient(
  RedbusSettings.fromConfig(config.getConfig("app.redbus")), // host, port, producerEnabled, consumerEnabled[, outboxBatchSize]
  db,                                  // any Slick JdbcProfile database, no cast needed
  lifecycle.addStopHook,               // consumer shutdown registration
  ProtoClient.Log(debug = log.debug(_), info = log.info(_), error = log.error(_, _)),
)

bus.startFlusher()                     // needs an implicit ActorSystem; repeated calls do nothing
bus.produceProto("topic", message, producer.Option.WithIdempotencyKey(key))   // Future[Boolean]
bus.produceProtoDba("topic", message)  // DBIOAction inside the caller's transaction

bus.consumeProto[Request]("topic", "group", InboxMode.Transactional, consumer.Option.WithBatchSize(10)) {
  (request, meta) =>
    db.run(Inbox.guard(meta.claimDba, log.info("already processed"))(businessWrites(request)).transactionally)
}
```

- A disabled side is a no-op: `produceProto` returns `false`, `produceProtoDba` writes no row,
  `consumeProto` completes immediately and `startFlusher` starts nothing.
- A payload that does not decode as the requested message is logged through `Log.error`
  (`redbus.receive.invalid: topic=… group=… payloadBytes=… action=drop`) and acknowledged, because a
  retry cannot fix it. An exception the processor throws synchronously fails only that message.
- `InboxMode` is `OnlyOnce`, `Transactional` or `Disabled`. `consumer.Option.WithInbox(db, mode)` is
  the same choice as a single option for the lower-level `Client`; the older `WithOnlyOnceProcessor`
  and `WithTransactionalInbox` now delegate to it and behave as before.
- `Inbox.guard(claim, onSkip)(fn)` runs the transactional claim before `fn`, skips `fn` when the
  message was already processed, and always runs `fn` when there is no claim (a call path that does
  not come from the bus).
- The flusher actor has a fixed name, so start it from one client per actor system.

Migrating a service wrapper: build `RedbusSettings` from the existing config section (or construct it
from the service's typed config), replace the private `redbus.Client` with `ProtoClient`, delete the
local enabled checks, the `parseFrom` try/catch, the `asInstanceOf` on the database, the inbox-mode
enum and the claim helper, and pass the idempotency key as `producer.Option.WithIdempotencyKey`. Keep
topic and group constants and application-specific error mapping in the service.

### Verify

From this directory, compile and test both published Scala lines and generate both release POMs:

```shell
sbt --batch +compile +test +makePom
```

From the repository root, also compile the Scala example that exercises the SDK's public API:

```shell
make scala-proto-compile
```

### Temporary processing failures

Return a failed future with `consumer.RetryLaterException(cause, delay)` when an external provider
asks the consumer to wait (for example, a rate limit). A compatible Redbus server schedules the next
delivery after the requested delay without consuming a retry attempt. Servers older than this SDK
ignore the additive result fields and use their configured repeat strategy.

### Transactional inbox

`consumer.Option.WithOnlyOnceProcessor(db)` skips messages already recorded in the client's
`redbus_inbox` table (schema: `api/inbox.sql`) and records a message after the processor succeeds.
The record is written separately from the processor's own writes, so a crash in between or a
concurrent redelivery can process a message twice.

Since **0.4.3**, `consumer.Option.WithTransactionalInbox(db)` closes that gap. The SDK still skips
recorded messages, but it no longer writes the record itself. The processor receives
`MessageMeta.claimDba`, a `DBIOAction[Boolean, NoStream, Effect.Write]` that inserts the record with
`INSERT … ON CONFLICT DO NOTHING` and sets `created_at` itself, so the table's column default is
not required. Run it as the first step of the same transaction as the business
writes; `false` means the message was already processed, so skip the business logic and return a
successful future:

```scala
def process(data: Array[Byte], meta: MessageMeta): Future[Unit] = {
  val claim = meta.claimDba.getOrElse(DBIO.successful(true))
  db.run((for {
    claimed <- claim
    _ <- if (claimed) businessWrites(data) else DBIO.successful(())
  } yield ()).transactionally)
}
```

`Inbox.guard` (see *Typed client*) wraps this pattern. Both options use the same table and key, so
switching between them needs no migration. Use
`consumer.IncomeMessages.claim(group, topic, idempotencyKey)` to build the same action in tests.

Do not use **0.4.2** for this mode: its claim relied on the `created_at` default from
`api/inbox.sql` and fails on tables where the column is `NOT NULL` without a default.

`IncomeMessagesPostgresSpec` needs a local PostgreSQL (the `postgres` database by default). Override
the JDBC URL with `REDBUS_PG_SPEC_URL` or turn the spec off with `REDBUS_PG_SPEC=false`; without a
reachable server its tests are reported as canceled.

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

Every release is published as a pair of Scala 2.13 and Scala 3 artifacts. Before publishing, verify
that neither target coordinate already exists: Artifactory releases are immutable and must not be
overwritten. The repository host is fixed to `maven.libicraft.ru`. Provide both `MAVEN_USER` and
`MAVEN_PASSWORD`; if neither is set, sbt falls back to `~/.sbt/1.0/credentials`. Setting only one
environment variable fails before publishing.

```shell
set -a
. ../../../.env
set +a
sbt --batch +publish
```
