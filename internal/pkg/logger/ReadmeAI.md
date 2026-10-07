# Logger — module context

`internal/pkg/logger` is the bus's own leveled logger (`Debug`…`Fatal`, `Consumer`, `Produce`) and
its optional Loki sink. Every package logs through it; configuration lives in `internal/config`
(`log.*`, `REDBUS_LOG_*`, `REDBUS_LOKI_*`), wiring in `cmd/redbus` and `cmd/redbus-admin`.

## Loki sink

- The platform has no log collector in the cluster: each service pushes its own lines to Loki over
  HTTP. Until 2026-10 the bus did not, and its Kafka errors were visible only through `kubectl logs`.
- Labels: `app` and `l` (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`) as in the other Go services
  (lms-whatsapp), plus `env` from `REDBUS_LOKI_ENV` as in the Scala services (`env=prod|stage`): one
  Loki serves both environments. An empty env adds no label. The Scala services also label `role`
  and keep the level in the line, so a query across both kinds has to account for that.
- `Log` only enqueues; a full queue (10 000 lines) drops the line and the next push reports the
  count. The worker pushes every 2 s or per 500 lines, one request at a time, so a slow Loki fills
  the queue instead of blocking the bus. Push failures go to the standard `log` package (never back
  into the sink), at most once a minute.
- `StartLoki` returns the stop function; `cmd/*` call it on exit with a 5 s bound, and `Fatal` flushes
  before `os.Exit`. Lines printed with the standard `log` package (e.g. the Kafka writer's
  `WithLog` dump) bypass the sink by design.
- `log.loki.level` (default `info`) bounds the pushed volume. `stream` logs every message result at
  info, so `warning` is the lever if Loki volume becomes a problem; Kafka read and reconnect errors
  are logged at warning (`ConsumerWarning`) for that reason.
