package grpcapi

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	connectStream := stream.New(server)

	// Receive connect data
	ok, data, err := connectStream.Recv(ctx, nil)
	if !ok || err != nil {
		return err
	}
	if data.Connect == nil {
		logger.Info(ctx, "First consume request has no connect data, closing stream")
		return fmt.Errorf("%w: first request must contain connect data", model.ErrStream)
	}

	// Бюджет ожидания результата батча: заявленный клиентом per-message таймаут, иначе серверный.
	limits := b.conf.Grpc.ConsumeLimits().
		WithPerMessage(time.Duration(data.Connect.ConsumeTimeoutSec) * time.Second)
	// Причина закрытия стрима запоминается отдельно от отмены контекста: заблокированный
	// server.Recv() просыпается только при завершении RPC, то есть уже после возврата отсюда,
	// поэтому статус ошибки нужно вернуть самому хендлеру.
	abortErrCh := make(chan error, 1)
	abort := func(err error) {
		select {
		case abortErrCh <- err:
		default:
		}
		cancel()
	}
	serverStream := stream.New(server, stream.WithAbort(abort), stream.WithLimits(limits))

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
	if ok, err := connectStream.Send(ctx, c, &pb.ConsumeResponse{Connect: connectResult}); !ok || err != nil {
		return err
	}

	if connectErr != nil {
		return connectErr
	}

	// Consume
	handler := func(ctx context.Context, list model.MessageList) error {
		logger.Consumer(ctx, c, "Receive %d messages (%s) from kafka and send", len(list), strings.Join(list.GetIdList(), ", "))
		startedAt := time.Now()
		data, err := serverStream.ProcessMessageList(ctx, c, list)
		b.metrics.ObserveConsumerBatch(string(c.GetTopic()), string(c.GetGroup()), len(list), time.Since(startedAt))
		if err != nil {
			b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "stream_error", len(list))
			return fmt.Errorf("%w: %v", model.ErrHandler, err)
		}
		if data == nil {
			b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "stream_closed", len(list))
			return fmt.Errorf("%w: consume stream closed without result", model.ErrHandler)
		}
		byID := list.IndexByID()
		successCount := 0
		retryCount := 0
		matchedCount := 0
		for i := range data.ResultList {
			result := data.ResultList[i]
			m, ok := byID[result.Id]
			if !ok {
				// Лишний или чужой результат внутри принятого ответа: логируем и пропускаем,
				// валить из-за него consumer нельзя.
				b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "invalid_result", 1)
				logger.Consumer(ctx, c, "Skip result id %q, not in batch, have [%s]", result.Id, strings.Join(list.GetIdList(), ", "))
				continue
			}
			matchedCount++
			if result.Ok {
				successCount++
			} else {
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
					Headers:    m.Headers,
					Strategy:   b.dataBus.FindRepeatStrategy(c.GetTopic(), c.GetGroup(), c.GetID()),
				}, result.Message, retryAfter(result.RetryAfterSec)); err != nil {
					b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "retry_enqueue_error", len(list))
					return fmt.Errorf("%w: %v", model.ErrHandler, err)
				}
				retryCount++
			}
		}
		b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "success", successCount)
		b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "retry", retryCount)
		if matchedCount == 0 && len(list) > 0 {
			// Ни один результат не относится к отправленному батчу: фаза "батч ↔ ответ"
			// разошлась, дальше молча терялись бы сообщения. Закрываем стрим.
			b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "desynchronized", len(list))
			return fmt.Errorf("%w: no result matched batch, have [%s]", model.ErrHandler, strings.Join(list.GetIdList(), ", "))
		}
		if missingCount := len(list) - matchedCount; missingCount > 0 {
			b.metrics.ObserveConsumed(string(c.GetTopic()), string(c.GetGroup()), "missing_result", missingCount)
		}
		return nil
	}

	repeatStrategy := fromPBRepeatStrategy(data.Connect.RepeatStrategy)
	consumeErr := b.dataBus.Consume(ctx, c, server, repeatStrategy, limits, abort, handler, cancel)
	if consumeErr == nil {
		select {
		case abortErr := <-abortErrCh:
			consumeErr = abortErr
		default:
		}
	}
	return consumeErr
}

func retryAfter(seconds int32) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
