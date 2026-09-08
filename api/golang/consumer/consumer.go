package consumer

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
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

	connectPayload := pb.ConsumeRequest{Connect: &pb.ConsumeRequest_Connect{
		Id:             fmt.Sprintf("%d-%d", os.Getpid(), time.Now().Unix()),
		Topic:          topic,
		Group:          group,
		RepeatStrategy: toPBRepeatStrategy(listener.repeatStrategy),
		BatchSize:      int32(listener.batchSize),
		// Сообщаем шине свой бюджет обработки, чтобы её дедлайн ожидания результата не рвал
		// легитимную долгую обработку и при этом не был бесконечным.
		ConsumeTimeoutSec: consumeTimeoutSec(listener.consumeTimeout),
	}}

	// Один стрим — один обслуживающий цикл. Стрим передаётся параметром, а не через общую
	// переменную: иначе реконнект и обслуживание гонялись бы за одним значением, а результат
	// батча мог уйти уже в другой стрим.
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			stream, ok := c.waitConnectedStream(ctx, busClient, &connectPayload)
			if !ok {
				return
			}
			log.Printf("Connect to %v:%v, id = %v\n", c.host, c.port, connectPayload.Connect.Id)
			c.serveStream(ctx, stream, listener, processor)
			if ctx.Err() != nil {
				return
			}
			log.Printf("Connection to %v:%v not available, %v waiting...\n", c.host, c.port, c.unavailableTimeout)
			if !sleepCtx(ctx, c.unavailableTimeout) {
				return
			}
		}
	}()

	<-ctx.Done()
	log.Printf("Disconnected\n")
	return nil
}

// waitConnectedStream открывает стрим и подтверждает подключение, повторяя попытки до отмены ctx.
func (c *Service) waitConnectedStream(
	ctx context.Context,
	busClient pb.RedbusServiceClient,
	connectPayload *pb.ConsumeRequest,
) (pb.RedbusService_ConsumeClient, bool) {
	var attempt int
	for {
		if ctx.Err() != nil {
			return nil, false
		}
		attempt++
		stream, err := c.connectStream(ctx, busClient, connectPayload)
		if err == nil {
			return stream, true
		}
		log.Printf("Connect to %v:%v error: %v, attempt %v, %v waiting...\n", c.host, c.port, err, attempt, c.unavailableTimeout)
		if !sleepCtx(ctx, c.unavailableTimeout) {
			return nil, false
		}
	}
}

func (c *Service) connectStream(
	ctx context.Context,
	busClient pb.RedbusServiceClient,
	connectPayload *pb.ConsumeRequest,
) (pb.RedbusService_ConsumeClient, error) {
	stream, err := busClient.Consume(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(connectPayload); err != nil {
		return nil, err
	}
	connectResponse, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	if connectResponse.Connect == nil {
		return nil, fmt.Errorf("connect response is empty: %v", connectResponse)
	}
	if !connectResponse.Connect.Ok {
		return nil, fmt.Errorf("%s", connectResponse.Connect.Message)
	}
	return stream, nil
}

// serveStream обслуживает один стрим до его завершения.
func (c *Service) serveStream(
	ctx context.Context,
	stream pb.RedbusService_ConsumeClient,
	listener Listener,
	processor ConsumeProcessor,
) {
	for {
		if ctx.Err() != nil {
			return
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
		if len(payloadResponse.MessageList) == 0 {
			continue
		}
		messageIdList := fromPBMessageIds(payloadResponse.MessageList)
		log.Printf("Receive messages: %v\n", strings.Join(messageIdList, ","))

		// process messages
		processResultMap := c.processMessageList(ctx, listener, processor, payloadResponse.MessageList)

		// send result of process messages
		resultList := toPBResultList(processResultMap)
		// batchId возвращается шине как есть: по нему она отличает ответ на текущий батч от
		// позднего или чужого. Старые шины поле игнорируют.
		if err := stream.Send(&pb.ConsumeRequest{BatchId: payloadResponse.BatchId, ResultList: resultList}); err != nil {
			log.Printf("Can't send result of process messages: %v, error: %v\n", strings.Join(messageIdList, ","), err)
			return
		}
	}
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
	// Канал буферизован и не закрывается: после сработавшего таймаута обработчик всё ещё жив и
	// однажды запишет результат. Закрытый или небуферизованный канал давал бы здесь панику
	// "send on closed channel", а повторную — recover, пишущий в тот же канал.
	processErrCh := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				processErrCh <- fmt.Errorf("Recovered: %v", r)
			}
		}()
		processErrCh <- processor(processCtx, message.Data)
	}()

	select {
	case err := <-processErrCh:
		return err
	case <-processCtx.Done():
		if ctx.Err() != nil {
			return fmt.Errorf("Consumer stopped while processing %v", message.Id)
		}
		return fmt.Errorf("Execution timeout %v limit for %v", listener.consumeTimeout, message.Id)
	}
}

// sleepCtx ждёт duration и возвращает false, если ожидание прервано отменой контекста.
func sleepCtx(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func consumeTimeoutSec(timeout time.Duration) int32 {
	if timeout <= 0 {
		return 0
	}
	sec := math.Ceil(timeout.Seconds())
	if sec > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(sec)
}
