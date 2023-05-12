package adminapi

import (
	"context"
	"github.com/sergiusd/redbus/internal/app/model"
)

type IDataBusService interface {
	GetStat(ctx context.Context) (model.DataBusStat, error)
}

type IRepeater interface {
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
