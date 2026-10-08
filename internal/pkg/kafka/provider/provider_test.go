package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/internal/app/model"
)

type kafkaClientStub struct {
	listOffsetsRequest *kafka.ListOffsetsRequest
	metadataOverride   *kafka.MetadataResponse
	listOverride       *kafka.ListOffsetsResponse
	listError          error
	fetchOverride      *kafka.OffsetFetchResponse
	fetchError         error
}

func (s *kafkaClientStub) Metadata(context.Context, *kafka.MetadataRequest) (*kafka.MetadataResponse, error) {
	if s.metadataOverride != nil {
		return s.metadataOverride, nil
	}
	return &kafka.MetadataResponse{Topics: []kafka.Topic{
		{Name: "orders", Partitions: []kafka.Partition{{ID: 0}, {ID: 1}}},
		{Name: "__internal", Internal: true, Partitions: []kafka.Partition{{ID: 0}}},
	}}, nil
}

func (s *kafkaClientStub) ListOffsets(_ context.Context, request *kafka.ListOffsetsRequest) (*kafka.ListOffsetsResponse, error) {
	s.listOffsetsRequest = request
	if s.listOverride != nil || s.listError != nil {
		return s.listOverride, s.listError
	}
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
	if s.fetchOverride != nil || s.fetchError != nil {
		return s.fetchOverride, s.fetchError
	}
	return &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{
		"orders": {
			{Partition: 0, CommittedOffset: 8},
			{Partition: 1, CommittedOffset: -1},
		},
	}}, nil
}

func TestGetTopicListReportsUnavailableLag(t *testing.T) {
	for _, test := range []struct {
		name   string
		client kafkaClientStub
		reason string
	}{
		{"partition metadata error", kafkaClientStub{metadataOverride: &kafka.MetadataResponse{Topics: []kafka.Topic{{Name: "orders", Partitions: []kafka.Partition{{ID: 0, Error: kafka.LeaderNotAvailable}, {ID: 1}}}}}}, "metadata unavailable"},
		{"topic metadata error", kafkaClientStub{metadataOverride: &kafka.MetadataResponse{Topics: []kafka.Topic{{Name: "orders", Error: kafka.LeaderNotAvailable, Partitions: []kafka.Partition{{ID: 0}, {ID: 1}}}}}}, "metadata unavailable"},
		{"list partition error", kafkaClientStub{listOverride: &kafka.ListOffsetsResponse{Topics: map[string][]kafka.PartitionOffsets{"orders": {{Partition: 0, Error: kafka.LeaderNotAvailable}, {Partition: 1, FirstOffset: 5, LastOffset: 20}}}}}, "offsets unavailable"},
		{"list request error", kafkaClientStub{listError: errors.New("broker unreachable")}, "broker unreachable"},
		{"missing list partition", kafkaClientStub{listOverride: &kafka.ListOffsetsResponse{Topics: map[string][]kafka.PartitionOffsets{"orders": {{Partition: 1, FirstOffset: 5, LastOffset: 20}}}}}, "missing ListOffsets"},
		{"invalid list offsets", kafkaClientStub{listOverride: &kafka.ListOffsetsResponse{Topics: map[string][]kafka.PartitionOffsets{"orders": {{Partition: 0, FirstOffset: -1, LastOffset: 10}}}}}, "invalid offsets"},
		{"fetch partition error", kafkaClientStub{fetchOverride: &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{"orders": {{Partition: 0, Error: kafka.GroupAuthorizationFailed}}}}}, "committed offset unavailable"},
		{"fetch response error", kafkaClientStub{fetchOverride: &kafka.OffsetFetchResponse{Error: kafka.GroupAuthorizationFailed}}, "committed offset unavailable"},
		{"fetch request error", kafkaClientStub{fetchError: errors.New("coordinator unreachable")}, "coordinator unreachable"},
		{"missing fetch partition", kafkaClientStub{fetchOverride: &kafka.OffsetFetchResponse{}}, "missing partition"},
		{"invalid committed offset", kafkaClientStub{fetchOverride: &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{"orders": {{Partition: 0, CommittedOffset: -2}}}}}, "invalid committed offset"},
		{"negative lag", kafkaClientStub{fetchOverride: &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{"orders": {{Partition: 0, CommittedOffset: 11}}}}}, "exceeds latest offset"},
		{"expired committed offset", kafkaClientStub{fetchOverride: &kafka.OffsetFetchResponse{Topics: map[string][]kafka.OffsetFetchPartition{"orders": {{Partition: 0, CommittedOffset: 2}}}}}, "precedes first retained offset"},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &Provider{client: &test.client, addr: kafka.TCP("localhost:9092")}
			topics, err := provider.GetTopicList(context.Background(), model.TopicGroupList{{Topic: "orders", Group: "billing"}})
			require.NoError(t, err)
			partition := topics[0].GroupList[0].PartitionList[0]
			require.Equal(t, model.Offset(-1), partition.Lag)
			require.Contains(t, partition.LagError, test.reason)
			require.Equal(t, partition.LagError, topics[0].GroupList[0].ConsumerList[0].PartitionList[0].LagError)
		})
	}
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
