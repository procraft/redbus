package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/kafka/credential"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

var errHandler = errors.New("error in handler")

func (b *GrpcApi) Consume(srv pb.RedbusService_ConsumeServer) error {

	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	logger.Info(ctx, "Handle new consume")

	// Receive connect data
	ok, data, err := consumerRecv(ctx, nil, srv)
	if !ok || err != nil {
		return err
	}

	// Get kafka connection
	kafkaHost := []string{b.conf.Kafka.HostPort}
	c, connectErr := b.dataBus.CreateConsumerConnection(
		ctx,
		kafkaHost,
		credential.FromConf(b.conf.Kafka.Credentials),
		data.Connect.Topic,
		data.Connect.Group,
		data.Connect.Id,
		int(data.Connect.BatchSize),
	)
	var connectResult *pb.ConsumeResponse_Connect
	if connectErr != nil {
		connectResult = &pb.ConsumeResponse_Connect{Ok: false, Message: err.Error()}
	} else {
		connectResult = &pb.ConsumeResponse_Connect{Ok: true}
	}

	// Notify about connection
	if ok, err := consumerSend(ctx, c, srv, &pb.ConsumeResponse{Connect: connectResult}); !ok || err != nil {
		return err
	}

	if connectErr != nil {
		return connectErr
	}

	// Consume
	handler := func(ctx context.Context, list model.MessageList) error {
		logger.Consumer(ctx, c, "Receive %d messages (%s) from kafka and send", len(list), strings.Join(list.GetIdList(), ", "))
		data, err := SendToConsumerAndWaitResponse(ctx, c, srv, list)
		if err != nil {
			return fmt.Errorf("%w: %v", errHandler, err)
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
					return fmt.Errorf("%w: %v", errHandler, err)
				}
			}
		}
		return nil
	}

	repeatStrategy := fromPBRepeatStrategy(data.Connect.RepeatStrategy)
	return b.dataBus.Consume(ctx, srv, data.Connect.Topic, data.Connect.Group, data.Connect.Id, repeatStrategy, c, handler, cancel)
}

func consumerSend(ctx context.Context, c model.IConsumer, srv pb.RedbusService_ConsumeServer, data *pb.ConsumeResponse) (bool, error) {
	err := srv.Send(data)
	if err == io.EOF {
		return false, err
	}
	if err != nil {
		logger.Consumer(ctx, c, "Can't send to example client: %v", err)
		return true, err
	}
	return true, nil
}

func consumerRecv(ctx context.Context, c model.IConsumer, srv pb.RedbusService_ConsumeServer) (bool, *pb.ConsumeRequest, error) {
	rest, err := srv.Recv()
	if err == io.EOF {
		return false, nil, nil
	}
	if err != nil {
		logger.Consumer(ctx, c, "Can't receive from example client: %v", err)
		return true, nil, err
	}
	return true, rest, nil
}

func SendToConsumerAndWaitResponse(ctx context.Context, c model.IConsumer, srv pb.RedbusService_ConsumeServer, list model.MessageList) (*pb.ConsumeRequest, error) {
	ok, err := consumerSend(ctx, c, srv, &pb.ConsumeResponse{MessageList: toPBMessageList(list)})
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errHandler, err)
	}
	logger.Consumer(ctx, c, "Wait %d message processing", len(list))
	ok, data, err := consumerRecv(ctx, c, srv)
	if err == nil {
		for _, v := range data.ResultList {
			if v.Ok {
				logger.Consumer(ctx, c, "[%v] Message processing success", v.Id)
			} else {
				logger.Consumer(ctx, c, "[%v] Message processing error: %v", v.Id, v.Message)
			}
		}
	}
	if !ok {
		return nil, nil
	}
	return data, err
}
