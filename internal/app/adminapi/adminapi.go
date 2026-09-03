package adminapi

import (
	"context"

	"github.com/prokraft/redbus/internal/app/model"
)

type IService interface {
	Health(ctx context.Context) error
	GetStateSnapshot(ctx context.Context) (model.Stat, error)
	GetTopicStats(ctx context.Context) (model.StatTopicList, error)
	GetConsumerStats(ctx context.Context) (model.StatConsumerList, error)
	GetRetryStats(ctx context.Context) (model.RepeatStat, error)
	RestartFailed(ctx context.Context, topic, group string) error
	RestartFailedSince(ctx context.Context, topic, group string, lookbackSeconds int64) error
	RestartFailedByError(ctx context.Context, topic, group, errorMessage string, lookbackSeconds int64) error
}

type IEventSource interface {
	Handler(handler func(event model.Event))
}

type AdminApi struct {
	service             IService
	eventSource         IEventSource
	eventConsumersCount func() int
}

func New(service IService, eventSource IEventSource) *AdminApi {
	return &AdminApi{
		service:             service,
		eventSource:         eventSource,
		eventConsumersCount: func() int { return 0 },
	}
}

func (a *AdminApi) EventConsumersCount() int {
	return a.eventConsumersCount()
}
