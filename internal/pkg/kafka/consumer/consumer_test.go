package consumer

import (
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"

	kpkg "github.com/prokraft/redbus/internal/app/model"
	redbusruntime "github.com/prokraft/redbus/internal/pkg/runtime"
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

func TestToMessageListKeepsPartitionOfEachMessage(t *testing.T) {
	list := toMessageList([]kafka.Message{
		{Partition: 0, Offset: 10, Value: []byte("a")},
		{Partition: 3, Offset: 10, Value: []byte("b")},
		{Partition: 7, Offset: 42, Value: []byte("c"), Headers: []kafka.Header{{Key: "k", Value: []byte("v")}}},
	})

	require.Equal(t, []string{"0/10", "3/10", "7/42"}, list.GetIdList())
	require.Len(t, list.IndexByID(), 3, "id разных партиций не должны схлопываться")
	require.Equal(t, map[string]string{"k": "v"}, list[2].Headers)
}
