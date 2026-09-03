package controlapi

import (
	"context"
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/api/admincontrol"
	"github.com/prokraft/redbus/internal/app/model"
	redruntime "github.com/prokraft/redbus/internal/pkg/runtime"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	restartError *string
	restartSince time.Time
}

func (s *repeaterStub) GetStat(context.Context) (model.RepeatStat, error) {
	return s.stat, nil
}

func (s *repeaterStub) RestartFailed(_ context.Context, topic, group string) error {
	s.restartTopic = topic
	s.restartGroup = group
	s.restartError = nil
	return nil
}

func (s *repeaterStub) RestartFailedSince(_ context.Context, topic, group string, since time.Time) error {
	s.restartTopic = topic
	s.restartGroup = group
	s.restartError = nil
	s.restartSince = since
	return nil
}

func (s *repeaterStub) RestartFailedByError(_ context.Context, topic, group, errorMessage string, since time.Time) error {
	s.restartTopic = topic
	s.restartGroup = group
	s.restartError = &errorMessage
	s.restartSince = since
	return nil
}

func TestControlApi(t *testing.T) {
	redruntime.SetStatic("2026-09-03T12:00:00Z")
	t.Cleanup(redruntime.ResetNowFn)
	firstFailedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	lastFailedAt := time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)
	repeater := &repeaterStub{stat: model.RepeatStat{{
		Topic: "orders", Group: "billing", AllCount: 4, FailedCount: 1, LastError: "failed",
		Errors: []model.RepeatErrorStat{{
			Error: "failed", FailedCount: 1, FirstFailedAt: firstFailedAt, LastFailedAt: lastFailedAt,
		}},
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
	require.Equal(t, int32(1), retries.GetList()[0].GetErrors()[0].GetFailedCount())
	require.Equal(t, firstFailedAt.UnixMilli(), retries.GetList()[0].GetErrors()[0].GetFirstFailedAtUnixMs())
	require.Equal(t, lastFailedAt.UnixMilli(), retries.GetList()[0].GetErrors()[0].GetLastFailedAtUnixMs())

	_, err = api.RestartFailed(context.Background(), &admincontrol.RestartFailedRequest{Topic: "orders", Group: "billing"})
	require.NoError(t, err)
	require.Equal(t, "orders", repeater.restartTopic)
	require.Equal(t, "billing", repeater.restartGroup)
	require.Nil(t, repeater.restartError)

	_, err = api.RestartFailedSince(context.Background(), &admincontrol.RestartFailedSinceRequest{
		Topic: "orders", Group: "billing", LookbackSeconds: int64((48 * time.Hour) / time.Second),
	})
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), repeater.restartSince)

	_, err = api.RestartFailedByError(context.Background(), &admincontrol.RestartFailedByErrorRequest{
		Topic: "orders", Group: "billing", Error: "failed", LookbackSeconds: int64((24 * time.Hour) / time.Second),
	})
	require.NoError(t, err)
	require.Equal(t, "orders", repeater.restartTopic)
	require.Equal(t, "billing", repeater.restartGroup)
	require.Equal(t, "failed", *repeater.restartError)
	require.Equal(t, time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), repeater.restartSince)

	_, err = api.RestartFailedSince(context.Background(), &admincontrol.RestartFailedSinceRequest{
		Topic: "orders", Group: "billing", LookbackSeconds: 0,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
