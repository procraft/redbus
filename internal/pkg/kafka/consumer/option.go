package consumer

import (
	"github.com/procraft/redbus/internal/pkg/kafka/credential"
)

type Option func(conf *conf)

func WithMessageIdNamespace(value string) Option {
	return func(conf *conf) {
		conf.messageIdNamespace = value
	}
}

func WithLog() Option {
	return func(conf *conf) {
		conf.log = true
	}
}

func WithCredentials(credentials *credential.Conf) Option {
	return func(conf *conf) {
		conf.credentials = credentials
	}
}

func WithBatchSize(value int) Option {
	return func(conf *conf) {
		conf.batchSize = value
	}
}
