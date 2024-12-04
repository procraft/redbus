package config

import (
	"encoding/json"
	"github.com/caarlos0/env/v6"
	"os"

	"github.com/prokraft/redbus/internal/app/model"
)

type Config struct {
	Grpc   grpcConfig   `json:"grpc"`
	Admin  adminConfig  `json:"admin"`
	Kafka  kafkaConfig  `json:"kafka"`
	Repeat repeatConfig `json:"repeat"`
	DB     dbConfig     `json:"db"`
}

type grpcConfig struct {
	ServerPort int `json:"serverPort" env:"REDBUS_GRPC_SERVER_PORT"`
}

type adminConfig struct {
	ServerPort int `json:"serverPort" env:"REDBUS_ADMIN_SERVER_PORT"`
}

type kafkaConfig struct {
	FailTimeout            model.Duration `json:"failTimeout,string" env:"REDBUS_KAFKA_FAIL_TIMEOUT"`
	HostPort               string         `json:"hostPort" env:"REDBUS_KAFKA_HOST_PORT"`
	TopicNumPartitions     int            `json:"topicNumPartitions" env:"REDBUS_KAFKA_TOPIC_NUM_PARTITIONS"`
	TopicReplicationFactor int            `json:"topicReplicationFactor" env:"REDBUS_KAFKA_TOPIC_REPLICATION_FACTOR"`
}

type repeatConfig struct {
	Interval        model.Duration        `json:"interval,string" env:"REDBUS_REPEAT_INTERVAL"`
	DefaultStrategy *model.RepeatStrategy `json:"defaultStrategy"`
}

type dbConfig struct {
	Host     string `json:"host" env:"REDBUS_DB_HOST"`
	Port     int    `json:"port" env:"REDBUS_DB_PORT"`
	User     string `json:"user" env:"REDBUS_DB_USER"`
	Password string `json:"password" env:"REDBUS_DB_PASSWORD"`
	Name     string `json:"name" env:"REDBUS_DB_NAME"`
	PoolSize int    `json:"poolSize" env:"REDBUS_DB_POOL_SIZE"`
}

func FromFileAndEnv(mainPath string, extraPath ...string) (*Config, error) {
	var cfg Config

	allPath := []string{mainPath}
	allPath = append(allPath, extraPath...)

	for _, path := range allPath {
		if _, err := os.Stat(path); err == nil {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, err
			}
		}
	}

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
