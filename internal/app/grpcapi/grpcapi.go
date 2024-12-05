package grpcapi

import (
	"context"
	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/config"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
)

type GrpcApi struct {
	conf     *config.Config
	dataBus  IDataBus
	repeater IRepeater
}

func New(
	conf *config.Config,
	dataBus IDataBus,
	repeater IRepeater,
) *GrpcApi {
	return &GrpcApi{
		conf:     conf,
		dataBus:  dataBus,
		repeater: repeater,
	}
}

type IDataBus interface {
	CreateConsumerConnection(ctx context.Context, kafkaHost []string, credentials *credential.Conf, topic, group, id string, batchSize int) (model.IConsumer, error)
	FindRepeatStrategy(topic, group, id string) *model.RepeatStrategy
	Consume(ctx context.Context, srv pb.RedbusService_ConsumeServer, topic, group, id string, repeatStrategy *model.RepeatStrategy, c model.IConsumer, handler func(ctx context.Context, list model.MessageList) error, cancel context.CancelFunc) error

	Produce(ctx context.Context, topic, key string, message []byte) error
}

type IRepeater interface {
	Add(ctx context.Context, data model.RepeatData, errorMsg string) error
	Repeat(ctx context.Context) error
}
