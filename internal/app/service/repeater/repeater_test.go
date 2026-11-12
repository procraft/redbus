package repeater

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/pb"
	"github.com/procraft/redbus/internal/app/model"
	"github.com/procraft/redbus/internal/app/service/connstore"
	"github.com/procraft/redbus/internal/pkg/runtime"
)

type repositoryStub struct {
	inserted *model.Repeat
	updated  []*model.Repeat
}

func (s *repositoryStub) Insert(_ context.Context, repeat model.Repeat) error {
	s.inserted = &repeat
	return nil
}
func (s *repositoryStub) FindForRepeat(context.Context, model.TopicGroupList) (model.RepeatList, error) {
	return nil, nil
}
func (s *repositoryStub) Delete(context.Context, int64) error { return nil }
func (s *repositoryStub) UpdateAttempt(_ context.Context, repeat *model.Repeat) error {
	s.updated = append(s.updated, repeat)
	return nil
}
func (s *repositoryStub) GetCount(context.Context) (model.RepeatCount, error) {
	return model.RepeatCount{}, nil
}
func (s *repositoryStub) GetStat(context.Context) (model.RepeatStat, error) { return nil, nil }
func (s *repositoryStub) GetTriageStat(context.Context, time.Time, time.Time, string, string) (model.RepeatTriageStat, error) {
	return model.RepeatTriageStat{}, nil
}
func (s *repositoryStub) RestartFailed(context.Context, string, string) error { return nil }
func (s *repositoryStub) RestartFailedSince(context.Context, string, string, time.Time) error {
	return nil
}
func (s *repositoryStub) RestartFailedByError(context.Context, string, string, string, time.Time) error {
	return nil
}
func (s *repositoryStub) DeleteFailedByError(context.Context, string, string, string) error {
	return nil
}

type connStoreStub struct {
	bag *connstore.ConsumerBag
}

func (connStoreStub) GetConsumerTopicGroupList() model.TopicGroupList { return nil }
func (s connStoreStub) FindBestConsumerBag(model.TopicName, model.GroupName, model.ConsumerId) *connstore.ConsumerBag {
	return s.bag
}

type repeatConsumerStub struct {
	model.IConsumer
}

func (repeatConsumerStub) GetHosts() []string            { return []string{"kafka:9092"} }
func (repeatConsumerStub) GetTopic() model.TopicName     { return "topic" }
func (repeatConsumerStub) GetGroup() model.GroupName     { return "group" }
func (repeatConsumerStub) GetID() model.ConsumerId       { return "consumer" }
func (repeatConsumerStub) GetState() model.ConsumerState { return model.ConsumerStateConnected }
func (repeatConsumerStub) Lock()                         {}
func (repeatConsumerStub) Unlock()                       {}

type repeatStreamStub struct {
	pb.RedbusService_ConsumeServer
	sent []*pb.ConsumeResponse
}

func (s *repeatStreamStub) Send(response *pb.ConsumeResponse) error {
	s.sent = append(s.sent, response)
	return nil
}

func (s *repeatStreamStub) Recv() (*pb.ConsumeRequest, error) {
	batch := s.sent[len(s.sent)-1]
	return &pb.ConsumeRequest{
		BatchId:    batch.BatchId,
		ResultList: []*pb.ConsumeRequest_Result{{Id: batch.MessageList[0].Id, Message: "handler error"}},
	}, nil
}

type metricsStub struct{}

func (metricsStub) ObserveRetryEnqueued(string, string, string)   {}
func (metricsStub) ObserveRetryAttempt(string, string, string)    {}
func (metricsStub) ObserveRetrySkipped(string, string, string)    {}
func (metricsStub) ObserveRepeaterRun(string, int, time.Duration) {}

func TestAddUsesConsumerRetryDelayForInitialFailure(t *testing.T) {
	runtime.SetStatic("2026-09-09T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	repo := &repositoryStub{}
	service := New(
		model.NewRepeatStrategyEven(5, model.Duration{Duration: time.Minute}),
		connStoreStub{},
		repo,
		metricsStub{},
	)

	err := service.Add(context.Background(), model.RepeatData{}, "provider throttled", true, 17*time.Minute)

	require.NoError(t, err)
	require.NotNil(t, repo.inserted)
	require.Equal(t, 0, repo.inserted.Attempt)
	require.True(t, repo.inserted.Deferred)
	require.Equal(t, "provider throttled", repo.inserted.Error)
	require.Equal(t, "2026-09-09T10:17:00Z", repo.inserted.StartedAt.Format(time.RFC3339))
}

func TestAddStoresOrdinaryFailureAsNotDeferred(t *testing.T) {
	repo := &repositoryStub{}
	service := New(
		model.NewRepeatStrategyEven(5, model.Duration{Duration: time.Minute}),
		connStoreStub{},
		repo,
		metricsStub{},
	)

	require.NoError(t, service.Add(context.Background(), model.RepeatData{}, "broken", false, 0))
	require.NotNil(t, repo.inserted)
	require.False(t, repo.inserted.Deferred)
}

func TestRepeatOutcomeSeparatesDeferredFromFailed(t *testing.T) {
	finishedAt := time.Now()
	require.Equal(t, "exhausted", repeatOutcome(&model.Repeat{FinishedAt: &finishedAt}))
	require.Equal(t, "deferred", repeatOutcome(&model.Repeat{Deferred: true}))
	require.Equal(t, "failed", repeatOutcome(&model.Repeat{}))
}

func TestRepeatDeliveryMarksOnlyAttemptThatWouldExhaustStrategy(t *testing.T) {
	server := &repeatStreamStub{}
	repo := &repositoryStub{}
	defaultStrategy := model.NewRepeatStrategyEven(1, model.Duration{Duration: time.Minute})
	service := New(defaultStrategy, connStoreStub{
		bag: &connstore.ConsumerBag{Consumer: repeatConsumerStub{}, Server: server},
	}, repo, metricsStub{})

	service.repeatProcessor(context.Background(), model.RepeatList{
		{MessageId: "first-retry", Attempt: 0},
		{MessageId: "last-retry", Attempt: 1},
	})

	require.Len(t, server.sent, 2)
	require.False(t, server.sent[0].MessageList[0].GetLastAttempt())
	require.True(t, server.sent[1].MessageList[0].GetLastAttempt())
	require.Len(t, repo.updated, 2)
	require.Nil(t, repo.updated[0].FinishedAt)
	require.NotNil(t, repo.updated[1].FinishedAt)
}
