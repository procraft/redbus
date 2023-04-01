package databus

import (
	"context"
	"log"
	"strings"

	"github.com/sergiusd/redbus/api/golang/pb"
)

func (b *DataBus) Produce(ctx context.Context, req *pb.ProduceRequest) (*pb.ProduceResponse, error) {
	log.Printf("Handle produce to topic %v: %v / %v", req.Topic, req.Key, req.Message)
	if _, ok := b.producerMap[req.Topic]; !ok {
		topicProducer, err := b.createProducerFn(ctx, req.Topic)
		if err != nil {
			return nil, err
		}
		b.producerMap[req.Topic] = topicProducer
	}
	if err := b.producerMap[req.Topic].Produce(ctx, req.Key, req.Message); err != nil {
		return nil, err
	}
	b.produceLog(req.Topic, "Produce to kafka: %v", req.Message)
	return &pb.ProduceResponse{Result: true}, nil
}

func (b *DataBus) produceLog(topic string, message string, args ...any) {
	args = append([]any{strings.Join(b.kafkaHost, ","), topic}, args...)
	log.Printf("[%v/%v] "+message+"\n", args...)
}
