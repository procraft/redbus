package producer

import (
	"context"
	"fmt"
	"github.com/prokraft/redbus/internal/app/model"
	"log"
	"time"

	"github.com/prokraft/redbus/internal/pkg/kafka/credential"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
	topic  model.TopicName
	conf   conf
}

type CreateOptions struct {
	NumPartitions     int
	ReplicationFactor int
}

type conf struct {
	log         bool
	createTopic *CreateOptions
	credentials *credential.Conf
	balancer    kafka.Balancer
}

func New(ctx context.Context, hosts []string, credentials *credential.Conf, topic model.TopicName, options ...Option) (*Producer, error) {
	p := Producer{
		topic: topic,
		conf: conf{
			log:         false,
			createTopic: nil,
			credentials: credentials,
			balancer:    &kafka.CRC32Balancer{},
		},
	}
	for _, o := range options {
		o(&p.conf)
	}

	if p.conf.createTopic != nil {
		if err := p.createTopic(ctx, hosts, *p.conf.createTopic); err != nil {
			return nil, fmt.Errorf("Can't create topic: %v, err: %w\n", p.topic, err)
		}
	}

	p.writer = &kafka.Writer{
		Addr:         kafka.TCP(hosts...),
		Topic:        string(topic),
		RequiredAcks: kafka.RequireOne,
		Balancer:     p.conf.balancer,
	}

	auth := "noauth"
	if p.conf.credentials != nil {
		auth = p.conf.credentials.User
	}

	transport, err := p.conf.credentials.GetTransport(ctx)
	if err != nil {
		return nil, err
	}
	if transport != nil {
		p.writer.Transport = transport
	}

	log.Printf("Ready to produce kafka %v@%v '%v', %T\n", auth, hosts, p.topic, p.conf.balancer)

	return &p, nil
}

func (p *Producer) Produce(ctx context.Context, key string, message []byte, headers map[string]string) error {
	kafkaHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		kafkaHeaders = append(kafkaHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}
	kafkaMessage := kafka.Message{Key: []byte(key), Value: message, Headers: kafkaHeaders}
	if err := p.writer.WriteMessages(ctx, kafkaMessage); err != nil {
		return fmt.Errorf("Failed to produce messages, messages: %v, topic: %v, err: %w\n", kafkaMessage, p.topic, err)
	}
	if p.conf.log {
		log.Printf("Produce kafka message at topic %v: %#v\n", p.topic, kafkaMessage)
	}
	return nil
}

func (p *Producer) createTopic(ctx context.Context, hosts []string, options CreateOptions) error {
	if len(hosts) == 0 {
		return fmt.Errorf("Can't create kafka topic, kafka hosts list is empty\n")
	}

	if options.NumPartitions == 0 {
		options.NumPartitions = 1
	}
	if options.ReplicationFactor == 0 {
		options.ReplicationFactor = 1
	}

	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	if err := p.conf.credentials.UpdateDialer(ctx, dialer); err != nil {
		return err
	}

	conn, err := dialer.DialContext(ctx, "tcp", hosts[0])
	if err != nil {
		return fmt.Errorf("Can't connect to kafka for create topic: %w\n", err)
	}
	defer conn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             string(p.topic),
			NumPartitions:     options.NumPartitions,
			ReplicationFactor: options.ReplicationFactor,
		},
	}

	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		return fmt.Errorf("Can't create kafka topic: %w\n", err)
	}

	log.Printf("Create kafka topic: %v, partitions: %v, replicas: %v\n", p.topic, options.NumPartitions, options.ReplicationFactor)

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
