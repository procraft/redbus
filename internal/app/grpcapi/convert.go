package grpcapi

import (
	"time"

	"github.com/sergiusd/redbus/api/golang/pb"
	"github.com/sergiusd/redbus/internal/app/model"
)

func fromPBRepeatStrategy(strategy *pb.ConsumeRequest_Connect_RepeatStrategy) *model.RepeatStrategy {
	if strategy == nil {
		return nil
	}

	if strategy.EvenConfig != nil {
		return model.NewRepeatStrategyEven(
			int(strategy.MaxAttempts),
			time.Second*time.Duration(strategy.EvenConfig.IntervalSec),
		)
	}

	if strategy.ProgressiveConfig != nil {
		return model.NewRepeatStrategyStrategy(
			int(strategy.MaxAttempts),
			time.Second*time.Duration(strategy.ProgressiveConfig.IntervalSec),
			strategy.ProgressiveConfig.Multiplier,
		)
	}

	return nil
}
