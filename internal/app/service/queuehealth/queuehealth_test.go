package queuehealth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/stretchr/testify/require"
)

type groupStub struct{ groups model.TopicGroupList }

func (s *groupStub) GetConsumerTopicGroupList() model.TopicGroupList { return s.groups }

type kafkaStub struct {
	got    model.TopicGroupList
	values []model.QueueSnapshot
	err    error
}

func (s *kafkaStub) GetQueueSnapshot(ctx context.Context, groups model.TopicGroupList) ([]model.QueueSnapshot, error) {
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > RefreshTimeout {
		panic("unbounded snapshot")
	}
	s.got = groups
	return s.values, s.err
}

type retryStub struct {
	stats []model.RetryQueueStat
	err   error
	calls int
}

func (s *retryStub) GetQueueStats(ctx context.Context) ([]model.RetryQueueStat, error) {
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > RefreshTimeout {
		panic("unbounded retry snapshot")
	}
	s.calls++
	return s.stats, s.err
}

type metricsRecorder struct {
	values []model.QueueSnapshot
	source string
	counts []int
	err    error
}

func (s *metricsRecorder) SetQueueSnapshots(source string, values []model.QueueSnapshot, err error) {
	s.source, s.values, s.err = source, values, err
}
func (s *metricsRecorder) SetRetryRecords(pending, deferred, failed int) {
	s.counts = []int{pending, deferred, failed}
}

func TestKafkaContinuesMonitoringDisconnectedGroupAndPublishesFailures(t *testing.T) {
	groups := &groupStub{model.TopicGroupList{{Topic: "email", Group: "operational"}}}
	kafka := &kafkaStub{values: []model.QueueSnapshot{{Topic: "email", Group: "operational", Valid: true}}}
	metrics := &metricsRecorder{}
	service := New(kafka, groups, &retryStub{}, metrics)
	require.NoError(t, service.RefreshKafka(context.Background()))
	groups.groups = nil
	kafka.values = nil
	kafka.err = errors.New("broker unavailable")
	require.Error(t, service.RefreshKafka(context.Background()))
	require.Equal(t, model.TopicGroupList{{Topic: "email", Group: "operational"}}, kafka.got)
	require.Equal(t, "kafka", metrics.source)
	require.Len(t, metrics.values, 1)
	require.False(t, metrics.values[0].Valid)
}

func TestRetryUsesOneGroupedScanAndKeepsCountsOnFailure(t *testing.T) {
	oldest := time.Now().Add(-time.Hour)
	retry := &retryStub{stats: []model.RetryQueueStat{{Topic: "email", Group: "operational", All: 12, Failed: 3, Deferred: 4, Oldest: &oldest}}}
	metrics := &metricsRecorder{}
	service := New(&kafkaStub{}, &groupStub{model.TopicGroupList{{Topic: "email", Group: "auth"}}}, retry, metrics)
	require.NoError(t, service.RefreshRetry(context.Background()))
	require.Equal(t, 1, retry.calls)
	require.Equal(t, []int{5, 4, 3}, metrics.counts)
	require.Equal(t, "retry", metrics.source)
	require.Equal(t, []model.QueueSnapshot{
		{Topic: "email", Group: "operational", Oldest: oldest, Valid: true},
		{Topic: "email", Group: "auth", Valid: true},
	}, metrics.values)
	retry.err = errors.New("database timeout")
	require.Error(t, service.RefreshRetry(context.Background()))
	require.Equal(t, []int{5, 4, 3}, metrics.counts)
	require.Len(t, metrics.values, 1)
	require.False(t, metrics.values[0].Valid)
}

func TestKafkaPartialFailurePreservesSuccessfulGroup(t *testing.T) {
	groups := &groupStub{model.TopicGroupList{{Topic: "email", Group: "auth"}, {Topic: "email", Group: "operational"}}}
	kafka := &kafkaStub{values: []model.QueueSnapshot{{Topic: "email", Group: "auth", Valid: true}}, err: errors.New("one group failed")}
	metrics := &metricsRecorder{}
	service := New(kafka, groups, &retryStub{}, metrics)
	require.Error(t, service.RefreshKafka(context.Background()))
	require.Len(t, metrics.values, 2)
	require.True(t, metrics.values[0].Valid)
	require.False(t, metrics.values[1].Valid)
}
