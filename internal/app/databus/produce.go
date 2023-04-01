package databus

import (
	"context"
	"github.com/sergiusd/redbus/api/golang/pb"
	"log"
	"strings"

	"github.com/sergiusd/redbus/internal/pkg/kafka/producer"

	"github.com/segmentio/kafka-go"
)

func (b *DataBus) Produce(ctx context.Context, req *pb.ProduceRequest) (*pb.ProduceResponse, error) {
	log.Printf("Handle produce to topic %v: %v / %v", req.Topic, req.Key, req.Message)
	if _, ok := b.producerMap[req.Topic]; !ok {
		topicProducer, err := producer.New(ctx, b.kafkaHost, req.Topic,
			producer.WithCreateTopic(b.conf.KafkaTopicNumPartitions, b.conf.KafkaTopicReplicationFactor),
			producer.WithLog(),
			producer.WithBalancer(&kafka.RoundRobin{}),
		)
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
