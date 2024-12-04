package producer

import (
	"github.com/segmentio/kafka-go"

	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
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
