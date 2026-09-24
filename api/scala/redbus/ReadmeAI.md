# Scala SDK — module context

`api/scala/redbus` is the Scala client SDK for the bus (`"sergiusd" %% "redbus"`), published to the
private maven repository. It is consumed by already-deployed services, each pinned to its own
version, so every behavioural change here is a release: bump `version` in `build.sbt`, publish, then
report which consumers must raise their pin.

## Actor runtime and release lines

The SDK uses an actor runtime as a scheduler and to serialise the state of two actors
(`FlusherActor`, `PostgresListener`). Which runtime that is, is part of the published contract:

- `0.2.x` — Akka 2.6.20, for consumers on Play 2.9. `0.2.8` is the last release of that line; a fix
  for an Akka consumer branches from its release commit, not from the Pekko line.
- `0.3.x` — Apache Pekko 1.0.3 (`org.apache.pekko`), the runtime Play 3.0.11 resolves, Scala 2.13
  and ScalaPB 0.10.11. `0.3.0` is the last release of that line.
- `0.4.x` — the same Pekko public API, cross-published from the same sources for Scala 2.13 and
  Scala 3.3 LTS. Since `0.4.1`, the generator and runtime are aligned on ScalaPB 0.11.20 with
  sbt-protoc 1.0.8 and use `grpc-netty-shaded`, so both Scala suffixes get the same protobuf
  definition and grpc/protobuf dependency line without exposing unshaded Netty classes.

The ScalaPB upgrade keeps the protobuf wire contract and handwritten client surface, but it removes
the generated `Message.Builder`/`HasBuilder` JVM API that existed in ScalaPB 0.10.11. Before raising a
consumer pin from `0.3.0` to `0.4.x`, scan it for those generated builders; normal case-class
construction, `copy`, message parsing and the Redbus `Client`/producer/consumer APIs are unchanged.
Do not overwrite `redbus_2.13:0.3.0`; it remains the rollback line for an unconverted consumer.

The split is forced, not cosmetic: `Client.startProducerDbaFlusher(db, …)(implicit as: ActorSystem)`
takes the *consumer's* system, so its type has to be the host's. A Play 3 service hands over a Pekko
system and cannot compile against 0.2.x; a Play 2.9 service cannot compile against 0.3.x.

Configuration: `Client` (`ActorSystem.create()`) and `Consumer` (`ActorSystem("ConsumerActorSystem")`)
build their own systems from `ConfigFactory.load()`, i.e. from the *host's* classpath configuration.
Pekko 1.0.3's `reference.conf` has a single `pekko` root and contains no `akka` key, and there is no
akka→pekko fallback, so:

- a host that renamed `akka { … }` to `pekko { … }` (what the Play 3 migration does anyway) keeps the
  same effective tuning for the SDK's systems as before;
- a host that left an `akka { … }` block behind gets it silently ignored — HOCON does not reject an
  unknown root — and the SDK's systems run on Pekko defaults. This is quiet, so it belongs in the
  migration checklist rather than in a runtime assertion;
- host-wide settings now reach the SDK: `pekko.actor.provider`, a resized
  `pekko.actor.default-dispatcher` or `pekko.coordinated-shutdown.*` apply to these systems too.

## Surfaces

- `ProtoClient` (since `0.4.4`) — the recommended entry point: ScalaPB-typed produce/outbox/consume
  over `Client`, configured by `RedbusSettings` (`fromConfig` reads `host`, `port`,
  `producerEnabled`, `consumerEnabled`, optional `outboxBatchSize`). It replaces the per-service
  `RedbusClient` wrappers; invariants below in *Typed client*.
- `Client` — low-level entry point: `produce`, `consume`, `startProducerDbaFlusher`, `close`.
  Unchanged for services pinned to older versions.
- `producer.Producer.produce` — direct gRPC produce. `producer.Producer.produceDba` — transactional
  outbox: a `DBIOAction` that inserts into the client's `redbus_outbox` table (`api/outbox.sql`) so
  the message is committed together with the caller's own writes.
- `producer.Flusher` / `FlusherActor` — drains `redbus_outbox` into the bus.
- `consumer.Consumer` — bidirectional `Consume` stream with reconnect and the `consumer.Option.*`
  settings (repeat strategy, batch size, consume timeout, inbox dedup — see below).
  It echoes `ConsumeResponse.batchId` back in the result request so the bus can tell the answer to
  the current batch from a late or foreign one, and declares `Connect.consumeTimeoutSec` so the
  bus sizes its own result deadline from the client's real processing budget. Both fields are
  optional on the wire: a bus that predates them ignores them. See
  `internal/pkg/stream/ReadmeAI.md` for the server side of that protocol.
  A processor may fail with `RetryLaterException(cause, delay)` for a temporary provider condition:
  the SDK sends `preserveAttempt = true` and a rounded-up `retryAfterSec`, so a compatible bus waits
  without consuming an attempt. Older buses ignore the additive fields and apply their normal retry
  strategy.
- ScalaPB code is generated from `api/api.proto` at build time; never edit generated sources.

## Typed client

- A disabled side never touches the bus: `produceProto` → `false`, `produceProtoDba` →
  `DBIOAction.successful(0)`, `consumeProto` → `Future.unit`, `startFlusher` → nothing. The gRPC
  `Client` (and its actor system) is created lazily and only when a side is enabled.
- Decoding lives in `ProtoClient.decoding`: a payload that fails `parseFrom` is logged with topic,
  group and size and acknowledged; a synchronous processor exception becomes a failed future for
  that message. Only the typed path does this — the byte-level `Client`/`Consumer` keep their
  behaviour (a synchronous throw still fails the whole batch and reconnects), because deployed
  services must not see a change.
- `startFlusher` is guarded by an `AtomicBoolean` per client instance. The SDK flusher actor has a
  fixed name, so two clients on one actor system still clash.
- The inbox option is prepended to the caller's consumer options, so a caller option given later
  wins, the same as for the lower-level `Client`.
- Database parameters of the new API take `slick.jdbc.JdbcBackend#JdbcDatabaseDef`. Slick types a
  database by its profile's path (`profile.backend.Database`), so an application's own
  `PostgresProfile` subclass produced a type that did not match `PostgresProfile.backend.Database`
  and forced an `asInstanceOf`. All profiles share the `JdbcBackend` object and one erased class, so
  `JdbcDatabases.postgres` narrows it internally. The older signatures keep
  `PostgresProfile.backend.Database` for compatibility.
- `ProtoClient.Transport` is the package-private seam that `ProtoClientSpec` replaces.

## Inbox dedup modes

Both consumer modes use the client's `redbus_inbox` table (`api/inbox.sql`) with the key
`group|topic|idempotencyKey`, where the message id stands in for an empty idempotency key. The key
is built only in `IncomeMessages`, so marks written by either mode are visible to the other and a
consumer can switch modes without a migration. The stream-independent logic lives in
`consumer.InboxProcessing` (package-private, with a `Store` seam for unit tests).

`Option.WithInbox(db, InboxMode)` (since `0.4.4`) selects the mode in one option: `OnlyOnce`,
`Transactional` or `Disabled`. `WithOnlyOnceProcessor` and `WithTransactionalInbox` delegate to it and
set exactly the same `Listener` fields as before. `Inbox.guard(claim, onSkip)(fn)` is the processor
side of the transactional mode: claim first, skip `fn` on `false`, always run `fn` without a claim.

- `Option.WithOnlyOnceProcessor(db)` — pre-check `isProcessed`, run the processor, then
  `setProcessed` after success. Three separate database steps: a crash after the processor's commit
  and before the mark, or a concurrent redelivery (rebalance, batch timeout), processes the message
  twice. Kept unchanged for existing consumers.
- `Option.WithTransactionalInbox(db)` (since `0.4.3`) — the same cheap pre-check, but the SDK never
  writes the mark. The processor gets `MessageMeta.claimDba = Some(IncomeMessages.claim(...))`:
  `INSERT … (key, created_at) VALUES (…, now()) ON CONFLICT ("key") DO NOTHING`, `true` only when
  this call inserted the row.

Processor contract in the transactional mode: run `claimDba` as the first step of the **same**
transaction as the business writes; on `false` skip the business logic and complete successfully
(the message was already processed). The SDK never opens that transaction, so the host keeps its own
transaction wrapper and post-commit side effects. A concurrent claim of the same key blocks on the
primary key until the first transaction ends: it then gets `false` after a commit, or claims the key
after a rollback. Side effects outside the database (external calls) are not covered by the claim
and still need their own idempotency.

The two options are mutually exclusive; the last one given wins. `claimDba` stays `None` without the
transactional option. `IncomeMessages.claim(group, topic, idempotencyKey)` is public for tests and
for hosts that build the key themselves.

Client tables may differ from `api/inbox.sql`: some were created by renaming an older table, and
their `created_at` is `NOT NULL` without a default. Every SDK write therefore sets `created_at`
itself (`setProcessed` from the client clock, `claim` with `now()`). `0.4.2` shipped a `claim` that
relied on the column default and fails on such tables with a not-null violation; it is published
and immutable, so consumers of the transactional mode must use `0.4.3` or later.

`IncomeMessagesPostgresSpec` runs the real SQL against a local PostgreSQL, on both table shapes
(with and without the `created_at` default), and checks that a concurrent claim waits for the
first transaction. It connects to the maintenance database `postgres`, creates uniquely named
schemas and drops them afterwards; `REDBUS_PG_SPEC_URL` overrides the JDBC URL and
`REDBUS_PG_SPEC=false|0|off` turns it off. An unreachable server cancels these tests (look for
`CANCELED` in the sbt output) instead of failing them. The package-private `…In(schema, …)` variants
in `IncomeMessages` exist only for this spec; the public API always targets `public.redbus_inbox`.

## Outbox flusher invariants

- Triggers: `pg_notify('redbus_outbox')` from the table trigger (polled every 100 ms by
  `PostgresListener`) **and** a periodic sweep (`sweepInterval`, default
  `Flusher.defaultSweepInterval` = 30 s, first sweep immediately at start). The sweep is what
  delivers rows left over from a restart or a missed notification — do not remove it.
- One pass at a time. A trigger arriving during a pass sets `pending`, and another pass starts right
  after the current one finishes. Actor state is mutated only inside `receive`; the future completion
  reports back with `ProcessingFinished` via `self`.
- Rows are sent in `id` order. A produce failure (failed future or `ok = false`) stops the pass,
  logs through the client `logger`, keeps the row and is retried on the next trigger or sweep. This
  is the expected behaviour while the bus is unavailable.
- Each query fetches at most `batchSize` rows (default 100) in `id` order. A batch ends at the first
  topic change, is sent through one confirmed `ProduceBatch` call, and its ids are deleted together
  only after the whole call succeeds. The same pass keeps fetching bounded batches until the outbox
  is empty.
- A Kafka partial error or context cancellation is ambiguous: the whole selected batch stays in the
  outbox and may produce duplicates on retry, so consumer idempotency remains required. `batchSize`
  bounds row count, not serialized bytes; with the server's current default gRPC receive limit, a
  request above 4 MiB is rejected and retried until configuration or batching policy changes.
- `FlusherActor` takes a `Flusher.Store` (package-private constructor) so the pass logic is unit
  tested without a database; production uses `Flusher.SlickStore`.
- `PostgresListener` opens its own JDBC connection from the given Slick `Database` for `LISTEN`;
  the flusher must be started once per process.

## Checks

```shell
sbt --batch +compile +test +makePom
```

From the repository root, `make scala-proto-compile` additionally compiles the Scala example against
the SDK from the same checkout.

Publishing uses the fixed `maven.libicraft.ru` host and needs either both `MAVEN_USER` and
`MAVEN_PASSWORD` or `~/.sbt/1.0/credentials`; validation runs only for the `publish` task (see
`README.md`).
Publish both suffixes together with `+publish`, only after checking that neither coordinate for the
new version exists. Release artifacts are additive and must not be overwritten.
