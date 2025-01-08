package grpcapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"github.com/prokraft/redbus/internal/pkg/logger"
	"github.com/prokraft/redbus/internal/pkg/stream"
)

func (b *GrpcApi) Consume(server pb.RedbusService_ConsumeServer) error {

	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	logger.Info(ctx, "Handle new consume")
	serverStream := stream.New(server)

	// Receive connect data
	ok, data, err := serverStream.Recv(ctx, nil)
	if !ok || err != nil {
		return err
	}

	// Get consumer with kafka connection
	kafkaHost := []string{b.conf.Kafka.HostPort}
	c, connectErr := b.dataBus.CreateConsumer(
		ctx,
		kafkaHost,
		credential.FromConf(b.conf.Kafka.Credentials),
		model.TopicName(data.Connect.Topic),
		model.GroupName(data.Connect.Group),
		model.ConsumerId(data.Connect.Id),
		int(data.Connect.BatchSize),
	)
	var connectResult *pb.ConsumeResponse_Connect
	if connectErr != nil {
		connectResult = &pb.ConsumeResponse_Connect{Ok: false, Message: connectErr.Error()}
	} else {
		connectResult = &pb.ConsumeResponse_Connect{Ok: true}
	}

	// Notify about connection
	if ok, err := serverStream.Send(ctx, c, &pb.ConsumeResponse{Connect: connectResult}); !ok || err != nil {
		return err
	}

	if connectErr != nil {
		return connectErr
	}

	// Consume
	handler := func(ctx context.Context, list model.MessageList) error {
		logger.Consumer(ctx, c, "Receive %d messages (%s) from kafka and send", len(list), strings.Join(list.GetIdList(), ", "))
		data, err := serverStream.ProcessMessageList(ctx, c, list)
		if err != nil {
			return fmt.Errorf("%w: %v", model.ErrHandler, err)
		}
		for i := range data.ResultList {
			result := data.ResultList[i]
			m := list.GetById(result.Id)
			if !result.Ok {
				var key *[]byte
				if len(m.Key) != 0 {
					key = &m.Key
				}
				if err := b.repeater.Add(ctx, model.RepeatData{
					Topic:      c.GetTopic(),
					Group:      c.GetGroup(),
					ConsumerId: c.GetID(),
					Key:        key,
					Message:    m.Value,
					MessageId:  m.Id,
					Strategy:   b.dataBus.FindRepeatStrategy(c.GetTopic(), c.GetGroup(), c.GetID()),
				}, result.Message); err != nil {
					return fmt.Errorf("%w: %v", model.ErrHandler, err)
				}
			}
		}
		return nil
	}

	repeatStrategy := fromPBRepeatStrategy(data.Connect.RepeatStrategy)
	return b.dataBus.Consume(ctx, c, server, repeatStrategy, handler, cancel)
}
