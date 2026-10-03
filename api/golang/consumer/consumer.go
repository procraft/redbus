package consumer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/procraft/redbus/api/golang/inbox"
	"github.com/procraft/redbus/api/golang/pb"
)

func New(host string, port int, options ...ServiceOptionFn) *Service {
	c := Service{
		host:               host,
		port:               port,
		unavailableTimeout: 60 * time.Second,
		log:                slog.Default(),
	}
	for _, o := range options {
		o(&c)
	}
	return &c
}

// Consume is ConsumeMessages for a handler that needs only the payload.
func (c *Service) Consume(ctx context.Context, topic, group string, processor ConsumeProcessor, options ...OptionFn) error {
	return c.ConsumeMessages(ctx, topic, group, func(ctx context.Context, msg Message) error {
		return processor(ctx, msg.Data)
	}, options...)
}

// ConsumeMessages consumes topic as group until ctx is cancelled, reconnecting after every
// failure. Messages of one batch are handled concurrently; the batch result is sent after all of
// them finish. It returns nil after cancellation, once the stream loop has stopped.
func (c *Service) ConsumeMessages(ctx context.Context, topic, group string, handler Handler, options ...OptionFn) error {
	listener := Listener{
		consumeTimeout: 60 * time.Second,
		batchSize:      1,
	}
	for _, o := range options {
		o(&listener)
	}
	if listener.inboxMode != inbox.Disabled && listener.inboxDB == nil {
		return fmt.Errorf("redbus: inbox mode %v needs a database", listener.inboxMode)
	}
	var store inboxStore
	if listener.inboxDB != nil {
		store = sqlInboxStore{db: listener.inboxDB}
	}
	log := c.log.With("topic", topic, "group", group)

	// bus client
	busConn, err := grpc.Dial(
		fmt.Sprintf("%v:%v", c.host, c.port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}
	defer busConn.Close()
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

	s := session{
		service:  c,
		listener: listener,
		topic:    topic,
		group:    group,
		handler:  handler,
		store:    store,
		log:      log,
	}

	// Один стрим — один обслуживающий цикл. Стрим передаётся параметром, а не через общую
	// переменную: иначе реконнект и обслуживание гонялись бы за одним значением, а результат
	// батча мог уйти уже в другой стрим.
	log.Info("redbus: start consumer")
	for {
		stream, ok := c.waitConnectedStream(ctx, busClient, &connectPayload, log)
		if !ok {
			break
		}
		log.Info("redbus: connected", "addr", c.addr(), "id", connectPayload.Connect.Id)
		s.serve(ctx, stream)
		if ctx.Err() != nil {
			break
		}
		log.Warn("redbus: connection not available", "addr", c.addr(), "wait", c.unavailableTimeout)
		if !sleepCtx(ctx, c.unavailableTimeout) {
			break
		}
	}
	log.Info("redbus: consumer stopped")
	return nil
}

func (c *Service) addr() string {
	return fmt.Sprintf("%v:%v", c.host, c.port)
}

// waitConnectedStream открывает стрим и подтверждает подключение, повторяя попытки до отмены ctx.
func (c *Service) waitConnectedStream(
	ctx context.Context,
	busClient pb.RedbusServiceClient,
	connectPayload *pb.ConsumeRequest,
	log *slog.Logger,
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
		if ctx.Err() != nil {
			return nil, false
		}
		log.Warn("redbus: connect error", "addr", c.addr(), "error", err, "attempt", attempt, "wait", c.unavailableTimeout)
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

// session holds what serving one consumer needs; the gRPC stream itself is passed per call.
type session struct {
	service  *Service
	listener Listener
	topic    string
	group    string
	handler  Handler
	store    inboxStore
	log      *slog.Logger
}

// serve обслуживает один стрим до его завершения.
func (s *session) serve(ctx context.Context, stream pb.RedbusService_ConsumeClient) {
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
			if ctx.Err() == nil {
				s.log.Warn("redbus: can't receive payload", "error", err)
			}
			return
		}
		if len(payloadResponse.MessageList) == 0 {
			continue
		}
		messageIdList := fromPBMessageIds(payloadResponse.MessageList)
		s.log.Debug("redbus: receive messages", "ids", strings.Join(messageIdList, ","))

		// process messages
		processResultMap := s.processMessageList(ctx, payloadResponse.MessageList)

		// send result of process messages
		resultList := toPBResultList(processResultMap, s.log)
		// batchId возвращается шине как есть: по нему она отличает ответ на текущий батч от
		// позднего или чужого. Старые шины поле игнорируют.
		if err := stream.Send(&pb.ConsumeRequest{BatchId: payloadResponse.BatchId, ResultList: resultList}); err != nil {
			if ctx.Err() != nil {
				return
			}
			s.log.Warn("redbus: can't send result of process messages", "ids", strings.Join(messageIdList, ","), "error", err)
			return
		}
	}
}

func (s *session) processMessageList(ctx context.Context, messageList []*pb.ConsumeResponse_Message) []ProcessResult {
	if len(messageList) == 0 {
		return nil
	}
	if len(messageList) == 1 {
		err := s.processMessage(ctx, messageList[0])
		return []ProcessResult{{id: messageList[0].Id, err: err}}
	}
	resultCh := make(chan ProcessResult, len(messageList))
	for i := range messageList {
		go func(m *pb.ConsumeResponse_Message) {
			err := s.processMessage(ctx, m)
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

func (s *session) processMessage(ctx context.Context, message *pb.ConsumeResponse_Message) error {
	processCtx, processCancel := context.WithTimeout(ctx, s.listener.consumeTimeout)
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
		processErrCh <- processWithInbox(
			processCtx, s.store, s.listener.inboxMode, s.group, s.topic, fromPBMessage(message, s.log), s.handler, s.log,
		)
	}()

	select {
	case err := <-processErrCh:
		return err
	case <-processCtx.Done():
		if ctx.Err() != nil {
			return fmt.Errorf("Consumer stopped while processing %v", message.Id)
		}
		return fmt.Errorf("Execution timeout %v limit for %v", s.listener.consumeTimeout, message.Id)
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
