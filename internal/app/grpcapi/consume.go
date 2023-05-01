package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/sergiusd/redbus/api/golang/pb"
	"github.com/sergiusd/redbus/internal/app/model"
	"github.com/sergiusd/redbus/internal/pkg/logger"
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
	c, connectErr := b.dataBus.CreateConsumerConnection(ctx, kafkaHost, data.Connect.Topic, data.Connect.Group, data.Connect.Id)
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
	handler := func(ctx context.Context, messageKey, message []byte, id string) error {
		logger.Consumer(ctx, c, "[%s] Receive message from kafka and send", id)
		data, err := SendToConsumerAndWaitResponse(ctx, c, srv, message, id)
		if err != nil {
			return fmt.Errorf("%w: %v", errHandler, err)
		}
		if !data.Payload.Ok {
			var key *[]byte
			if len(messageKey) != 0 {
				key = &messageKey
			}
			if err := b.repeater.Add(ctx, model.RepeatData{
				Topic:      c.GetTopic(),
				Group:      c.GetGroup(),
				ConsumerId: c.GetID(),
				Key:        key,
				Message:    message,
				MessageId:  id,
				Strategy:   b.dataBus.FindRepeatStrategy(c.GetTopic(), c.GetGroup(), c.GetID()),
			}, data.Payload.Message); err != nil {
				return fmt.Errorf("%w: %v", errHandler, err)
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

func SendToConsumerAndWaitResponse(ctx context.Context, c model.IConsumer, srv pb.RedbusService_ConsumeServer, message []byte, id string) (*pb.ConsumeRequest, error) {
	ok, err := consumerSend(ctx, c, srv, &pb.ConsumeResponse{Payload: &pb.ConsumeResponse_Payload{Id: id, Data: message}})
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errHandler, err)
	}
	logger.Consumer(ctx, c, "[%v] Wait message processing", id)
	ok, data, err := consumerRecv(ctx, c, srv)
	if err == nil {
		if data.Payload.Ok {
			logger.Consumer(ctx, c, "[%v] Message processing success", id)
		} else {
			logger.Consumer(ctx, c, "[%v] Message processing error: %v", id, data.Payload.Message)
		}
	}
	if !ok {
		return nil, nil
	}
	return data, err
}
