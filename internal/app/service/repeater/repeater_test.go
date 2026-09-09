package repeater

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/app/service/connstore"
	"github.com/prokraft/redbus/internal/pkg/runtime"
)

type repositoryStub struct {
	inserted *model.Repeat
}

func (s *repositoryStub) Insert(_ context.Context, repeat model.Repeat) error {
	s.inserted = &repeat
	return nil
}
func (s *repositoryStub) FindForRepeat(context.Context, model.TopicGroupList) (model.RepeatList, error) {
	return nil, nil
}
func (s *repositoryStub) Delete(context.Context, int64) error                 { return nil }
func (s *repositoryStub) UpdateAttempt(context.Context, *model.Repeat) error  { return nil }
func (s *repositoryStub) GetCount(context.Context) (int, int, error)          { return 0, 0, nil }
func (s *repositoryStub) GetStat(context.Context) (model.RepeatStat, error)   { return nil, nil }
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

type connStoreStub struct{}

func (connStoreStub) GetConsumerTopicGroupList() model.TopicGroupList { return nil }
func (connStoreStub) FindBestConsumerBag(model.TopicName, model.GroupName, model.ConsumerId) *connstore.ConsumerBag {
	return nil
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

	err := service.Add(context.Background(), model.RepeatData{}, "provider throttled", 17*time.Minute)

	require.NoError(t, err)
	require.NotNil(t, repo.inserted)
	require.Equal(t, 0, repo.inserted.Attempt)
	require.Equal(t, "provider throttled", repo.inserted.Error)
	require.Equal(t, "2026-09-09T10:17:00Z", repo.inserted.StartedAt.Format(time.RFC3339))
}
