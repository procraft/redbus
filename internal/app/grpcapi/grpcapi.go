package grpcapi

import (
	"context"
	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/config"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"time"
)

type GrpcApi struct {
	conf     *config.Config
	dataBus  IDataBus
	repeater IRepeater
	metrics  IMetrics
	pb.UnimplementedRedbusServiceServer
}

func (b *GrpcApi) mustEmbedUnimplementedRedbusServiceServer() {
	panic("implement me")
}

func New(
	conf *config.Config,
	dataBus IDataBus,
	repeater IRepeater,
	metrics IMetrics,
) *GrpcApi {
	return &GrpcApi{
		conf:     conf,
		dataBus:  dataBus,
		repeater: repeater,
		metrics:  metrics,
	}
}

type IMetrics interface {
	ObserveConsumed(topic, group, result string, count int)
	ObserveConsumerBatch(topic, group string, size int, duration time.Duration)
}

type IDataBus interface {
	CreateConsumer(ctx context.Context, kafkaHost []string, credentials *credential.Conf, topic model.TopicName, group model.GroupName, id model.ConsumerId, batchSize int) (model.IConsumer, error)
	FindRepeatStrategy(topic model.TopicName, group model.GroupName, id model.ConsumerId) *model.RepeatStrategy
	Consume(ctx context.Context, c model.IConsumer, srv pb.RedbusService_ConsumeServer, repeatStrategy *model.RepeatStrategy, handler func(ctx context.Context, list model.MessageList) error, cancel context.CancelFunc) error

	Produce(ctx context.Context, topic model.TopicName, key string, message []byte, version int64, idempotencyKey string, timestamp *time.Time) error
}

type IRepeater interface {
	Add(ctx context.Context, data model.RepeatData, errorMsg string) error
	Repeat(ctx context.Context) error
}
