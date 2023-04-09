package producer

import (
	"context"
	"fmt"
	"github.com/sergiusd/redbus/api/golang/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Producer struct {
	client pb.RedbusServiceClient
}

func New(host string, port int) (*Producer, error) {
	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", host, port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("Can not connect with databus %w", err)
	}
	return &Producer{
		client: pb.NewRedbusServiceClient(conn),
	}, nil
}

func (p *Producer) Produce(ctx context.Context, topic, key string, message []byte) error {
	_, err := p.client.Produce(ctx, &pb.ProduceRequest{Topic: topic, Key: key, Message: message})
	if err != nil {
		return err
	}
	return nil
}
