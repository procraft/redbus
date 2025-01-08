package adminapi

import (
	"context"
	"github.com/prokraft/redbus/internal/app/model"
)

type IDataBusService interface {
	GetStat(ctx context.Context) (model.Stat, error)
	GetTopicList(ctx context.Context) (model.StatTopicList, error)
}

type IRepeater interface {
	GetStat(ctx context.Context) (model.RepeatStat, error)
	RestartFailed(ctx context.Context, topic, group string) error
}

type IEventSource interface {
	Handler(handler func(event model.Event))
}

type AdminApi struct {
	dataBus     IDataBusService
	repeater    IRepeater
	eventSource IEventSource
}

func New(dataBus IDataBusService, repeater IRepeater, eventSource IEventSource) *AdminApi {
	return &AdminApi{
		dataBus:     dataBus,
		repeater:    repeater,
		eventSource: eventSource,
	}
}
