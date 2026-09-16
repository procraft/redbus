## Redbus service scala API and SDK.

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
pushd ../../.. && make export-env && popd
sbt publish
```
