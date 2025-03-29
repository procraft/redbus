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

func New(ctx context.Context, host string, credentials *credential.Conf) (*Provider, error) {
	conn, err := kafka.Dial("tcp", host)
	if err != nil {
		return nil, err
	}
	transport, err := credentials.GetTransport(ctx)
	if err != nil {
		return nil, err
	}
	client := &kafka.Client{
		Addr:      kafka.TCP(host),
		Transport: transport,
	}
	return &Provider{conn: conn, client: client}, err
}

func (p *Provider) GetTopicList(ctx context.Context) ([]model.StatTopic, error) {
	metadata, err := p.client.Metadata(ctx, &kafka.MetadataRequest{
		Addr: p.client.Addr,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get kafka metadata: %w", err)
	}
	offsetRequest := make(map[string][]kafka.OffsetRequest, len(metadata.Topics))
	for _, topic := range metadata.Topics {
		if topic.Internal || topic.Name[:2] == "__" {
			continue
		}
		offsetRequest[topic.Name] = []kafka.OffsetRequest{kafka.FirstOffsetOf(0), kafka.LastOffsetOf(0)}
	}
	offsetResp, err := p.client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
		Addr:   p.client.Addr,
		Topics: offsetRequest,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to get kafka list offsets: %w", err)
	}
	topicList := make([]model.StatTopic, 0, len(metadata.Topics))
	for _, topic := range metadata.Topics {
		if topic.Internal || topic.Name[:2] == "__" {
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
	return topicList, nil
}
