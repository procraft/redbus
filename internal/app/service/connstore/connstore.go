package connstore

import (
	"context"

	"github.com/prokraft/redbus/api/golang/pb"

	"github.com/prokraft/redbus/internal/app/model"
)

type ConnStore struct {
	eventSource   IEventSource
	producerStore *ProducerStore
	consumerStore *ConsumerStore
}

func New(
	createProducerFn CreateProducerFn,
	eventSource IEventSource,
) *ConnStore {
	return &ConnStore{
		eventSource:   eventSource,
		producerStore: NewProducerStore(createProducerFn),
		consumerStore: NewConsumerStore(),
	}
}

type IEventSource interface {
	Publish(fn func() model.Event)
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
	s.eventSource.Publish(func() model.Event {
		return model.EventConsumers{ConsumerCount: s.GetConsumerCount(), ConsumeTopicCount: s.GetConsumeTopicCount()}
	})
}

func (s *ConnStore) RemoveConsumer(topic, group, id string) {
	s.consumerStore.remove(topic, group, id)
	s.eventSource.Publish(func() model.Event {
		return model.EventConsumers{ConsumerCount: s.GetConsumerCount(), ConsumeTopicCount: s.GetConsumeTopicCount()}
	})
}

func (s *ConnStore) GetConsumerCount() int {
	return len(s.consumerStore.store)
}

func (s *ConnStore) GetConsumeTopicCount() int {
	ret := make(map[string]struct{}, len(s.consumerStore.store))
	for c := range s.consumerStore.store {
		ret[c.Topic] = struct{}{}
	}
	return len(ret)
}
