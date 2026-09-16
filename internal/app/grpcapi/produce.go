package grpcapi

import (
	"context"
	"fmt"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
)

func (b *GrpcApi) Produce(ctx context.Context, req *pb.ProduceRequest) (*pb.ProduceResponse, error) {
	timestamp, err := parseProduceTimestamp(req.Timestamp)
	if err != nil {
		return nil, err
	}
	if err := b.dataBus.Produce(ctx, model.TopicName(req.Topic), req.Key, req.Message, req.Version, req.IdempotencyKey, timestamp); err != nil {
		return nil, err
	}
	return &pb.ProduceResponse{Ok: true}, nil
}

func (b *GrpcApi) ProduceBatch(ctx context.Context, req *pb.ProduceBatchRequest) (*pb.ProduceBatchResponse, error) {
	messages := make([]model.ProduceData, 0, len(req.MessageList))
	for _, message := range req.MessageList {
		timestamp, err := parseProduceTimestamp(message.Timestamp)
		if err != nil {
			return nil, err
		}
		messages = append(messages, model.ProduceData{
			Key:            message.Key,
			Message:        message.Message,
			Version:        message.Version,
			IdempotencyKey: message.IdempotencyKey,
			Timestamp:      timestamp,
		})
	}
	if err := b.dataBus.ProduceBatch(ctx, model.TopicName(req.Topic), messages); err != nil {
		return nil, err
	}
	return &pb.ProduceBatchResponse{Ok: true}, nil
}

func parseProduceTimestamp(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	timestamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("Can't parse timestamp %s: %w", value, err)
	}
	return &timestamp, nil
}
