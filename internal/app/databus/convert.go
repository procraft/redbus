package databus

import (
	"time"

	"github.com/sergiusd/redbus/api/golang/pb"
	"github.com/sergiusd/redbus/internal/app/model"
)

func fromPBStrategy(strategy *pb.ConsumeRequest_Connect_Strategy) *model.RepeatStrategy {
	if strategy == nil {
		return nil
	}

	if strategy.EvenConfig != nil {
		return model.NewEvenRepeatStrategy(
			int(strategy.MaxAttempts),
			time.Second*time.Duration(strategy.EvenConfig.IntervalSec),
		)
	}

	if strategy.ProgressiveConfig != nil {
		return model.NewProgressiveRepeatStrategy(
			int(strategy.MaxAttempts),
			time.Second*time.Duration(strategy.ProgressiveConfig.IntervalSec),
			strategy.ProgressiveConfig.Multiplier,
		)
	}

	return nil
}
