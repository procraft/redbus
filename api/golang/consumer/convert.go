package consumer

import (
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
)

func toPBRepeatStrategy(strategy *RepeatStrategy) *pb.ConsumeRequest_Connect_RepeatStrategy {
	if strategy == nil {
		return nil
	}

	if strategy.evenStrategy != nil {
		return &pb.ConsumeRequest_Connect_RepeatStrategy{
			MaxAttempts: int32(strategy.maxAttempts),
			EvenConfig: &pb.ConsumeRequest_Connect_RepeatStrategy_EvenConfig{
				IntervalSec: int32(strategy.evenStrategy.intervalSec),
			},
		}
	}

	if strategy.progressiveStrategy != nil {
		return &pb.ConsumeRequest_Connect_RepeatStrategy{
			MaxAttempts: int32(strategy.maxAttempts),
			ProgressiveConfig: &pb.ConsumeRequest_Connect_RepeatStrategy_ProgressiveConfig{
				IntervalSec: int32(strategy.progressiveStrategy.intervalSec),
				Multiplier:  strategy.progressiveStrategy.multiplier,
			},
		}
	}

	return nil
}

func fromPBMessageIds(messageList []*pb.ConsumeResponse_Message) []string {
	ret := make([]string, 0, len(messageList))
	for _, v := range messageList {
		ret = append(ret, v.Id)
	}
	return ret
}

func fromPBMessage(m *pb.ConsumeResponse_Message, log *slog.Logger) Message {
	msg := Message{
		ID:             m.Id,
		Data:           m.Data,
		IdempotencyKey: m.IdempotencyKey,
		Version:        m.Version,
	}
	if m.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339, m.Timestamp)
		if err != nil {
			log.Warn("redbus: unparsable message timestamp", "id", m.Id, "timestamp", m.Timestamp, "error", err)
		} else {
			msg.Timestamp = ts
		}
	}
	return msg
}

func toPBResultList(resultList []ProcessResult, log *slog.Logger) []*pb.ConsumeRequest_Result {
	ret := make([]*pb.ConsumeRequest_Result, 0, len(resultList))
	for _, v := range resultList {
		if v.err == nil {
			log.Debug("redbus: process payload success", "id", v.id)
			ret = append(ret, &pb.ConsumeRequest_Result{Id: v.id, Ok: true})
		} else {
			log.Warn("redbus: process payload error", "id", v.id, "error", v.err)
			result := &pb.ConsumeRequest_Result{Id: v.id, Ok: false, Message: v.err.Error()}
			var retryLater *RetryLaterError
			if errors.As(v.err, &retryLater) {
				result.PreserveAttempt = true
				result.RetryAfterSec = durationSeconds(retryLater.Delay)
			}
			ret = append(ret, result)
		}
	}
	return ret
}

func durationSeconds(delay time.Duration) int32 {
	if delay <= 0 {
		return 0
	}
	seconds := math.Ceil(delay.Seconds())
	if seconds > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(seconds)
}
