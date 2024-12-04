package consumer

import (
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

func WithBatchSize(value int) Option {
	return func(conf *conf) {
		conf.batchSize = value
	}
}
