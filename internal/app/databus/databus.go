package databus

import (
	"context"

	"github.com/sergiusd/redbus/internal/app/model"

	"github.com/sergiusd/redbus/internal/app/config"
)

type DataBus struct {
	conf          *config.Config
	kafkaHost     []string
	producerStore *model.ProducerStore
	consumerStore *model.ConsumerStore
	repeater      IRepeater
}

func New(
	conf *config.Config,
	createProducerFn model.CreateProducerFn,
	repeater IRepeater,
) *DataBus {
	return &DataBus{
		conf:          conf,
		kafkaHost:     []string{conf.KafkaHostPort},
		producerStore: model.NewProducerStore(createProducerFn),
		consumerStore: model.NewConsumerStore(),
		repeater:      repeater,
	}
}

type IRepeater interface {
	Add(ctx context.Context, data model.RepeatData, errorMsg string) error
	Repeat(ctx context.Context) error
}
