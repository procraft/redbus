package consumer

import (
	"github.com/sergiusd/redbus/api/golang/pb"
	"log"
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

func toPBResultList(resultList []ProcessResult) []*pb.ConsumeRequest_Result {
	ret := make([]*pb.ConsumeRequest_Result, 0, len(resultList))
	for _, v := range resultList {
		if v.err == nil {
			log.Printf("[%v] Process payload success\n", v.id)
			ret = append(ret, &pb.ConsumeRequest_Result{Id: v.id, Ok: true})
		} else {
			log.Printf("[%v] Process payload error: %v\n", v.id, v.err)
			ret = append(ret, &pb.ConsumeRequest_Result{Id: v.id, Ok: false, Message: v.err.Error()})
		}
	}
	return ret
}
