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

func (s *ConnStore) GetProducer(ctx context.Context, topic model.TopicName) (model.IProducer, error) {
	return s.producerStore.get(ctx, topic)
}

func (s *ConnStore) FindRepeatStrategy(topic model.TopicName, group model.GroupName, id model.ConsumerId) *model.RepeatStrategy {
	c := s.consumerStore.findBest(topic, group, id)
	if c == nil {
		return nil
	}
	return c.RepeatStrategy
}

func (s *ConnStore) FindBestConsumerBag(topic model.TopicName, group model.GroupName, id model.ConsumerId) *ConsumerBag {
	return s.consumerStore.findBest(topic, group, id)
}

func (s *ConnStore) GetConsumerTopicGroupList() model.TopicGroupList {
	return s.consumerStore.getTopicGroupList()
}

func (s *ConnStore) AddConsumer(c model.IConsumer, srv pb.RedbusService_ConsumeServer, repeatStrategy *model.RepeatStrategy) {
	s.consumerStore.add(c, repeatStrategy, srv)
	s.eventSource.Publish(func() model.Event {
		return model.EventConsumers{ConsumerCount: s.GetConsumerCount(), ConsumeTopicCount: s.GetConsumeTopicCount()}
	})
}

func (s *ConnStore) RemoveConsumer(c model.IConsumer) {
	s.consumerStore.remove(c)
	s.eventSource.Publish(func() model.Event {
		return model.EventConsumers{ConsumerCount: s.GetConsumerCount(), ConsumeTopicCount: s.GetConsumeTopicCount()}
	})
}

func (s *ConnStore) GetConsumerCount() int {
	return len(s.consumerStore.store)
}

func (s *ConnStore) GetConsumeTopicCount() int {
	ret := make(map[model.TopicName]struct{}, len(s.consumerStore.store))
	for c := range s.consumerStore.store {
		ret[c.Topic] = struct{}{}
	}
	return len(ret)
}

func (s *ConnStore) GetStatTopicGroupPartition() map[model.TopicName][]model.StatGroup {
	type ConsumerOffsetMap = map[model.ConsumerId]model.PartitionOffsetMap
	type PartitionOffsetMap = map[model.GroupName]ConsumerOffsetMap

	offsetMap := s.consumerStore.getOffsetMap()
	topicGroupMap := make(map[model.TopicName]PartitionOffsetMap, len(offsetMap))
	for key, partitionList := range offsetMap {
		if _, ok := topicGroupMap[key.Topic]; !ok {
			topicGroupMap[key.Topic] = make(PartitionOffsetMap, len(partitionList))
		}
		if _, ok := topicGroupMap[key.Topic][key.Group]; !ok {
			topicGroupMap[key.Topic][key.Group] = make(ConsumerOffsetMap, len(partitionList))
		}
		topicGroupMap[key.Topic][key.Group][key.Id] = partitionList
	}
	ret := make(map[model.TopicName][]model.StatGroup, len(offsetMap))
	for topic, groupList := range topicGroupMap {
		topicGroupList := make([]model.StatGroup, 0, len(groupList))
		for group, partitionList := range groupList {
			groupPartitionList := make([]model.StatGroupPartition, 0, len(partitionList))
			for consumerId, partitionOffsetMap := range partitionList {
				for partitionN, partitionOffset := range partitionOffsetMap {
					groupPartitionList = append(groupPartitionList, model.StatGroupPartition{
						N:             partitionN,
						Offset:        partitionOffset,
						ConsumerId:    consumerId,
						ConsumerState: s.consumerStore.getState(consumerId).String(),
					})
				}
			}
			topicGroupList = append(topicGroupList, model.StatGroup{
				Name:          group,
				PartitionList: groupPartitionList,
			})
		}
		ret[topic] = topicGroupList
	}
	return ret
}
