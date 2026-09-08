package stream

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/prokraft/redbus/api/golang/pb"
	"github.com/prokraft/redbus/internal/app/model"
)

type recvItem struct {
	req *pb.ConsumeRequest
	err error
}

// serverStub подменяет pb.RedbusService_ConsumeServer: отправленные ответы складывает в sent,
// а Recv отдаёт заранее подготовленные запросы. Пустой recv блокирует Recv до closeCh, как
// делает настоящий gRPC-стрим, который просыпается только при завершении RPC.
type serverStub struct {
	grpc.ServerStream
	mu      sync.Mutex
	sent    []*pb.ConsumeResponse
	sentIDs chan string
	recv    chan recvItem
	closeCh chan struct{}
}

func newServerStub() *serverStub {
	return &serverStub{
		sentIDs: make(chan string, 8),
		recv:    make(chan recvItem, 8),
		closeCh: make(chan struct{}),
	}
}

func (s *serverStub) Send(response *pb.ConsumeResponse) error {
	s.mu.Lock()
	s.sent = append(s.sent, response)
	s.mu.Unlock()
	s.sentIDs <- response.BatchId
	return nil
}

// lastBatchID возвращает идентификатор последнего отправленного батча.
func (s *serverStub) lastBatchID(t *testing.T) string {
	t.Helper()
	select {
	case id := <-s.sentIDs:
		return id
	case <-time.After(time.Second):
		t.Fatal("батч не был отправлен")
		return ""
	}
}

func (s *serverStub) Recv() (*pb.ConsumeRequest, error) {
	select {
	case item := <-s.recv:
		return item.req, item.err
	case <-s.closeCh:
		return nil, errors.New("rpc closed")
	}
}

type consumerStub struct {
	model.IConsumer
}

func (c *consumerStub) GetHosts() []string        { return []string{"kafka:9092"} }
func (c *consumerStub) GetTopic() model.TopicName { return "orders" }
func (c *consumerStub) GetGroup() model.GroupName { return "billing" }
func (c *consumerStub) GetID() model.ConsumerId   { return "worker-1" }
func (c *consumerStub) Lock()                     {}
func (c *consumerStub) Unlock()                   {}

func batch() model.MessageList {
	return model.MessageList{{Id: "1/2008", Value: []byte("payload")}}
}

func TestProcessMessageListSendsBatchIdAndAcceptsMatchingResult(t *testing.T) {
	server := newServerStub()
	s := New(server, WithAbort(func(error) {}), WithLimits(model.ConsumeLimits{PerMessage: time.Minute, Slack: time.Second, Max: time.Minute}))

	var batchID string
	go func() {
		// batchId известен только после отправки батча.
		batchID = server.lastBatchID(t)
		server.recv <- recvItem{req: &pb.ConsumeRequest{
			BatchId:    batchID,
			ResultList: []*pb.ConsumeRequest_Result{{Id: "1/2008", Ok: true}},
		}}
	}()

	data, err := s.ProcessMessageList(context.Background(), &consumerStub{}, batch())

	require.NoError(t, err)
	require.NotNil(t, data)
	require.Len(t, data.ResultList, 1)
	require.NotEmpty(t, batchID, "батч должен уходить с идентификатором")
}

func TestProcessMessageListAcceptsLegacyResultWithoutBatchId(t *testing.T) {
	server := newServerStub()
	// Клиент старой сборки не знает про batchId и присылает пустое значение.
	server.recv <- recvItem{req: &pb.ConsumeRequest{
		ResultList: []*pb.ConsumeRequest_Result{{Id: "1/2008", Ok: true}},
	}}
	s := New(server, WithAbort(func(error) {}))

	data, err := s.ProcessMessageList(context.Background(), &consumerStub{}, batch())

	require.NoError(t, err)
	require.NotNil(t, data)
	require.Len(t, data.ResultList, 1)
}

func TestProcessMessageListDiscardsForeignBatchResult(t *testing.T) {
	server := newServerStub()
	s := New(server, WithAbort(func(error) {}))

	go func() {
		batchID := server.lastBatchID(t)
		// Поздний ответ на давно закрытый батч не должен сдвигать фазу "батч ↔ ответ".
		server.recv <- recvItem{req: &pb.ConsumeRequest{
			BatchId:    "b-stale",
			ResultList: []*pb.ConsumeRequest_Result{{Id: "0/1", Ok: true}},
		}}
		server.recv <- recvItem{req: &pb.ConsumeRequest{
			BatchId:    batchID,
			ResultList: []*pb.ConsumeRequest_Result{{Id: "1/2008", Ok: false, Message: "boom"}},
		}}
	}()

	data, err := s.ProcessMessageList(context.Background(), &consumerStub{}, batch())

	require.NoError(t, err)
	require.NotNil(t, data)
	require.Len(t, data.ResultList, 1)
	require.Equal(t, "1/2008", data.ResultList[0].Id)
	require.False(t, data.ResultList[0].Ok)
}

func TestProcessMessageListClosesStreamWhenResultIsNeverSent(t *testing.T) {
	server := newServerStub()
	aborted := make(chan error, 1)
	abort := func(err error) {
		aborted <- err
		// Настоящий Recv просыпается, когда RPC завершается после возврата из хендлера.
		close(server.closeCh)
	}
	s := New(server, WithAbort(abort), WithLimits(model.ConsumeLimits{
		PerMessage: 20 * time.Millisecond,
		Max:        time.Second,
	}))

	data, err := s.ProcessMessageList(context.Background(), &consumerStub{}, batch())

	require.Nil(t, data)
	require.ErrorIs(t, err, model.ErrStream)
	require.Contains(t, err.Error(), "no result")
	select {
	case abortErr := <-aborted:
		require.ErrorIs(t, abortErr, model.ErrStream)
		require.Contains(t, abortErr.Error(), "no result", "причина закрытия уходит клиенту статусом")
	default:
		t.Fatal("стрим должен быть закрыт по истечении бюджета ожидания результата")
	}
}

func TestProcessMessageListReturnsNilWhenClientClosesStream(t *testing.T) {
	server := newServerStub()
	server.recv <- recvItem{err: io.EOF}
	s := New(server, WithAbort(func(error) {}))

	data, err := s.ProcessMessageList(context.Background(), &consumerStub{}, batch())

	require.NoError(t, err)
	require.Nil(t, data, "закрытие стрима клиентом не должно приводить к панике на nil-ответе")
}
