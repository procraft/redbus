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
	GetProducer(ctx context.Context, topic model.TopicName) (model.IProducer, error)
	FindRepeatStrategy(topic model.TopicName, group model.GroupName, id model.ConsumerId) *model.RepeatStrategy
	AddConsumer(c model.IConsumer, srv pb.RedbusService_ConsumeServer, repeatStrategy *model.RepeatStrategy)
	RemoveConsumer(c model.IConsumer)
	GetConsumerCount() int
	GetConsumeTopicCount() int
	GetConsumerTopicGroupList() model.TopicGroupList
	GetStatTopicGroupPartition() map[model.TopicName][]model.StatGroup
}

type IRepeater interface {
	Add(ctx context.Context, data model.RepeatData, errorMsg string) error
	GetCount(ctx context.Context) (int, int, error)
}

type IKafkaProvider interface {
	GetTopicList(ctx context.Context) ([]model.StatTopic, error)
}
