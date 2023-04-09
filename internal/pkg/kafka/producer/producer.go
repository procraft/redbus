package producer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sergiusd/redbus/internal/pkg/kafka/credential"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
	topic  string
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

func New(ctx context.Context, hosts []string, topic string, options ...Option) (*Producer, error) {
	p := Producer{
		topic: topic,
		conf: conf{
			log:         false,
			createTopic: nil,
			credentials: nil,
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
		Topic:        topic,
		RequiredAcks: kafka.RequireOne,
		Balancer:     p.conf.balancer,
	}

	auth := "noauth"
	if p.conf.credentials != nil {
		saslConfig, tlsConfig, err := p.conf.credentials.GetSaslAndTls(ctx)
		if err != nil {
			return nil, err
		}
		if saslConfig != nil {
			p.writer.Transport = &kafka.Transport{
				SASL: *saslConfig,
				TLS:  tlsConfig,
			}
		}
		auth = p.conf.credentials.User
	}

	log.Printf("Ready to produce kafka %v@%v '%v', %T\n", auth, hosts, p.topic, p.conf.balancer)

	return &p, nil
}

func (p *Producer) Produce(ctx context.Context, keyAndMessage ...[]byte) error {
	if len(keyAndMessage) == 0 {
		return nil
	}
	if len(keyAndMessage)&1 != 0 {
		return fmt.Errorf("keyAndMessage should be both pair values, given: %v", keyAndMessage)
	}
	kafkaMessages := make([]kafka.Message, 0, len(keyAndMessage)/2)
	for i := 0; i < len(keyAndMessage); i += 2 {
		kafkaMessages = append(kafkaMessages, kafka.Message{Key: keyAndMessage[i], Value: keyAndMessage[i+1]})
	}
	if err := p.writer.WriteMessages(ctx, kafkaMessages...); err != nil {
		return fmt.Errorf("Failed to produce messages, messages: %v, topic: %v, err: %w\n", keyAndMessage, p.topic, err)
	}
	if p.conf.log {
		log.Printf("Produce kafka message at topic %v: %#v\n", p.topic, keyAndMessage)
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

	if p.conf.credentials != nil {
		saslConfig, tlsConfig, err := (*p.conf.credentials).GetSaslAndTls(ctx)
		if err != nil {
			return err
		}
		if saslConfig != nil {
			dialer.SASLMechanism = *saslConfig
			dialer.TLS = tlsConfig
		}
	}

	conn, err := dialer.DialContext(ctx, "tcp", hosts[0])
	if err != nil {
		return fmt.Errorf("Can't connect to kafka for create topic: %w\n", err)
	}
	defer conn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             p.topic,
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
