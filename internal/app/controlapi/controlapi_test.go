package controlapi

import (
	"context"
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/api/admincontrol"
	"github.com/prokraft/redbus/internal/app/model"

	"github.com/stretchr/testify/require"
)

type dataBusStub struct {
	stat   model.Stat
	topics model.StatTopicList
}

func (s dataBusStub) GetStat(context.Context) (model.Stat, error) {
	return s.stat, nil
}

func (s dataBusStub) GetTopicList(context.Context) (model.StatTopicList, error) {
	return s.topics, nil
}

type repeaterStub struct {
	stat         model.RepeatStat
	restartTopic string
	restartGroup string
}

func (s *repeaterStub) GetStat(context.Context) (model.RepeatStat, error) {
	return s.stat, nil
}

func (s *repeaterStub) RestartFailed(_ context.Context, topic, group string) error {
	s.restartTopic = topic
	s.restartGroup = group
	return nil
}

func TestControlApi(t *testing.T) {
	repeater := &repeaterStub{stat: model.RepeatStat{{
		Topic: "orders", Group: "billing", AllCount: 4, FailedCount: 1, LastError: "failed",
	}}}
	api := New(dataBusStub{
		stat: model.Stat{ConsumeTopicCount: 2, ConsumerCount: 3, RepeatAllCount: 4, RepeatFailedCount: 1},
		topics: model.StatTopicList{{
			Name:          "orders",
			PartitionList: []model.StatPartition{{N: 1, FirstOffset: 10, LastOffset: 20}},
			GroupList: []model.StatGroup{{
				Name: "billing", KafkaGroupId: "billing-orders", State: "Stable",
				ConsumerList: []model.StatConsumer{{
					Id: "worker-1", Topic: "orders", Group: "billing", State: "connected",
					ConnectedAt:       time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC),
					MessagesProcessed: 42,
					PartitionList: []model.StatConsumerPartition{{
						N: 1, GroupOffset: 15, LastOffset: 20, Lag: 5, Committed: true,
					}},
				}},
				PartitionList: []model.StatGroupPartition{{
					N: 1, Offset: 15, FirstOffset: 10, LastOffset: 20, Lag: 5, Committed: true,
					ConsumerId: "worker-1", ConsumerState: "connected",
				}},
			}},
		}},
	}, repeater)

	snapshot, err := api.GetStateSnapshot(context.Background(), &admincontrol.Empty{})
	require.NoError(t, err)
	require.Equal(t, int32(3), snapshot.GetConsumerCount())
	require.Equal(t, int32(1), snapshot.GetRepeatFailedCount())

	topics, err := api.GetTopicStats(context.Background(), &admincontrol.Empty{})
	require.NoError(t, err)
	require.Equal(t, "worker-1", topics.GetList()[0].GetGroups()[0].GetPartitions()[0].GetConsumerId())
	require.Equal(t, "connected", topics.GetList()[0].GetGroups()[0].GetConsumers()[0].GetState())
	require.Equal(t, int64(5), topics.GetList()[0].GetGroups()[0].GetPartitions()[0].GetLag())
	require.Equal(t, uint64(42), topics.GetList()[0].GetGroups()[0].GetConsumers()[0].GetMessagesProcessed())

	retries, err := api.GetRetryStats(context.Background(), &admincontrol.Empty{})
	require.NoError(t, err)
	require.Equal(t, "failed", retries.GetList()[0].GetLastError())

	_, err = api.RestartFailed(context.Background(), &admincontrol.RestartFailedRequest{Topic: "orders", Group: "billing"})
	require.NoError(t, err)
	require.Equal(t, "orders", repeater.restartTopic)
	require.Equal(t, "billing", repeater.restartGroup)
}
