package adminapi

import (
	"context"
	"github.com/prokraft/redbus/internal/app/model"
)

type IService interface {
	Health(ctx context.Context) error
	GetStateSnapshot(ctx context.Context) (model.Stat, error)
	GetTopicStats(ctx context.Context) (model.StatTopicList, error)
	GetRetryStats(ctx context.Context) (model.RepeatStat, error)
	RestartFailed(ctx context.Context, topic, group string) error
}

type IEventSource interface {
	Handler(handler func(event model.Event))
}

type AdminApi struct {
	service     IService
	eventSource IEventSource
}

func New(service IService, eventSource IEventSource) *AdminApi {
	return &AdminApi{
		service:     service,
		eventSource: eventSource,
	}
}
