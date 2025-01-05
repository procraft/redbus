package provider

import (
	"context"
	"fmt"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"github.com/segmentio/kafka-go"
)

type Provider struct {
	conn   *kafka.Conn
	client *kafka.Client
}

func New(host string, credentials *credential.Conf) (*Provider, error) {
	conn, err := kafka.Dial("tcp", host)
	client := &kafka.Client{
		Addr: kafka.TCP(host),
	}
	return &Provider{conn: conn, client: client}, err
}

func (p *Provider) GetTopicList(ctx context.Context) ([]model.Topic, error) {
	metadata, err := p.client.Metadata(ctx, &kafka.MetadataRequest{
		Addr: p.client.Addr,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get kafka metadata: %w", err)
	}
	offsetResp, err := p.client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
		Addr: p.client.Addr,
		Topics: map[string][]kafka.OffsetRequest{
			"topic-1": {kafka.FirstOffsetOf(0), kafka.LastOffsetOf(0)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get kafka list offsets: %w", err)
	}
	//groupsResp, err := p.client.ListGroups(ctx, &kafka.ListGroupsRequest{
	//	Addr: p.client.Addr,
	//})
	//if err != nil {
	//	return nil, fmt.Errorf("Failed to get kafka list groups: %w", err)
	//}
	//fmt.Printf("%v\n", groupsResp)
	topicList := make([]model.Topic, 0, len(metadata.Topics))
	for _, topic := range metadata.Topics {
		if topic.Internal || topic.Name[:2] == "__" {
			continue
		}
		partitionList := make([]model.Partition, 0)
		if offsetsList, ok := offsetResp.Topics[topic.Name]; ok {
			for _, offset := range offsetsList {
				partitionList = append(partitionList, model.Partition{
					N:           offset.Partition,
					FirstOffset: offset.FirstOffset,
					LastOffset:  offset.LastOffset,
				})
			}
		}
		//partitionList := make([]model.Partition, 0, len(topic.Partitions))
		//for _, partition := range topic.Partitions {
		//	partitionList = append(partitionList, model.Partition{N: partition.ID})
		//}
		topicList = append(topicList, model.Topic{
			Name:          topic.Name,
			PartitionList: partitionList,
		})
	}
	return topicList, nil
}
