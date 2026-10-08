package admincontrol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	controlpb "github.com/procraft/redbus/internal/api/admincontrol"
)

func TestConsumerFromProto(t *testing.T) {
	consumer := consumerFromProto(&controlpb.Consumer{
		Id:                  "worker-1",
		Topic:               "orders",
		Group:               "billing",
		State:               "connected",
		ConnectedAtUnixMs:   1_777_632_000_000,
		LastMessageAtUnixMs: 1_777_632_060_000,
		MessagesProcessed:   42,
		Partitions: []*controlpb.ConsumerPartition{{
			Number: 0, GroupOffset: 8, LastOffset: 10, Lag: 2, Committed: true,
		}},
	})

	require.Equal(t, "worker-1", string(consumer.Id))
	require.Equal(t, time.UnixMilli(1_777_632_000_000), consumer.ConnectedAt)
	require.NotNil(t, consumer.LastMessageAt)
	require.Equal(t, uint64(42), consumer.MessagesProcessed)
	require.Equal(t, int64(2), int64(consumer.PartitionList[0].Lag))
}

func TestRepeatErrorStatFromProto(t *testing.T) {
	result := repeatErrorStatFromProto(&controlpb.RetryErrorStat{
		Error: "boom <N>", SampleError: "boom 7", FailedCount: 3,
		FirstFailedAtUnixMs: 1_777_632_000_000,
		LastFailedAtUnixMs:  1_777_632_060_000,
	})

	require.Equal(t, "boom <N>", result.Error)
	require.Equal(t, "boom 7", result.Sample)
	require.Equal(t, 3, result.FailedCount)
	require.Equal(t, time.UnixMilli(1_777_632_000_000), result.FirstFailedAt)
	require.Equal(t, time.UnixMilli(1_777_632_060_000), result.LastFailedAt)
}

func TestConsumerFromProtoPreservesLagFailure(t *testing.T) {
	consumer := consumerFromProto(&controlpb.Consumer{Partitions: []*controlpb.ConsumerPartition{{
		Number: 1, Lag: -1, LagError: "offsets unavailable: LEADER_NOT_AVAILABLE",
	}}})
	require.Equal(t, "offsets unavailable: LEADER_NOT_AVAILABLE", consumer.PartitionList[0].LagError)
	require.Equal(t, int64(-1), int64(consumer.PartitionList[0].Lag))
}
