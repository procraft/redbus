package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/caarlos0/env/v6"

	"github.com/procraft/redbus/internal/app/model"
)

type Config struct {
	Grpc    grpcConfig    `json:"grpc"`
	Control controlConfig `json:"control"`
	Metrics metricsConfig `json:"metrics"`
	Admin   adminConfig   `json:"admin"`
	Kafka   kafkaConfig   `json:"kafka"`
	Repeat  repeatConfig  `json:"repeat"`
	DB      dbConfig      `json:"db"`
	Log     logConfig     `json:"log"`
}

type logConfig struct {
	Json    bool `json:"json" env:"REDBUS_LOG_JSON"`
	Verbose bool `json:"verbose" env:"REDBUS_LOG_VERBOSE"`
}

type metricsConfig struct {
	ServerPort int `json:"serverPort" env:"REDBUS_METRICS_SERVER_PORT"`
}

type grpcConfig struct {
	ServerPort int `json:"serverPort" env:"REDBUS_GRPC_SERVER_PORT"`
	// KeepaliveTime — период ping'а простаивающего соединения, KeepaliveTimeout — ожидание ответа
	// на ping. Без них полумёртвый клиент остаётся CONNECTED, а его consumer держит партиции.
	KeepaliveTime    model.Duration `json:"keepaliveTime,string" env:"REDBUS_GRPC_KEEPALIVE_TIME"`
	KeepaliveTimeout model.Duration `json:"keepaliveTimeout,string" env:"REDBUS_GRPC_KEEPALIVE_TIMEOUT"`
	// KeepaliveMinTime — минимальный интервал ping'ов, который сервер терпит от клиента.
	KeepaliveMinTime model.Duration `json:"keepaliveMinTime,string" env:"REDBUS_GRPC_KEEPALIVE_MIN_TIME"`
	// Бюджет ожидания результата батча, если клиент не объявил свой consumeTimeoutSec в Connect.
	ConsumeResultTimeout model.Duration `json:"consumeResultTimeout,string" env:"REDBUS_GRPC_CONSUME_RESULT_TIMEOUT"`
	// Запас поверх batchSize * per-message timeout, чтобы не рвать легитимную долгую обработку.
	ConsumeResultSlack model.Duration `json:"consumeResultSlack,string" env:"REDBUS_GRPC_CONSUME_RESULT_SLACK"`
	// Верхняя граница рассчитанного бюджета.
	ConsumeResultTimeoutMax model.Duration `json:"consumeResultTimeoutMax,string" env:"REDBUS_GRPC_CONSUME_RESULT_TIMEOUT_MAX"`
}

// Значения по умолчанию применяются, когда поле не задано ни в config.json, ни в окружении:
// конфиг может приезжать из старого деплоя, а нулевой таймаут отключил бы защиту.
const (
	DefaultGrpcKeepaliveTime           = 30 * time.Second
	DefaultGrpcKeepaliveTimeout        = 10 * time.Second
	DefaultGrpcKeepaliveMinTime        = 10 * time.Second
	DefaultConsumeResultTimeout        = 60 * time.Second
	DefaultConsumeResultSlack          = 30 * time.Second
	DefaultConsumeResultTimeoutMax     = time.Hour
	minAllowedConsumeResultTimeoutUnit = time.Second
)

// ConsumeLimits возвращает бюджеты ожидания результата батча для consume-стрима.
func (c grpcConfig) ConsumeLimits() model.ConsumeLimits {
	return model.ConsumeLimits{
		PerMessage: durationOr(c.ConsumeResultTimeout, DefaultConsumeResultTimeout),
		Slack:      durationOr(c.ConsumeResultSlack, DefaultConsumeResultSlack),
		Max:        durationOr(c.ConsumeResultTimeoutMax, DefaultConsumeResultTimeoutMax),
	}
}

func (c grpcConfig) Keepalive() (time.Duration, time.Duration, time.Duration) {
	return durationOr(c.KeepaliveTime, DefaultGrpcKeepaliveTime),
		durationOr(c.KeepaliveTimeout, DefaultGrpcKeepaliveTimeout),
		durationOr(c.KeepaliveMinTime, DefaultGrpcKeepaliveMinTime)
}

func durationOr(v model.Duration, fallback time.Duration) time.Duration {
	if v.Duration < minAllowedConsumeResultTimeoutUnit {
		return fallback
	}
	return v.Duration
}

type controlConfig struct {
	ServerPort int `json:"serverPort" env:"REDBUS_CONTROL_SERVER_PORT"`
}

type adminConfig struct {
	ServerPort     int            `json:"serverPort" env:"REDBUS_ADMIN_SERVER_PORT"`
	Token          string         `json:"token" env:"REDBUS_ADMIN_TOKEN"`
	ApiHost        string         `json:"apiHost" env:"REDBUS_API_HOST"`
	ControlAddress string         `json:"controlAddress" env:"REDBUS_ADMIN_CONTROL_ADDRESS"`
	PollInterval   model.Duration `json:"pollInterval,string" env:"REDBUS_ADMIN_POLL_INTERVAL"`
	RequestTimeout model.Duration `json:"requestTimeout,string" env:"REDBUS_ADMIN_REQUEST_TIMEOUT"`
	StaticDir      string         `json:"staticDir" env:"REDBUS_ADMIN_STATIC_DIR"`
}

type kafkaConfig struct {
	HostPort               string                 `json:"hostPort" env:"REDBUS_KAFKA_HOST_PORT"`
	Credentials            KafkaCredentialsConfig `json:"credentials"`
	FailTimeout            model.Duration         `json:"failTimeout,string" env:"REDBUS_KAFKA_FAIL_TIMEOUT"`
	CreateTopicIfNotExists bool                   `json:"createTopicIfNotExists" env:"REDBUS_KAFKA_CREATE_TOPIC"`
	TopicNumPartitions     int                    `json:"topicNumPartitions" env:"REDBUS_KAFKA_TOPIC_NUM_PARTITIONS"`
	TopicReplicationFactor int                    `json:"topicReplicationFactor" env:"REDBUS_KAFKA_TOPIC_REPLICATION_FACTOR"`
}

type KafkaCredentialsConfig struct {
	Algo     string `json:"algo" env:"REDBUS_KAFKA_ALGO"`
	User     string `json:"user" env:"REDBUS_KAFKA_USER"`
	Password string `json:"password" env:"REDBUS_KAFKA_PASSWORD"`
	Cert     string `json:"cert" env:"REDBUS_KAFKA_CERT"`
}

type repeatConfig struct {
	Interval        model.Duration        `json:"interval,string" env:"REDBUS_REPEAT_INTERVAL"`
	DefaultStrategy *model.RepeatStrategy `json:"defaultStrategy"`
}

type dbConfig struct {
	Host string `json:"host" env:"REDBUS_DB_HOST"`
	// SSLMode — libpq sslmode for the bus database: disable, require, verify-ca or verify-full.
	// Empty means DefaultDBSSLMode, so a deployment that sets nothing still uses TLS.
	SSLMode string `json:"sslMode" env:"REDBUS_DB_SSLMODE"`
	// SSLRootCert — path to the CA certificate used by verify-ca / verify-full.
	SSLRootCert string `json:"sslRootCert" env:"REDBUS_DB_SSLROOTCERT"`
	Port        int    `json:"port" env:"REDBUS_DB_PORT"`
	User        string `json:"user" env:"REDBUS_DB_USER"`
	Password    string `json:"password" env:"REDBUS_DB_PASSWORD"`
	Name        string `json:"name" env:"REDBUS_DB_NAME"`
	PoolSize    int    `json:"poolSize" env:"REDBUS_DB_POOL_SIZE"`
}

// DefaultDBSSLMode applies when neither config.json nor the environment sets db.sslMode.
const DefaultDBSSLMode = "require"

var allowedDBSSLModes = map[string]bool{
	"disable":     true,
	"require":     true,
	"verify-ca":   true,
	"verify-full": true,
}

// SSL returns the effective sslmode and root certificate path of the bus database.
func (c dbConfig) SSL() (mode, rootCert string) {
	mode = c.SSLMode
	if mode == "" {
		mode = DefaultDBSSLMode
	}
	return mode, c.SSLRootCert
}

func (c dbConfig) validate() error {
	mode, _ := c.SSL()
	if !allowedDBSSLModes[mode] {
		return fmt.Errorf("db.sslMode (REDBUS_DB_SSLMODE) %q is not one of disable, require, verify-ca, verify-full", mode)
	}
	return nil
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
	if err := cfg.DB.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
