package databus

import (
	"context"
	"github.com/prokraft/redbus/internal/config"

	"github.com/prokraft/redbus/api/golang/pb"

	"github.com/prokraft/redbus/internal/app/model"
)

type DataBus struct {
	conf      *config.Config
	connStore IConnStore
	repeater  IRepeater
}

func New(
	conf *config.Config,
	connStore IConnStore,
	repeater IRepeater,
) *DataBus {
	return &DataBus{
		conf:      conf,
		connStore: connStore,
		repeater:  repeater,
	}
}

type IConnStore interface {
	GetProducer(ctx context.Context, topic string) (model.IProducer, error)
	FindRepeatStrategy(topic, group, id string) *model.RepeatStrategy
	AddConsumer(srv pb.RedbusService_ConsumeServer, topic, group, id string, repeatStrategy *model.RepeatStrategy, c model.IConsumer)
	RemoveConsumer(topic, group, id string)
	GetConsumerCount() int
	GetConsumeTopicCount() int
}

type IRepeater interface {
	Add(ctx context.Context, data model.RepeatData, errorMsg string) error
	GetCount(ctx context.Context) (int, int, error)
}
