package databus

import (
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/stretchr/testify/require"
)

func TestMergeGroupStatsCombinesBrokerAssignmentsWithRuntimeMetrics(t *testing.T) {
	connectedAt := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	brokerGroup := model.StatGroup{
		Name: "billing", KafkaGroupId: "billing-orders", State: "Stable",
		PartitionList: []model.StatGroupPartition{{
			N: 0, Offset: 8, LastOffset: 10, Lag: 2, Committed: true, ConsumerId: "worker-1",
		}},
		ConsumerList: []model.StatConsumer{{
			Id: "worker-1", KafkaMemberId: "member-1", ClientHost: "/127.0.0.1",
			PartitionList: []model.StatConsumerPartition{{N: 0}},
		}},
	}
	runtimeGroup := model.StatGroup{
		Name: "billing",
		ConsumerList: []model.StatConsumer{{
			Id: "worker-1", Topic: "orders", Group: "billing", State: "connected",
			ConnectedAt: connectedAt, MessagesProcessed: 42, RepeatStrategy: "default",
		}},
	}

	mergeGroupStats(&brokerGroup, runtimeGroup)

	require.Len(t, brokerGroup.ConsumerList, 1)
	consumer := brokerGroup.ConsumerList[0]
	require.Equal(t, "connected", consumer.State)
	require.Equal(t, connectedAt, consumer.ConnectedAt)
	require.Equal(t, uint64(42), consumer.MessagesProcessed)
	require.Equal(t, "member-1", consumer.KafkaMemberId)
	require.Equal(t, "/127.0.0.1", consumer.ClientHost)
	require.Equal(t, []model.StatConsumerPartition{{
		N: 0, GroupOffset: 8, LastOffset: 10, Lag: 2, Committed: true,
	}}, consumer.PartitionList)
	require.Equal(t, "connected", brokerGroup.PartitionList[0].ConsumerState)
}
