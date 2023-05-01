package config

import (
	"time"

	"github.com/sergiusd/redbus/internal/app/model"
)

type Config struct {
	Grpc   grpcConfig
	Kafka  kafkaConfig
	Repeat repeatConfig
	DB     dbConfig
}

type grpcConfig struct {
	ServerPort int
}

type kafkaConfig struct {
	FailTimeout            time.Duration
	HostPort               string
	TopicNumPartitions     int
	TopicReplicationFactor int
}

type repeatConfig struct {
	Interval        time.Duration
	DefaultStrategy *model.RepeatStrategy
}

type dbConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	PoolSize int
}

func New() *Config {
	return &Config{
		Grpc: grpcConfig{
			ServerPort: 50005,
		},
		Kafka: kafkaConfig{
			FailTimeout:            time.Second * 5,
			HostPort:               "127.0.0.1:9092",
			TopicNumPartitions:     3,
			TopicReplicationFactor: 1,
		},
		Repeat: repeatConfig{
			Interval:        time.Second * 10,
			DefaultStrategy: model.NewRepeatStrategyEven(5, 5*time.Minute),
		},
		DB: dbConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "sergiusd",
			Password: "",
			Name:     "redbus",
			PoolSize: 10,
		},
	}
}
