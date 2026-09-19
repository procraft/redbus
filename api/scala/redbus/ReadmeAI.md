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
  Scala 3.3 LTS. The generator and runtime are aligned on ScalaPB 0.11.17 with sbt-protoc 1.0.8,
  so both Scala suffixes get the same protobuf definition and grpc/protobuf dependency line.

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

- `Client` — entry point: `produce`, `consume`, `startProducerDbaFlusher`, `close`.
- `producer.Producer.produce` — direct gRPC produce. `producer.Producer.produceDba` — transactional
  outbox: a `DBIOAction` that inserts into the client's `redbus_outbox` table (`api/outbox.sql`) so
  the message is committed together with the caller's own writes.
- `producer.Flusher` / `FlusherActor` — drains `redbus_outbox` into the bus.
- `consumer.Consumer` — bidirectional `Consume` stream with reconnect and the `consumer.Option.*`
  settings (repeat strategy, batch size, consume timeout, inbox-based only-once processing).
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
sbt --batch compile test
```

Publishing uses the fixed `maven.libicraft.ru` host and needs either both `MAVEN_USER` and
`MAVEN_PASSWORD` or `~/.sbt/1.0/credentials`; validation runs only for the `publish` task (see
`README.md`).
Publish both suffixes together with `+publish`, only after checking that neither coordinate for the
new version exists. Release artifacts are additive and must not be overwritten.
