package stream

import (
	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"strconv"
)

func toPBMessageList(list model.MessageList) []*pb.ConsumeResponse_Message {
	ret := make([]*pb.ConsumeResponse_Message, 0, len(list))
	for _, v := range list {
		item := pb.ConsumeResponse_Message{
			Id:   v.Id,
			Data: v.Value,
		}
		if val, ok := v.Headers[model.Version]; ok {
			valInt, _ := strconv.ParseInt(val, 10, 64)
			if valInt != 0 {
				item.Version = valInt
			}
		}
		if val, ok := v.Headers[model.IdempotencyKeyHeader]; ok {
			item.IdempotencyKey = val
		}
		if val, ok := v.Headers[model.TimestampHeader]; ok {
			item.Timestamp = val
		}
		ret = append(ret, &item)
	}
	return ret
}
