package stream

import (
	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
)

func toPBMessageList(list model.MessageList) []*pb.ConsumeResponse_Message {
	ret := make([]*pb.ConsumeResponse_Message, 0, len(list))
	for _, v := range list {
		ret = append(ret, &pb.ConsumeResponse_Message{Id: v.Id, Data: v.Value})
	}
	return ret
}
