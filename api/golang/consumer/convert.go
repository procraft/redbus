package consumer

import (
	"github.com/sergiusd/redbus/api/golang/pb"
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
