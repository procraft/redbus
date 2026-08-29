package admincontrol

import (
	"testing"
	"time"

	controlpb "github.com/prokraft/redbus/internal/api/admincontrol"
	"github.com/stretchr/testify/require"
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
