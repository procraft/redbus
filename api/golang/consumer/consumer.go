package consumer

import (
	"context"
	"fmt"
	"github.com/sergiusd/redbus/api/golang/pb"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc/credentials/insecure"

	"google.golang.org/grpc"
)

type Consumer struct {
	host               string
	port               int
	unavailableTimeout time.Duration
}

func New(host string, port int, unavailableTimeout time.Duration) *Consumer {
	return &Consumer{
		host:               host,
		port:               port,
		unavailableTimeout: unavailableTimeout,
	}
}

func (c *Consumer) Consume(ctx context.Context, topic, group string, processor func(data []byte, id string) error) error {
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
	busClient := pb.NewStreamServiceClient(busConn)

	connect := &pb.ConsumeRequest_Connect{
		Id:    fmt.Sprintf("%d-%d", os.Getpid(), time.Now().Unix()),
		Topic: topic,
		Group: group,
	}

	// connect to topic
	waitBusClientConnectedStream := func() pb.StreamService_ConsumeClient {
		connectPayload := pb.ConsumeRequest{Connect: connect}
		var stream pb.StreamService_ConsumeClient
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
			processErr := processor(payloadResponse.Payload.Data, payloadResponse.Payload.Id)

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
