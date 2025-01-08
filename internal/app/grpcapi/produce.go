package grpcapi

import (
	"context"
	"github.com/prokraft/redbus/internal/app/model"

	"github.com/prokraft/redbus/api/golang/pb"
)

func (b *GrpcApi) Produce(ctx context.Context, req *pb.ProduceRequest) (*pb.ProduceResponse, error) {
	if err := b.dataBus.Produce(ctx, model.TopicName(req.Topic), req.Key, req.Message); err != nil {
		return nil, err
	}
	return &pb.ProduceResponse{Ok: true}, nil
}
