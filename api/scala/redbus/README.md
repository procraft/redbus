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

### Transactional outbox flusher

`producer.Producer.produceDba` writes the message into the client's `redbus_outbox` table
(schema: `api/outbox.sql`) inside the caller's transaction. `Client.startProducerDbaFlusher(db)`
starts the flusher that delivers those rows to the bus: it reacts to the `pg_notify('redbus_outbox')`
sent by the table trigger and, in addition, sweeps the table on a fixed schedule — immediately at
start and then every `sweepInterval` (default 30 seconds, `startProducerDbaFlusher(db, sweepInterval)`).
The sweep delivers rows left over from a restart or a missed notification. Rows are sent in `id`
order; a produce failure keeps the row in the table and it is retried on the next pass.

### Publish

Run to update maven package.

```shell
pushd ../../.. && make export-env && popd
sbt publish
```
