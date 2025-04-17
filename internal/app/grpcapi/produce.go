package grpcapi

import (
	"context"
	"fmt"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
)

func (b *GrpcApi) Produce(ctx context.Context, req *pb.ProduceRequest) (*pb.ProduceResponse, error) {
	var timestamp *time.Time
	if req.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("Can't parse timestamp %s: %w", req.Timestamp, err)
		}
		timestamp = &t
	}
	if err := b.dataBus.Produce(ctx, model.TopicName(req.Topic), req.Key, req.Message, req.Version, req.IdempotencyKey, timestamp); err != nil {
		return nil, err
	}
	return &pb.ProduceResponse{Ok: true}, nil
}
