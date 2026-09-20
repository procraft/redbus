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
