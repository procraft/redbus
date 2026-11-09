package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/segmentio/kafka-go"
)

const headFetchMaxBytes = 1 << 20
const headFetchMaxRecords = 1024

type headFetcher interface {
	Fetch(context.Context, *kafka.FetchRequest) (*kafka.FetchResponse, error)
}

// GetQueueSnapshot reads broker positions and at most one bounded record set per
// nonempty partition. It never joins a group, commits, or decodes business data.
func (p *Provider) GetQueueSnapshot(ctx context.Context, groups model.TopicGroupList) ([]model.QueueSnapshot, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	topics, err := p.GetTopicList(ctx, groups)
	if err != nil {
		return nil, err
	}
	var result []model.QueueSnapshot
	var failures []error
	// Cache identical heads shared by groups during this one collection pass.
	type headKey struct {
		topic     model.TopicName
		partition model.PartitionN
		offset    model.Offset
	}
	type headResult struct {
		timestamp time.Time
		err       error
	}
	heads := make(map[headKey]headResult)
	for _, topic := range topics {
		for _, group := range topic.GroupList {
			observation := model.QueueSnapshot{Topic: topic.Name, Group: group.Name, Valid: topic.Error == "" && len(group.PartitionList) > 0}
			for _, partition := range group.PartitionList {
				if partition.LagError != "" || partition.Lag < 0 {
					observation.Valid = false
					continue
				}
				observation.Lag += int64(partition.Lag)
				if partition.Lag == 0 {
					continue
				}
				key := headKey{topic.Name, partition.N, partition.Offset}
				head, found := heads[key]
				if !found {
					head.timestamp, head.err = p.headTimestamp(ctx, key.topic, key.partition, key.offset)
					heads[key] = head
				}
				if head.err != nil {
					observation.Valid = false
					failures = append(failures, fmt.Errorf("queue head %s/%s partition %d: %w", topic.Name, group.Name, partition.N, head.err))
					continue
				}
				if observation.Oldest.IsZero() || head.timestamp.Before(observation.Oldest) {
					observation.Oldest = head.timestamp
				}
			}
			result = append(result, observation)
		}
	}
	return result, errors.Join(failures...)
}

func (p *Provider) headTimestamp(ctx context.Context, topic model.TopicName, partition model.PartitionN, offset model.Offset) (time.Time, error) {
	fetcher, ok := p.client.(headFetcher)
	if !ok {
		return time.Time{}, errors.New("Kafka client does not support head fetch")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	response, err := fetcher.Fetch(ctx, &kafka.FetchRequest{
		Addr: p.addr, Topic: string(topic), Partition: int(partition), Offset: int64(offset),
		MinBytes: 1, MaxBytes: headFetchMaxBytes, MaxWait: 500 * time.Millisecond,
	})
	if err != nil {
		return time.Time{}, err
	}
	if response == nil {
		return time.Time{}, errors.New("missing Fetch response")
	}
	if response.Error != nil {
		return time.Time{}, response.Error
	}
	if response.Records == nil {
		return time.Time{}, errors.New("missing head record")
	}
	// Kafka may return the whole compressed batch starting before the requested
	// offset. Bound metadata inspection; do not scan the backlog looking for a timestamp.
	for range headFetchMaxRecords {
		record, err := response.Records.ReadRecord()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return time.Time{}, errors.New("no record at uncommitted head")
			}
			return time.Time{}, err
		}
		if record == nil {
			return time.Time{}, errors.New("nil head record")
		}
		if record.Key != nil {
			_ = record.Key.Close()
		}
		if record.Value != nil {
			_ = record.Value.Close()
		}
		if record.Offset < int64(offset) {
			continue
		}
		if record.Time.IsZero() || record.Time.Unix() <= 0 || record.Time.After(time.Now().Add(time.Minute)) {
			return time.Time{}, errors.New("missing or future Kafka record timestamp")
		}
		return record.Time, nil
	}
	return time.Time{}, errors.New("head record exceeds bounded record-set inspection")
}
