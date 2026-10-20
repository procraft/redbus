package producer

import (
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/procraft/redbus/internal/pkg/kafka/credential"
)

type Option func(conf *conf)

func WithLog() Option {
	return func(conf *conf) {
		conf.log = true
	}
}

func WithCredentials(algo, user, password, cert string) Option {
	return func(conf *conf) {
		conf.credentials = &credential.Conf{Algo: credential.Algo(algo), User: user, Password: password, Cert: cert}
	}
}

func WithBalancer(balancer kafka.Balancer) Option {
	return func(conf *conf) {
		conf.balancer = balancer
	}
}

func WithCreateTopic(numPartitions, replicationFactor int) Option {
	return func(conf *conf) {
		conf.createTopic = &CreateOptions{
			NumPartitions:     numPartitions,
			ReplicationFactor: replicationFactor,
		}
	}
}

// WithBatchTimeout overrides DefaultBatchTimeout, the time the writer waits to fill a batch before
// sending it. A non-positive value falls back to the kafka-go default of one second.
func WithBatchTimeout(d time.Duration) Option {
	return func(conf *conf) {
		conf.batchTimeout = d
	}
}
