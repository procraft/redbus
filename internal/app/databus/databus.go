package databus

import (
	"github.com/sergiusd/redbus/internal/app/config"
	"github.com/sergiusd/redbus/internal/pkg/kafka/producer"
)

type DataBus struct {
	conf        config.Config
	kafkaHost   []string
	producerMap map[string]*producer.Producer
}

func New(conf config.Config) *DataBus {
	return &DataBus{
		conf:        conf,
		kafkaHost:   []string{conf.KafkaHostPort},
		producerMap: make(map[string]*producer.Producer),
	}
}
