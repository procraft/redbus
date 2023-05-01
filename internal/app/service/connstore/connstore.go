package connstore

import (
	"context"

	"github.com/sergiusd/redbus/api/golang/pb"

	"github.com/sergiusd/redbus/internal/app/model"
)

type ConnStore struct {
	producerStore *ProducerStore
	consumerStore *ConsumerStore
}

func New(
	createProducerFn CreateProducerFn,
) *ConnStore {
	return &ConnStore{
		producerStore: NewProducerStore(createProducerFn),
		consumerStore: NewConsumerStore(),
	}
}

func (s *ConnStore) GetProducer(ctx context.Context, topic string) (model.IProducer, error) {
	return s.producerStore.get(ctx, topic)
}

func (s *ConnStore) FindRepeatStrategy(topic, group, id string) *model.RepeatStrategy {
	c := s.consumerStore.findBest(topic, group, id)
	if c == nil {
		return nil
	}
	return c.RepeatStrategy
}

func (s *ConnStore) FindBestConsumerBag(topic, group, id string) *ConsumerBag {
	return s.consumerStore.findBest(topic, group, id)
}

func (s *ConnStore) GetConsumerTopicGroupList() model.TopicGroupList {
	return s.consumerStore.getTopicGroupList()
}

func (s *ConnStore) AddConsumer(srv pb.RedbusService_ConsumeServer, topic, group, id string, repeatStrategy *model.RepeatStrategy, c model.IConsumer) {
	s.consumerStore.add(topic, group, id, repeatStrategy, c, srv)
}

func (s *ConnStore) RemoveConsumer(topic, group, id string) {
	s.consumerStore.remove(topic, group, id)
}
