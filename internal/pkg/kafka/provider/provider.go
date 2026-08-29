package provider

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"github.com/segmentio/kafka-go"
)

type Provider struct {
	conn   *kafka.Conn
	addr   net.Addr
	client kafkaClient
}

type kafkaClient interface {
	Metadata(context.Context, *kafka.MetadataRequest) (*kafka.MetadataResponse, error)
	ListOffsets(context.Context, *kafka.ListOffsetsRequest) (*kafka.ListOffsetsResponse, error)
	DescribeGroups(context.Context, *kafka.DescribeGroupsRequest) (*kafka.DescribeGroupsResponse, error)
	OffsetFetch(context.Context, *kafka.OffsetFetchRequest) (*kafka.OffsetFetchResponse, error)
}

func New(ctx context.Context, host string, credentials *credential.Conf) (*Provider, error) {
	conn, err := kafka.Dial("tcp", host)
	if err != nil {
		return nil, fmt.Errorf("Can't connect kafka.Dial: %w", err)
	}
	client := &kafka.Client{
		Addr: kafka.TCP(host),
	}
	transport, err := credentials.GetTransport(ctx)
	if err != nil {
		return nil, fmt.Errorf("Can't get transport by kafka credentials: %w", err)
	}
	if transport != nil {
		client.Transport = transport
	}
	return &Provider{conn: conn, addr: client.Addr, client: client}, err
}

func (p *Provider) GetTopicList(ctx context.Context, topicGroups model.TopicGroupList) ([]model.StatTopic, error) {
	metadata, err := p.client.Metadata(ctx, &kafka.MetadataRequest{
		Addr: p.addr,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get kafka metadata: %w", err)
	}
	offsetRequest := make(map[string][]kafka.OffsetRequest, len(metadata.Topics))
	for _, topic := range metadata.Topics {
		if topic.Internal || strings.HasPrefix(topic.Name, "__") {
			continue
		}
		requests := make([]kafka.OffsetRequest, 0, len(topic.Partitions)*2)
		for _, partition := range topic.Partitions {
			requests = append(requests, kafka.FirstOffsetOf(partition.ID), kafka.LastOffsetOf(partition.ID))
		}
		offsetRequest[topic.Name] = requests
	}
	offsetResp, err := p.client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
		Addr:   p.addr,
		Topics: offsetRequest,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get kafka list offsets: %w", err)
	}
	topicList := make([]model.StatTopic, 0, len(metadata.Topics))
	for _, topic := range metadata.Topics {
		if topic.Internal || strings.HasPrefix(topic.Name, "__") {
			continue
		}
		partitionList := make([]model.StatPartition, 0)
		if offsetsList, ok := offsetResp.Topics[topic.Name]; ok {
			for _, offset := range offsetsList {
				partitionList = append(partitionList, model.StatPartition{
					N:           model.PartitionN(offset.Partition),
					FirstOffset: model.Offset(offset.FirstOffset),
					LastOffset:  model.Offset(offset.LastOffset),
				})
			}
		}
		topicList = append(topicList, model.StatTopic{
			Name:          model.TopicName(topic.Name),
			PartitionList: partitionList,
		})
	}
	sort.Slice(topicList, func(i, j int) bool { return topicList[i].Name < topicList[j].Name })
	for i := range topicList {
		sort.Slice(topicList[i].PartitionList, func(left, right int) bool {
			return topicList[i].PartitionList[left].N < topicList[i].PartitionList[right].N
		})
	}
	if err := p.addGroupStats(ctx, topicList, topicGroups); err != nil {
		return nil, err
	}
	return topicList, nil
}

func (p *Provider) addGroupStats(
	ctx context.Context,
	topics model.StatTopicList,
	topicGroups model.TopicGroupList,
) error {
	if len(topicGroups) == 0 {
		return nil
	}

	groupIDs := make([]string, 0, len(topicGroups))
	for _, topicGroup := range topicGroups {
		groupIDs = append(groupIDs, topicGroup.KafkaGroupId())
	}
	describeResponse, err := p.client.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{GroupIDs: groupIDs})
	descriptions := make(map[string]kafka.DescribeGroupsResponseGroup)
	describeError := ""
	if err != nil {
		describeError = fmt.Sprintf("failed to describe kafka consumer groups: %v", err)
	} else {
		for _, group := range describeResponse.Groups {
			descriptions[group.GroupID] = group
		}
	}

	topicIndex := make(map[model.TopicName]*model.StatTopic, len(topics))
	for i := range topics {
		topicIndex[topics[i].Name] = &topics[i]
	}
	for _, topicGroup := range topicGroups {
		topic := topicIndex[topicGroup.Topic]
		if topic == nil {
			continue
		}
		partitions := make([]int, 0, len(topic.PartitionList))
		for _, partition := range topic.PartitionList {
			partitions = append(partitions, int(partition.N))
		}
		offsetResponse, offsetErr := p.client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{
			GroupID: topicGroup.KafkaGroupId(),
			Topics:  map[string][]int{string(topicGroup.Topic): partitions},
		})

		description := descriptions[topicGroup.KafkaGroupId()]
		group := model.StatGroup{
			Name:         topicGroup.Group,
			KafkaGroupId: topicGroup.KafkaGroupId(),
			State:        description.GroupState,
			Error:        describeError,
		}
		if description.Error != nil {
			group.Error = appendGroupError(group.Error, description.Error.Error())
		}
		if offsetErr != nil {
			group.Error = appendGroupError(group.Error, fmt.Sprintf("failed to fetch offsets: %v", offsetErr))
		} else if offsetResponse != nil && offsetResponse.Error != nil {
			group.Error = appendGroupError(group.Error, fmt.Sprintf("failed to fetch offsets: %v", offsetResponse.Error))
		}
		assignedConsumers := make(map[model.PartitionN]model.ConsumerId)
		for _, member := range description.Members {
			consumerID := model.ConsumerId(member.ClientID)
			if consumerID == "" {
				consumerID = model.ConsumerId(member.MemberID)
			}
			consumer := model.StatConsumer{
				Id:            consumerID,
				Topic:         topicGroup.Topic,
				Group:         topicGroup.Group,
				State:         "broker-member",
				KafkaMemberId: member.MemberID,
				ClientHost:    member.ClientHost,
			}
			for _, assignment := range member.MemberAssignments.Topics {
				if assignment.Topic != string(topicGroup.Topic) {
					continue
				}
				for _, partition := range assignment.Partitions {
					partitionN := model.PartitionN(partition)
					assignedConsumers[partitionN] = consumerID
					consumer.PartitionList = append(consumer.PartitionList, model.StatConsumerPartition{N: partitionN})
				}
			}
			group.ConsumerList = append(group.ConsumerList, consumer)
		}

		committedOffsets := make(map[model.PartitionN]kafka.OffsetFetchPartition)
		if offsetResponse != nil {
			for _, partition := range offsetResponse.Topics[string(topicGroup.Topic)] {
				committedOffsets[model.PartitionN(partition.Partition)] = partition
			}
		}
		for _, partition := range topic.PartitionList {
			groupPartition := model.StatGroupPartition{
				N:           partition.N,
				FirstOffset: partition.FirstOffset,
				LastOffset:  partition.LastOffset,
				ConsumerId:  assignedConsumers[partition.N],
			}
			committedOffset, ok := committedOffsets[partition.N]
			if ok && committedOffset.Error == nil && committedOffset.CommittedOffset >= 0 {
				groupPartition.Offset = model.Offset(committedOffset.CommittedOffset)
				groupPartition.Committed = true
			} else {
				groupPartition.Offset = partition.FirstOffset
			}
			groupPartition.Lag = groupPartition.LastOffset - groupPartition.Offset
			group.PartitionList = append(group.PartitionList, groupPartition)
		}
		for consumerIndex := range group.ConsumerList {
			for partitionIndex := range group.ConsumerList[consumerIndex].PartitionList {
				consumerPartition := &group.ConsumerList[consumerIndex].PartitionList[partitionIndex]
				for _, groupPartition := range group.PartitionList {
					if groupPartition.N == consumerPartition.N {
						consumerPartition.GroupOffset = groupPartition.Offset
						consumerPartition.LastOffset = groupPartition.LastOffset
						consumerPartition.Lag = groupPartition.Lag
						consumerPartition.Committed = groupPartition.Committed
						break
					}
				}
			}
		}
		topic.GroupList = append(topic.GroupList, group)
	}
	for i := range topics {
		sort.Slice(topics[i].GroupList, func(left, right int) bool {
			return topics[i].GroupList[left].Name < topics[i].GroupList[right].Name
		})
	}
	return nil
}

func appendGroupError(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}
