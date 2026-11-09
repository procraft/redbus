# Kafka provider

This package reads broker metadata, group offsets and bounded partition heads for administration
and queue-health metrics. It must never join consumer groups or commit/reset their positions.

`GetTopicList` fetches committed offsets before high watermarks so progress during the snapshot
cannot produce negative lag. Missing/invalid/out-of-retention offsets stay unavailable.
`GetQueueSnapshot` reuses that snapshot and requests the first uncommitted record of each nonempty
partition with direct Kafka Fetch. Pinned kafka-go routes Fetch to the partition leader through
its transport's `protocol.BrokerMessage` implementation; the configured address is the bootstrap.
It inspects only record offset/time and closes exposed key/value byte references without reading
business data. The pinned RecordReader has no Close method; unused response buffers are released
by ordinary Go lifetime management. Fetch may return a complete first batch beyond its requested
1 MiB; metadata inspection is capped at 1024 records and every call has a one-second deadline.

Do not put these network calls in Prometheus Collect. The queuehealth background service refreshes
cached observations every30s with a20s deadline; the metrics collector expires them after120s.
Labels identify topic/group, not message/consumer/address. Group heads can belong to message
classes that the client will skip; head age is deliberately not an email-specific delivery delay.

Focused checks: `go test ./internal/pkg/kafka/provider ./internal/app/service/queuehealth ./internal/pkg/metrics`.
