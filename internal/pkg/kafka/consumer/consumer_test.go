package consumer

import (
	"testing"
	"time"

	kpkg "github.com/prokraft/redbus/internal/app/model"
	redbusruntime "github.com/prokraft/redbus/internal/pkg/runtime"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestSetOffsetStoresNextCommittedPosition(t *testing.T) {
	redbusruntime.SetStatic("2026-08-29T12:00:00Z")
	defer redbusruntime.ResetNowFn()
	consumer := Consumer{
		state:     int32(kpkg.ConsumerStateConnecting),
		offsetMap: make(kpkg.PartitionOffsetMap),
	}

	consumer.setOffset([]kafka.Message{
		{Partition: 0, Offset: 9},
		{Partition: 1, Offset: 4},
		{Partition: 0, Offset: 10},
	})

	require.Equal(t, kpkg.PartitionOffsetMap{0: 11, 1: 5}, consumer.GetOffsetMap())
	metrics := consumer.GetMetrics()
	require.Equal(t, uint64(3), metrics.MessagesProcessed)
	require.Equal(t, time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC), metrics.LastMessageAt)

	consumer.SetState(kpkg.ConsumerStateConnected)
	require.Equal(t, kpkg.ConsumerStateConnected, consumer.GetState())
	require.Equal(t, metrics.LastMessageAt, consumer.GetMetrics().StateSince)
}
