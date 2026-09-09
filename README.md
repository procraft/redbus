# Reliable Easy Data BUS

[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/prokraft/redbus/blob/master/LICENSE)

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

A failed result may also set `ConsumeRequest.Result.preserveAttempt` and `retryAfterSec`.
`preserveAttempt = true` keeps the current retry attempt from being exhausted, while a positive
`retryAfterSec` overrides the next delay. A non-positive delay falls back to the consumer's repeat
strategy, which prevents a malformed response from creating a busy retry loop. The Go and Scala SDKs
expose this combination as `NewRetryLaterError` and `RetryLaterException` respectively. Older clients
leave both fields at their protobuf defaults and retain the original retry behaviour.

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

Import [`deploy/grafana/redbus-overview.json`](./deploy/grafana/redbus-overview.json) into Grafana and select the
Prometheus data source to get the starter dashboard.
