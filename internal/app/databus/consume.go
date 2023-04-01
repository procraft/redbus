package databus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/sergiusd/redbus/api/golang/pb"

	"github.com/sergiusd/redbus/internal/pkg/kafka/consumer"
)

var errHandler = errors.New("error in handler")

func (b *DataBus) Consume(srv pb.StreamService_ConsumeServer) error {

	log.Printf("Handle new consume\n")

	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	// Receive connect data
	ok, data, err := b.consumerRecv(nil, srv)
	if !ok || err != nil {
		return err
	}

	// Get kafka example
	c, connectErr := consumer.New(ctx, b.kafkaHost, data.Connect.Topic, data.Connect.Group, data.Connect.Id, 0)
	var connectResult *pb.ConsumeResponse_Connect
	if connectErr != nil {
		b.consumerLog(c, "Failed connect to kafka: %v", connectErr)
		connectResult = &pb.ConsumeResponse_Connect{Ok: false, Message: err.Error()}
	} else {
		b.consumerLog(c, "Success connect to kafka")
		connectResult = &pb.ConsumeResponse_Connect{Ok: true}
	}

	// Notify about connection
	if ok, err := b.consumerSend(c, srv, &pb.ConsumeResponse{Connect: connectResult}); !ok || err != nil {
		return err
	}

	if connectErr != nil {
		return connectErr
	}

	// Consume
	b.consumerLog(c, "Start consuming")
	err = b.consumeProcess(ctx, cancel, srv, c)
	b.consumerLog(c, "Finish consuming: %v", err)
	return nil
}

func (b *DataBus) consumeProcess(ctx context.Context, cancel context.CancelFunc, srv pb.StreamService_ConsumeServer, c *consumer.Consumer) error {

	handler := func(ctx context.Context, messageKey, message []byte, id string) error {
		b.consumerLog(c, "[%s] Receive message from kafka and send", id)
		ok, err := b.consumerSend(c, srv, &pb.ConsumeResponse{Payload: &pb.ConsumeResponse_Payload{Id: id, Data: message}})
		if !ok {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %v", errHandler, err)
		}
		b.consumerLog(c, "[%v] Wait message processing", id)
		ok, data, err := b.consumerRecv(c, srv)
		if !ok {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %v", errHandler, err)
		}
		if data.Payload.Ok {
			b.consumerLog(c, "[%v] Message processing success", id)
		} else {
			b.consumerLog(c, "[%v] Message processing error: %v", id, data.Payload.Message)
			if err := b.repeater.Add(ctx, c.GetTopic(), c.GetGroup(), c.GetID(), messageKey, message, id, data.Payload.Message); err != nil {
				return fmt.Errorf("%w: %v", errHandler, err)
			}
		}
		return nil
	}

	go func() {
		defer func() {
			b.consumerLog(c, "Consume kafka stop")
			if err := c.Close(); err != nil {
				b.consumerLog(c, "Can't stop kafka example error: %v\n", err)
			}
		}()

		var attempt int
		var consumeErr error
		for {
			attempt++
			if attempt != 1 {
				b.consumerLog(c, "Consume kafka error: %v, %v waiting...", consumeErr, b.conf.KafkaFailTimeout)
				time.Sleep(b.conf.KafkaFailTimeout)
			}
			b.consumerLog(c, "Consume kafka starting...")
			consumeErr = c.Consume(ctx, func(ctx context.Context, msgKey, msg []byte, id string) error { return handler(ctx, msgKey, msg, id) })
			// handler error
			if errors.Is(consumeErr, errHandler) {
				cancel()
				return
			}
			// on done finish
			if errors.Is(consumeErr, context.Canceled) || consumeErr == nil {
				return
			}
		}
	}()

	<-ctx.Done()

	return nil
}

func (b *DataBus) consumerLog(c *consumer.Consumer, message string, args ...any) {
	if c == nil {
		args = append([]any{strings.Join(b.kafkaHost, ",")}, args...)
		log.Printf("[%v] "+message+"\n", args...)
		return
	}
	args = append([]any{strings.Join(b.kafkaHost, ","), c.GetTopic(), c.GetGroup(), c.GetID()}, args...)
	log.Printf("[%v/%v/%v/%v] "+message+"\n", args...)
}

func (b *DataBus) consumerSend(c *consumer.Consumer, srv pb.StreamService_ConsumeServer, data *pb.ConsumeResponse) (bool, error) {
	err := srv.Send(data)
	if err == io.EOF {
		return false, err
	}
	if err != nil {
		b.consumerLog(c, "Can't send to example client: %v", err)
		return true, err
	}
	return true, nil
}

func (b *DataBus) consumerRecv(c *consumer.Consumer, srv pb.StreamService_ConsumeServer) (bool, *pb.ConsumeRequest, error) {
	rest, err := srv.Recv()
	if err == io.EOF {
		return false, nil, nil
	}
	if err != nil {
		b.consumerLog(c, "Can't receive from example client: %v", err)
		return true, nil, err
	}
	return true, rest, nil
}
