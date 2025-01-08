package stream

import (
	"context"
	"fmt"
	"io"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/logger"
)

type Stream struct {
	server pb.RedbusService_ConsumeServer
}

func New(server pb.RedbusService_ConsumeServer) *Stream {
	return &Stream{server: server}
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

func (s Stream) ProcessMessageList(ctx context.Context, c model.IConsumer, list model.MessageList) (*pb.ConsumeRequest, error) {
	// обработка должна быть последовательно, чтобы запрос / ответ работали правильно
	c.Lock()
	defer c.Unlock()
	ok, err := s.Send(ctx, c, &pb.ConsumeResponse{MessageList: toPBMessageList(list)})
	if !ok {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrStream, err)
	}
	logger.Consumer(ctx, c, "Wait %d message processing", len(list))
	ok, data, err := s.Recv(ctx, c)
	if err == nil {
		for _, v := range data.ResultList {
			if v.Ok {
				logger.Consumer(ctx, c, "[%v] Message processing success", v.Id)
			} else {
				logger.Consumer(ctx, c, "[%v] Message processing error: %v", v.Id, v.Message)
			}
		}
	}
	if !ok {
		return nil, nil
	}
	return data, err
}
