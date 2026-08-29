package databus

import (
	"context"
	"fmt"

	"github.com/prokraft/redbus/internal/app/model"
)

func (b *DataBus) GetTopicList(ctx context.Context) (model.StatTopicList, error) {
	runtimeGroups := b.connStore.GetStatTopicGroupPartition()
	topicGroups := make(model.TopicGroupList, 0)
	for topic, groups := range runtimeGroups {
		for _, group := range groups {
			topicGroups = append(topicGroups, model.TopicGroup{Topic: topic, Group: group.Name})
		}
	}
	topicList, err := b.kafkaProvider.GetTopicList(ctx, topicGroups)
	if err != nil {
		return nil, fmt.Errorf("Can't get kafka topic list: %w", err)
	}
	for topicIndex := range topicList {
		groups := runtimeGroups[topicList[topicIndex].Name]
		for _, runtimeGroup := range groups {
			merged := false
			for groupIndex := range topicList[topicIndex].GroupList {
				if topicList[topicIndex].GroupList[groupIndex].Name == runtimeGroup.Name {
					mergeGroupStats(&topicList[topicIndex].GroupList[groupIndex], runtimeGroup)
					merged = true
					break
				}
			}
			if !merged {
				topicList[topicIndex].GroupList = append(topicList[topicIndex].GroupList, runtimeGroup)
			}
		}
	}
	return topicList, nil
}

func mergeGroupStats(brokerGroup *model.StatGroup, runtimeGroup model.StatGroup) {
	brokerConsumers := make(map[model.ConsumerId]model.StatConsumer, len(brokerGroup.ConsumerList))
	for _, consumer := range brokerGroup.ConsumerList {
		brokerConsumers[consumer.Id] = consumer
	}
	mergedConsumers := make([]model.StatConsumer, 0, len(runtimeGroup.ConsumerList)+len(brokerGroup.ConsumerList))
	seenConsumers := make(map[model.ConsumerId]struct{}, len(runtimeGroup.ConsumerList))
	for _, runtimeConsumer := range runtimeGroup.ConsumerList {
		if brokerConsumer, ok := brokerConsumers[runtimeConsumer.Id]; ok {
			runtimeConsumer.KafkaMemberId = brokerConsumer.KafkaMemberId
			runtimeConsumer.ClientHost = brokerConsumer.ClientHost
			if len(brokerConsumer.PartitionList) > 0 {
				runtimeConsumer.PartitionList = brokerConsumer.PartitionList
			}
		}
		enrichConsumerPartitions(&runtimeConsumer, brokerGroup.PartitionList)
		mergedConsumers = append(mergedConsumers, runtimeConsumer)
		seenConsumers[runtimeConsumer.Id] = struct{}{}
	}
	for _, brokerConsumer := range brokerGroup.ConsumerList {
		if _, seen := seenConsumers[brokerConsumer.Id]; !seen {
			mergedConsumers = append(mergedConsumers, brokerConsumer)
		}
	}
	brokerGroup.ConsumerList = mergedConsumers

	for partitionIndex := range brokerGroup.PartitionList {
		partition := &brokerGroup.PartitionList[partitionIndex]
		for _, consumer := range mergedConsumers {
			if consumer.Id == partition.ConsumerId {
				partition.ConsumerState = consumer.State
				break
			}
			if partition.ConsumerId == "" {
				for _, consumerPartition := range consumer.PartitionList {
					if consumerPartition.N == partition.N {
						partition.ConsumerId = consumer.Id
						partition.ConsumerState = consumer.State
						break
					}
				}
			}
		}
	}
}

func enrichConsumerPartitions(consumer *model.StatConsumer, groupPartitions []model.StatGroupPartition) {
	for partitionIndex := range consumer.PartitionList {
		partition := &consumer.PartitionList[partitionIndex]
		for _, groupPartition := range groupPartitions {
			if partition.N == groupPartition.N {
				partition.GroupOffset = groupPartition.Offset
				partition.LastOffset = groupPartition.LastOffset
				partition.Lag = groupPartition.Lag
				partition.Committed = groupPartition.Committed
				break
			}
		}
	}
}
