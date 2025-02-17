package databus

import (
	"context"
	"fmt"

	"github.com/prokraft/redbus/internal/app/model"
)

func (b *DataBus) GetTopicList(ctx context.Context) (model.StatTopicList, error) {
	topicList, err := b.kafkaProvider.GetTopicList(ctx)
	if err != nil {
		return nil, fmt.Errorf("Can't get kafka topic list: %w", err)
	}
	topicGroupPartitionMap := b.connStore.GetStatTopicGroupPartition()
	for i, topic := range topicList {
		topicList[i].GroupList = topicGroupPartitionMap[topic.Name]
	}
	return topicList, nil
}
