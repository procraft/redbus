package databus

import (
	"context"

	"github.com/sergiusd/redbus/internal/app/config"
)

type DataBus struct {
	conf             *config.Config
	kafkaHost        []string
	createProducerFn CreateProducerFn
	producerMap      map[string]IProducer
	repeater         IRepeater
}

func New(
	conf *config.Config,
	createProducerFn CreateProducerFn,
	repeater IRepeater,
) *DataBus {
	return &DataBus{
		conf:             conf,
		kafkaHost:        []string{conf.KafkaHostPort},
		createProducerFn: createProducerFn,
		producerMap:      make(map[string]IProducer),
		repeater:         repeater,
	}
}

type CreateProducerFn = func(ctx context.Context, topic string) (IProducer, error)

type IProducer interface {
	Produce(ctx context.Context, keyAndMessage ...string) error
	Close() error
}

type IRepeater interface {
	Add(ctx context.Context, topic, group, consumerId string, messageKey, message []byte, messageId, errorMessage string) error
}
