package databus

import (
	"context"
	"log"
	"strings"

	"github.com/sergiusd/redbus/api/golang/pb"
)

func (b *DataBus) Produce(ctx context.Context, req *pb.ProduceRequest) (*pb.ProduceResponse, error) {
	log.Printf("Handle produce to topic %v: %v / %v", req.Topic, req.Key, req.Message)
	p, err := b.producerStore.Get(ctx, req.Topic)
	if err != nil {
		return nil, err
	}
	if err := p.Produce(ctx, []byte(req.Key), req.Message); err != nil {
		return nil, err
	}
	b.produceLog(req.Topic, "Produce to kafka: %v", req.Message)
	return &pb.ProduceResponse{Ok: true}, nil
}

func (b *DataBus) produceLog(topic string, message string, args ...any) {
	args = append([]any{strings.Join(b.kafkaHost, ","), topic}, args...)
	log.Printf("[%v/%v] "+message+"\n", args...)
}
