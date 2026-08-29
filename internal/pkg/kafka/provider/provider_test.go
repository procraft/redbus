package provider

import (
	"context"
	"testing"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

type kafkaClientStub struct {
	listOffsetsRequest *kafka.ListOffsetsRequest
}

func (s *kafkaClientStub) Metadata(context.Context, *kafka.MetadataRequest) (*kafka.MetadataResponse, error) {
	return &kafka.MetadataResponse{Topics: []kafka.Topic{
		{Name: "orders", Partitions: []kafka.Partition{{ID: 0}, {ID: 1}}},
		{Name: "__internal", Internal: true, Partitions: []kafka.Partition{{ID: 0}}},
	}}, nil
}

func (s *kafkaClientStub) ListOffsets(_ context.Context, request *kafka.ListOffsetsRequest) (*kafka.ListOffsetsResponse, error) {
	s.listOffsetsRequest = request
	return &kafka.ListOffsetsResponse{Topics: map[string][]kafka.PartitionOffsets{
		"orders": {
			{Partition: 0, FirstOffset: 3, LastOffset: 10},
			{Partition: 1, FirstOffset: 5, LastOffset: 20},
		},
	}}, nil
}

func (s *kafkaClientStub) DescribeGroups(context.Context, *kafka.DescribeGroupsRequest) (*kafka.DescribeGroupsResponse, error) {
	return &kafka.DescribeGroupsResponse{Groups: []kafka.DescribeGroupsResponseGroup{{
		GroupID: "billing-orders", GroupState: "Stable",
		Members: []kafka.DescribeGroupsResponseMember{{
			MemberID: "member-1", ClientID: "worker-1", ClientHost: "/127.0.0.1",
			MemberAssignments: kafka.DescribeGroupsResponseAssignments{Topics: []kafka.GroupMemberTopic{{
				Topic: "orders", Partitions: []int{0, 1},
			}}},
		}},
	}}}, nil
}

func (s *kafkaClientStub) OffsetFetch(context.Context, *kafka.OffsetFetchRequest) (*kafka.OffsetFetchResponse, error) {
	return &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{
		"orders": {
			{Partition: 0, CommittedOffset: 8},
			{Partition: 1, CommittedOffset: -1},
		},
	}}, nil
}

func TestGetTopicListIncludesEveryPartitionAndBrokerGroupStats(t *testing.T) {
	client := &kafkaClientStub{}
	provider := &Provider{client: client, addr: kafka.TCP("localhost:9092")}

	topics, err := provider.GetTopicList(context.Background(), model.TopicGroupList{{
		Topic: "orders", Group: "billing",
	}})

	require.NoError(t, err)
	require.Len(t, client.listOffsetsRequest.Topics["orders"], 4)
	require.Len(t, topics, 1)
	require.Equal(t, []model.StatPartition{
		{N: 0, FirstOffset: 3, LastOffset: 10},
		{N: 1, FirstOffset: 5, LastOffset: 20},
	}, topics[0].PartitionList)
	require.Len(t, topics[0].GroupList, 1)
	group := topics[0].GroupList[0]
	require.Equal(t, "billing-orders", group.KafkaGroupId)
	require.Equal(t, "Stable", group.State)
	require.Equal(t, []model.StatGroupPartition{
		{N: 0, Offset: 8, FirstOffset: 3, LastOffset: 10, Lag: 2, Committed: true, ConsumerId: "worker-1"},
		{N: 1, Offset: 5, FirstOffset: 5, LastOffset: 20, Lag: 15, ConsumerId: "worker-1"},
	}, group.PartitionList)
	require.Equal(t, []model.StatConsumerPartition{
		{N: 0, GroupOffset: 8, LastOffset: 10, Lag: 2, Committed: true},
		{N: 1, GroupOffset: 5, LastOffset: 20, Lag: 15},
	}, group.ConsumerList[0].PartitionList)
}
