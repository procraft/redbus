package config

import "time"

type Config struct {
	GrpcServerPort              int
	KafkaFailTimeout            time.Duration
	KafkaHostPort               string
	KafkaTopicNumPartitions     int
	KafkaTopicReplicationFactor int
	DB                          dbConfig
}

type kafkaConfig struct {
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
		GrpcServerPort:              50005,
		KafkaFailTimeout:            time.Second * 5,
		KafkaHostPort:               "127.0.0.1:9092",
		KafkaTopicNumPartitions:     3,
		KafkaTopicReplicationFactor: 1,
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
