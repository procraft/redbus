package databus

import (
	"context"
	"github.com/prokraft/redbus/internal/config"

	"github.com/prokraft/redbus/api/golang/pb"

	"github.com/prokraft/redbus/internal/app/model"
)

type DataBus struct {
	conf          *config.Config
	connStore     IConnStore
	repeater      IRepeater
	kafkaProvider IKafkaProvider
}

func New(
	conf *config.Config,
	connStore IConnStore,
	repeater IRepeater,
	kafkaProvider IKafkaProvider,
) *DataBus {
	return &DataBus{
		conf:          conf,
		connStore:     connStore,
		repeater:      repeater,
		kafkaProvider: kafkaProvider,
	}
}

type IConnStore interface {
	GetProducer(ctx context.Context, topic string) (model.IProducer, error)
	FindRepeatStrategy(topic, group, id string) *model.RepeatStrategy
	AddConsumer(c model.IConsumer, srv pb.RedbusService_ConsumeServer, repeatStrategy *model.RepeatStrategy)
	RemoveConsumer(c model.IConsumer)
	GetConsumerCount() int
	GetConsumeTopicCount() int
}

type IRepeater interface {
	Add(ctx context.Context, data model.RepeatData, errorMsg string) error
	GetCount(ctx context.Context) (int, int, error)
}

type IKafkaProvider interface {
	GetTopicList(ctx context.Context) ([]model.Topic, error)
}
