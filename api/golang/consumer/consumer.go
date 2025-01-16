package consumer

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func New(host string, port int, options ...ServiceOptionFn) *Service {
	c := Service{
		host:               host,
		port:               port,
		unavailableTimeout: 60 * time.Second,
	}
	for _, o := range options {
		o(&c)
	}
	return &c
}

func (c *Service) Consume(ctx context.Context, topic, group string, processor ConsumeProcessor, options ...OptionFn) error {
	listener := Listener{
		consumeTimeout: 60 * time.Second,
		batchSize:      1,
	}
	for _, o := range options {
		o(&listener)
	}
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
		RepeatStrategy: toPBRepeatStrategy(listener.repeatStrategy),
		BatchSize:      int32(listener.batchSize),
	}

	// connect to topic
	connectPayload := pb.ConsumeRequest{Connect: connect}
	waitBusClientConnectedStream := func() pb.RedbusService_ConsumeClient {
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

			// receive messages
			payloadResponse, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("Can't receive payload: %v\n", err)
				return
			}
			messageIdList := fromPBMessageIds(payloadResponse.MessageList)
			log.Printf("Receive messages: %v\n", strings.Join(messageIdList, ","))

			// process messages
			processResultMap := c.processMessageList(ctx, listener, processor, payloadResponse.MessageList)

			// send result of process messages
			resultList := toPBResultList(processResultMap)
			if err := stream.Send(&pb.ConsumeRequest{ResultList: resultList}); err != nil {
				log.Printf("Can't send result of process messages: %v, error: %v\n", strings.Join(messageIdList, ","), err)
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

func (c *Service) processMessageList(
	ctx context.Context,
	listener Listener,
	processor ConsumeProcessor,
	messageList []*pb.ConsumeResponse_Message,
) []ProcessResult {
	if len(messageList) == 0 {
		return nil
	}
	if len(messageList) == 1 {
		err := c.processMessage(ctx, listener, processor, messageList[0])
		return []ProcessResult{{id: messageList[0].Id, err: err}}
	}
	resultCh := make(chan ProcessResult, len(messageList))
	for i := range messageList {
		go func(m *pb.ConsumeResponse_Message) {
			err := c.processMessage(ctx, listener, processor, m)
			resultCh <- ProcessResult{id: m.Id, err: err}
		}(messageList[i])
	}
	ret := make([]ProcessResult, 0, len(messageList))
	for i := 0; i < len(messageList); i++ {
		result := <-resultCh
		ret = append(ret, result)
	}
	return ret
}

func (c *Service) processMessage(
	ctx context.Context,
	listener Listener,
	processor ConsumeProcessor,
	message *pb.ConsumeResponse_Message,
) error {
	processCtx, processCancel := context.WithTimeout(ctx, listener.consumeTimeout)
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
		processErrCh <- processor(processCtx, message.Data)
	}()
	select {
	case <-processCtx.Done():
		processErr = fmt.Errorf("Execution timeout %v limit for %v", listener.consumeTimeout, message.Id)
	case err := <-processErrCh:
		processErr = err
	}

	return processErr
}
