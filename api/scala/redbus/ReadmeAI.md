# Scala SDK — module context

`api/scala/redbus` is the Scala client SDK for the bus (`"sergiusd" %% "redbus"`), published to the
private maven repository. It is consumed by already-deployed services, each pinned to its own
version, so every behavioural change here is a release: bump `version` in `build.sbt`, publish, then
report which consumers must raise their pin.

## Surfaces

- `Client` — entry point: `produce`, `consume`, `startProducerDbaFlusher`, `close`.
- `producer.Producer.produce` — direct gRPC produce. `producer.Producer.produceDba` — transactional
  outbox: a `DBIOAction` that inserts into the client's `redbus_outbox` table (`api/outbox.sql`) so
  the message is committed together with the caller's own writes.
- `producer.Flusher` / `FlusherActor` — drains `redbus_outbox` into the bus.
- `consumer.Consumer` — bidirectional `Consume` stream with reconnect and the `consumer.Option.*`
  settings (repeat strategy, batch size, consume timeout, inbox-based only-once processing).
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
- `FlusherActor` takes a `Flusher.Store` (package-private constructor) so the pass logic is unit
  tested without a database; production uses `Flusher.SlickStore`.
- `PostgresListener` opens its own JDBC connection from the given Slick `Database` for `LISTEN`;
  the flusher must be started once per process.

## Checks

```shell
sbt --batch compile test
```

Publishing needs `MAVEN_HOST`, `MAVEN_USER`, `MAVEN_PASSWORD` (see `README.md`).
