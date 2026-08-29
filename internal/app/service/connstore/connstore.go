package connstore

import (
	"context"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"

	"github.com/prokraft/redbus/internal/app/model"
)

type ConnStore struct {
	producerStore *ProducerStore
	consumerStore *ConsumerStore
}

func New(createProducerFn CreateProducerFn) *ConnStore {
	return &ConnStore{
		producerStore: NewProducerStore(createProducerFn),
		consumerStore: NewConsumerStore(),
	}
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
}

func (s *ConnStore) RemoveConsumer(c model.IConsumer) {
	s.consumerStore.remove(c)
}

func (s *ConnStore) GetConsumerCount() int {
	return s.consumerStore.count()
}

func (s *ConnStore) GetConsumeTopicCount() int {
	return s.consumerStore.consumeTopicCount()
}

func (s *ConnStore) GetStatTopicGroupPartition() map[model.TopicName][]model.StatGroup {
	type ConsumerStatMap = map[model.ConsumerId]consumerStatSnapshot
	type GroupStatMap = map[model.GroupName]ConsumerStatMap

	consumerStats := s.consumerStore.getStatSnapshot()
	topicGroupMap := make(map[model.TopicName]GroupStatMap, len(consumerStats))
	for key, stat := range consumerStats {
		if _, ok := topicGroupMap[key.Topic]; !ok {
			topicGroupMap[key.Topic] = make(GroupStatMap)
		}
		if _, ok := topicGroupMap[key.Topic][key.Group]; !ok {
			topicGroupMap[key.Topic][key.Group] = make(ConsumerStatMap)
		}
		topicGroupMap[key.Topic][key.Group][key.Id] = stat
	}
	ret := make(map[model.TopicName][]model.StatGroup, len(consumerStats))
	for topic, groupList := range topicGroupMap {
		topicGroupList := make([]model.StatGroup, 0, len(groupList))
		for group, consumers := range groupList {
			consumerList := make([]model.StatConsumer, 0, len(consumers))
			groupPartitionList := make([]model.StatGroupPartition, 0)
			for consumerId, stat := range consumers {
				consumerPartitions := make([]model.StatConsumerPartition, 0, len(stat.offsetMap))
				for partitionN, partitionOffset := range stat.offsetMap {
					consumerPartitions = append(consumerPartitions, model.StatConsumerPartition{
						N:           partitionN,
						GroupOffset: partitionOffset,
						Committed:   true,
					})
				}
				var lastMessageAt *time.Time
				if !stat.metrics.LastMessageAt.IsZero() {
					value := stat.metrics.LastMessageAt
					lastMessageAt = &value
				}
				consumerList = append(consumerList, model.StatConsumer{
					Id:                consumerId,
					Topic:             topic,
					Group:             group,
					State:             stat.state.String(),
					ConnectedAt:       stat.metrics.ConnectedAt,
					StateSince:        stat.metrics.StateSince,
					LastMessageAt:     lastMessageAt,
					MessagesProcessed: stat.metrics.MessagesProcessed,
					ReconnectCount:    stat.metrics.ReconnectCount,
					LastError:         stat.metrics.LastError,
					RepeatStrategy:    stat.repeatStrategy.String(),
					PartitionList:     consumerPartitions,
				})
				for partitionN, partitionOffset := range stat.offsetMap {
					groupPartitionList = append(groupPartitionList, model.StatGroupPartition{
						N:             partitionN,
						Offset:        partitionOffset,
						Committed:     true,
						ConsumerId:    consumerId,
						ConsumerState: stat.state.String(),
					})
				}
			}
			topicGroupList = append(topicGroupList, model.StatGroup{
				Name:          group,
				PartitionList: groupPartitionList,
				ConsumerList:  consumerList,
			})
		}
		ret[topic] = topicGroupList
	}
	return ret
}
