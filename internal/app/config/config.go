package config

import "time"

type Config struct {
	ServerPort                  int
	KafkaFailTimeout            time.Duration
	KafkaHostPort               string
	KafkaTopicNumPartitions     int
	KafkaTopicReplicationFactor int
}

func New() Config {
	return Config{
		ServerPort:                  50005,
		KafkaFailTimeout:            time.Second * 5,
		KafkaHostPort:               "127.0.0.1:9092",
		KafkaTopicNumPartitions:     3,
		KafkaTopicReplicationFactor: 1,
	}
}
