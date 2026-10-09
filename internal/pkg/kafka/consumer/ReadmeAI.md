# Kafka consumer

This package owns Kafka reads, commits and conversion to bus messages. Read this before changing
message identity or acknowledgement behavior. Retry persistence belongs to
[the repeater](../../../app/service/repeater/repeater.go); SDK inbox policy belongs to
[the client SDKs](../../../../api/golang/ReadmeAI.md).

## Message identity

`toMessageList` uses each record's own partition and offset. The consumer's optional
`messageIdNamespace` prefixes only the opaque message ID; keys, payloads and headers, including
producer idempotency keys, are unchanged. An empty namespace retains `partition/offset` exactly.
SDK inboxes fall back to this ID only when the producer supplied no idempotency key.

The namespace identifies a Kafka data generation, not a pod or deployment. Every bus instance reading
the same data must agree; keep the value when reusing the disk/PVC, and choose a new value after data
is recreated. Avoid mixed namespaces or old/new bus versions consuming a replacement cluster during
rollout. Returning to an original unprefixed cluster requires the empty namespace. Changing the
namespace on retained data changes fallback deduplication keys and can repeat business processing.
Persisted retries keep their original IDs and headers; do not rewrite them during a switch.

## Commits

Processing completes before `CommitMessages`. Group offsets are Kafka positions, independent of the
message ID namespace. This setting neither moves records nor transfers group offsets between clusters.
