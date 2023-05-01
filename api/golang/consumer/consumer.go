package consumer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/sergiusd/redbus/api/golang/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func New(host string, port int, options ...OptionFn) *Consumer {
	c := Consumer{
		host:               host,
		port:               port,
		unavailableTimeout: 60 * time.Second,
		consumeTimeout:     60 * time.Second,
	}
	for _, o := range options {
		o(&c)
	}
	return &c
}

func (c *Consumer) Consume(ctx context.Context, topic, group string, processor ConsumeProcessor) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// bus client
	busConn, err := grpc.Dial(
		fmt.Sprintf("%v:%v", c.host, c.port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	busClient := pb.NewRedbusServiceClient(busConn)

	connect := &pb.ConsumeRequest_Connect{
		Id:             fmt.Sprintf("%d-%d", os.Getpid(), time.Now().Unix()),
		Topic:          topic,
		Group:          group,
		RepeatStrategy: toPBRepeatStrategy(c.repeatStrategy),
	}

	// connect to topic
	waitBusClientConnectedStream := func() pb.RedbusService_ConsumeClient {
		connectPayload := pb.ConsumeRequest{Connect: connect}
		var stream pb.RedbusService_ConsumeClient
		var connectResponse *pb.ConsumeResponse
		var streamErr error
		var attempt int
		for {
			attempt++
			if attempt != 1 {
				log.Printf("Connect to %v:%v error: %v, attempt %v, %v waiting...\n", c.host, c.port, streamErr, attempt, c.unavailableTimeout)
				time.Sleep(c.unavailableTimeout)
			}
			stream, streamErr = busClient.Consume(ctx)
			if streamErr != nil {
				continue
			}
			streamErr = stream.Send(&connectPayload)
			if streamErr != nil {
				continue
			}
			connectResponse, streamErr = stream.Recv()
			if streamErr != nil {
				continue
			}
			if !connectResponse.Connect.Ok {
				streamErr = fmt.Errorf(connectResponse.Connect.Message)
				continue
			}
			break
		}
		log.Printf("Connect to %v:%v, id = %v\n", c.host, c.port, connect.Id)
		return stream
	}

	stream := waitBusClientConnectedStream()

	// serve messages from stream
	serveStream := func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// receive payload
			payloadResponse, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("Can't receive payload: %v\n", err)
				return
			}
			log.Printf("[%v] Receive payload\n", payloadResponse.Payload.Id)

			// process
			processErr := c.processMessage(ctx, processor, payloadResponse.Payload)

			// send result of process payload
			var payloadResult *pb.ConsumeRequest_Payload
			if processErr == nil {
				log.Printf("[%v] Process payload success\n", payloadResponse.Payload.Id)
				payloadResult = &pb.ConsumeRequest_Payload{Ok: true}
			} else {
				log.Printf("[%v] Process payload error: %v\n", payloadResponse.Payload.Id, processErr)
				payloadResult = &pb.ConsumeRequest_Payload{Ok: false, Message: processErr.Error()}
			}
			if err := stream.Send(&pb.ConsumeRequest{Payload: payloadResult}); err != nil {
				log.Printf("[%v] Can't send result of process payload: %v\n", payloadResponse.Payload.Id, err)
				return
			}
		}
	}

	// reconnect
	go func() {
		for {
			select {
			case <-ctx.Done():
				cancel()
				return
			case <-stream.Context().Done():
				log.Printf("Connection to %v:%v not available, %v waiting...\n", c.host, c.port, c.unavailableTimeout)
				time.Sleep(c.unavailableTimeout)
				stream = waitBusClientConnectedStream()
				go serveStream()
			}
		}
	}()

	go serveStream()

	<-ctx.Done()
	log.Printf("Disconnected\n")
	return nil
}

func (c *Consumer) processMessage(ctx context.Context, processor ConsumeProcessor, payload *pb.ConsumeResponse_Payload) error {
	processCtx, processCancel := context.WithTimeout(ctx, c.consumeTimeout)
	defer processCancel()
	processErrCh := make(chan error)
	defer close(processErrCh)

	var processErr error
	go func() {
		defer func() {
			if r := recover(); r != nil {
				processErrCh <- fmt.Errorf("Recovered: %v", r)
			}
		}()
		processErrCh <- processor(processCtx, payload.Data, payload.Id)
	}()
	select {
	case <-processCtx.Done():
		processErr = fmt.Errorf("Execution timeout %v limit for %v", c.consumeTimeout, payload.Id)
	case err := <-processErrCh:
		processErr = err
	}

	return processErr
}
