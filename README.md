# Reliable Easy Data BUS

[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/procraft/redbus/blob/master/LICENSE)

<img src="./doc/logo.png" height="347" alt="RED Bus logo"/>

RED Bus allows you to publish messages and process them with control over the result. You can set a retry
strategy for each consumer and the number of retries after which a message will be marked as failed.
The administrator can start reprocessing of unsuccessfully processed messages after fixing the problem through the
web interface.

## Issue

When you use messages in your system to maintain eventual consistency, it's important that every message is processed, 
and you must handle errors and implement a retry algorithm in every service in your system. You also need a tool to 
view failed messages and the ability to reprocess on demand.

## Resolve

Produce messages from anywhere and process them with repeated retries.  
RED Bus will do the rest for you.

![RED Bus Diagram](http://www.plantuml.com/plantuml/proxy?src=https://raw.githubusercontent.com/sergiusd/redbus/master/doc/resolve.puml)

## Build

```shell
    make build
```

## How to try

1. Set configuration 

   See `config.json` and `config/config.go`. Environment variable overwrited json data.
   For development environment you might use `config.local.json`


2. Start essential environment

   ```shell
       docker compose -f example/docker-compose.yml up   
   ```

3. Run data bus service

   ```shell
       ./bin/redbus
   ```

4. Start the admin panel

   ```shell
       cd web/admin
       nvm install 24
       nvm use 24
       corepack enable
       yarn install --immutable
       yarn dev
   ```

   `nvm install 24` and `corepack enable` only need to be run once. Use `nvm use 24` when opening a new shell before
   running the admin panel.

   Open <http://localhost:8081> in your browser. By default, the admin panel connects to the RED Bus admin API at
   `http://localhost:50006` using the settings from `web/admin/.env`.

5. Try consumer and producer

   - [GoLang client](./example/golang/README.md)
   - [Scala client](./example/scala/README.md)

## Consume stream safety

The bus protects itself from a client that stops answering. Relevant `config.json` keys (each with a
`REDBUS_*` environment override):

| Key | Default | Meaning |
|---|---|---|
| `grpc.keepaliveTime` | `30s` | how often an idle connection is pinged |
| `grpc.keepaliveTimeout` | `10s` | how long a ping answer is awaited before the connection is dropped |
| `grpc.keepaliveMinTime` | `10s` | minimum ping interval the server accepts from a client |
| `grpc.consumeResultTimeout` | `60s` | per message budget used when a client does not declare `Connect.consumeTimeoutSec` |
| `grpc.consumeResultSlack` | `30s` | extra time added on top of `budget * batch size` |
| `grpc.consumeResultTimeoutMax` | `1h` | upper bound of the computed budget |
| `log.json` | `false` | emit structured JSON log lines instead of plain text |
| `log.verbose` | `false` | include debug level messages |

A batch whose result does not arrive within the computed budget closes the consume stream with an
error, so the client sees the failure and reconnects instead of staying connected while holding
Kafka partitions.

The server tags every batch with `ConsumeResponse.batchId`; a client echoes it back in
`ConsumeRequest.batchId`. A result carrying a different batch id is discarded with a log entry
instead of shifting the request/response phase of the stream. Both fields are optional, so clients
built before they existed keep working against a newer bus.

A result with `ok = false` may also set `ConsumeRequest.Result.retryLater` and `retryAfterSec`.
`retryLater = true` means deferred processing rather than a failure: the consumer could not take the
message yet (a rate limiter, a busy provider) and asks for a later delivery. The bus keeps the current
retry attempt, never finishes the retry because of a deferral and stores it as *deferred*, so the
admin UI and the statistics count it apart from errors. A positive `retryAfterSec` overrides the next
delay; a non-positive delay falls back to the consumer's repeat strategy, which prevents a malformed
response from creating a busy retry loop. The Go and Scala SDKs expose this as `NewRetryLaterError`
and `RetryLaterException` respectively. Older clients leave both fields at their protobuf defaults and
retain the original retry behaviour. (`retryLater` was called `preserveAttempt` before; the field
number and wire format are unchanged.)

A retry record is therefore in one of three states:

| State | Meaning | Shown as |
|-------|---------|----------|
| pending | waiting for the next attempt after an ordinary failure | part of the retries |
| deferred | waiting after a `retryLater` result; an ordinary failure clears the mark | *Deferred*, never an error |
| failed | attempts exhausted (`finished_at` set); waits for a manual restart | *Failed* |

## Prometheus metrics

The Redbus process exposes Prometheus metrics at `http://localhost:50008/metrics`. Set
`metrics.serverPort` in `config.json` or `REDBUS_METRICS_SERVER_PORT`; use `0` to disable the HTTP endpoint.

Minimal Prometheus scrape configuration:

```yaml
scrape_configs:
  - job_name: redbus
    scrape_interval: 15s
    static_configs:
      - targets: ["redbus:50008"]
```

The endpoint includes Go/process metrics and Redbus metrics for produce/consume throughput, consumer state,
processing latency, Kafka reconnects, retry processing, gRPC calls, and the PostgreSQL connection pool. Labels are
limited to bounded values such as topic, group, result, and state; message and consumer identifiers are not exported.
Deferrals have their own label values: `redbus_consumed_messages_total{result="deferred"}`,
`redbus_retry_attempts_total{outcome="deferred"}` and `redbus_retry_records{state="deferred"}` (which is not part of
`state="pending"`).

Consumer health:

- `redbus_active_consumers{topic, group, state}` — consume streams by state (`connecting`, `connected`,
  `reconnecting`). A stream becomes `connected` only after its first batch or after 30 s of reading Kafka without
  an error, so a consumer looping on a Kafka error (e.g. `TOPIC_AUTHORIZATION_FAILED`) stays in
  `connecting`/`reconnecting` instead of flickering into `connected`. A series stays at `0` after its stream ends,
  until the bus restarts.
- `redbus_kafka_consumer_reconnects_total{topic, group, reason}` — Kafka read failures followed by a reconnect;
  `reason` is `authorization` (Kafka error 29), `rebalance` or `other`.
- `redbus_consumer_connections_total{topic, group, result}` — consume stream connection attempts, `result` is
  `success` or `error` (the first Kafka reader could not be created).

## Logs in Loki

The bus can push its own log lines to Loki (there is no collector agent required):

| Key | Env | Default | Meaning |
|---|---|---|---|
| `log.loki.url` | `REDBUS_LOKI_URL` | empty (off) | push endpoint, e.g. `https://loki.example/loki/api/v1/push` |
| `log.loki.username` | `REDBUS_LOKI_USERNAME` | empty | basic auth user |
| `log.loki.password` | `REDBUS_LOKI_PASSWORD` | empty | basic auth password |
| `log.loki.app` | `REDBUS_LOKI_APP` | `redbus` / `redbus-admin` | `app` label; the default is the process name |
| `log.loki.env` | `REDBUS_LOKI_ENV` | empty (no label) | `env` label, e.g. `prod` or `stage` when several environments share one Loki |
| `log.loki.level` | `REDBUS_LOKI_LEVEL` | `info` | lowest pushed level: `debug`, `info`, `warning`, `error` |

Streams carry the labels `app`, `l` (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`) and, when set, `env`. Lines are queued and pushed in
batches every 2 seconds; logging never waits for Loki: a full queue drops lines and the next push reports how many.
The queue is flushed on shutdown (bounded by 5 seconds). Lines written through the standard `log` package bypass
the sink.

Import [`deploy/grafana/redbus-overview.json`](./deploy/grafana/redbus-overview.json) into Grafana and select the
Prometheus data source to get the starter dashboard.
