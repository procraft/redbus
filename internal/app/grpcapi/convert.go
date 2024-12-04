package grpcapi

import (
	"time"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
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

func toPBMessageList(list model.MessageList) []*pb.ConsumeResponse_Message {
	ret := make([]*pb.ConsumeResponse_Message, 0, len(list))
	for _, v := range list {
		ret = append(ret, &pb.ConsumeResponse_Message{Id: v.Id, Data: v.Value})
	}
	return ret
}
