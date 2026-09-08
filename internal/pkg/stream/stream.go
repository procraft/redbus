package stream

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

// batchSeq нумерует батчи в пределах процесса. Один и тот же consume-стрим обслуживают и цикл
// чтения kafka, и repeater, поэтому счётчик общий, а не привязан к Stream.
var batchSeq uint64

func nextBatchID() string {
	return "b" + strconv.FormatUint(atomic.AddUint64(&batchSeq, 1), 10)
}

// AbortFn завершает consume-RPC с указанной причиной.
//
// Отмена контекста сама по себе не будит заблокированный server.Recv(): его пробуждает только
// завершение RPC, то есть возврат из gRPC-хендлера. Поэтому abort должен и запомнить причину
// (чтобы хендлер вернул её как статус, и клиент получил onError), и отменить контекст хендлера.
type AbortFn func(err error)

type Stream struct {
	server pb.RedbusService_ConsumeServer
	abort  AbortFn
	limits model.ConsumeLimits
}

type Option func(*Stream)

// WithAbort передаёт способ закрыть стрим, когда клиент не прислал результат батча в срок.
func WithAbort(abort AbortFn) Option {
	return func(s *Stream) { s.abort = abort }
}

// WithLimits задаёт бюджет ожидания результата батча. Без него ожидание не ограничивается.
func WithLimits(limits model.ConsumeLimits) Option {
	return func(s *Stream) { s.limits = limits }
}

func New(server pb.RedbusService_ConsumeServer, options ...Option) *Stream {
	s := &Stream{server: server}
	for _, o := range options {
		o(s)
	}
	return s
}

func (s Stream) Send(ctx context.Context, c model.IConsumer, data *pb.ConsumeResponse) (bool, error) {
	err := s.server.Send(data)
	if err == io.EOF {
		return false, err
	}
	if err != nil {
		logger.Consumer(ctx, c, "Can't send to example client: %v", err)
		return true, err
	}
	return true, nil
}

func (s Stream) Recv(ctx context.Context, c model.IConsumer) (bool, *pb.ConsumeRequest, error) {
	rest, err := s.server.Recv()
	if err == io.EOF {
		return false, nil, nil
	}
	if err != nil {
		logger.Consumer(ctx, c, "Can't receive from example client: %v", err)
		return true, nil, err
	}
	return true, rest, nil
}

// ProcessMessageList отправляет батч клиенту и ждёт результат именно этого батча.
//
// Возвращает (nil, nil), если клиент закрыл стрим. Ошибка оборачивает model.ErrStream.
func (s Stream) ProcessMessageList(ctx context.Context, c model.IConsumer, list model.MessageList) (*pb.ConsumeRequest, error) {
	// обработка должна быть последовательно, чтобы запрос / ответ работали правильно
	c.Lock()
	defer c.Unlock()

	batchID := nextBatchID()
	ok, err := s.Send(ctx, c, &pb.ConsumeResponse{BatchId: batchID, MessageList: toPBMessageList(list)})
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrStream, err)
	}

	timeout := s.limits.BatchTimeout(len(list))
	logger.Consumer(ctx, c, "[%s] Wait %d message processing, timeout %v", batchID, len(list), timeout)
	watchdog := s.startResultWatchdog(ctx, c, batchID, timeout)
	defer watchdog.stop()

	for {
		ok, data, err := s.Recv(ctx, c)
		if !ok {
			return nil, nil
		}
		if err != nil {
			if watchdog.fired() {
				return nil, fmt.Errorf("%w: no result for %s within %v, closing stream", model.ErrStream, batchID, timeout)
			}
			return nil, fmt.Errorf("%w: %v", model.ErrStream, err)
		}
		// Клиенты, собранные до появления batchId, присылают пустое значение: для них остаётся
		// позиционное сопоставление. Чужой непустой batchId — это поздний или лишний ответ,
		// он отбрасывается, иначе фаза "батч ↔ ответ" сдвинулась бы навсегда.
		if data.BatchId != "" && data.BatchId != batchID {
			logger.Consumer(ctx, c, "[%s] Discard result of foreign batch %s (%d results)",
				batchID, data.BatchId, len(data.ResultList))
			continue
		}
		for _, v := range data.ResultList {
			if v.Ok {
				logger.Consumer(ctx, c, "[%s] [%v] Message processing success", batchID, v.Id)
			} else {
				logger.Consumer(ctx, c, "[%s] [%v] Message processing error: %v", batchID, v.Id, v.Message)
			}
		}
		return data, nil
	}
}

type resultWatchdog struct {
	timer   *time.Timer
	expired atomic.Bool
}

func (w *resultWatchdog) fired() bool {
	return w != nil && w.expired.Load()
}

func (w *resultWatchdog) stop() {
	if w != nil && w.timer != nil {
		w.timer.Stop()
	}
}

// startResultWatchdog закрывает RPC, если клиент не прислал результат батча за timeout.
// Завершение RPC — единственный способ разбудить заблокированный server.Recv(): второй Recv в
// отдельной горутине запускать нельзя, gRPC этого не допускает.
func (s Stream) startResultWatchdog(ctx context.Context, c model.IConsumer, batchID string, timeout time.Duration) *resultWatchdog {
	if timeout <= 0 || s.abort == nil {
		return nil
	}
	w := &resultWatchdog{}
	w.timer = time.AfterFunc(timeout, func() {
		w.expired.Store(true)
		logger.Consumer(ctx, c, "[%s] No result within %v, closing consume stream", batchID, timeout)
		s.abort(fmt.Errorf("%w: no result for %s within %v", model.ErrStream, batchID, timeout))
	})
	return w
}
