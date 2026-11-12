package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/procraft/redbus/api/golang/pb"
)

// fakeBus accepts one consumer, sends one batch and hands the result request to the test.
type fakeBus struct {
	pb.UnimplementedRedbusServiceServer
	batch   *pb.ConsumeResponse
	connect chan *pb.ConsumeRequest_Connect
	results chan *pb.ConsumeRequest
}

func (b *fakeBus) Consume(stream grpc.BidiStreamingServer[pb.ConsumeRequest, pb.ConsumeResponse]) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	b.connect <- req.Connect
	if err := stream.Send(&pb.ConsumeResponse{Connect: &pb.ConsumeResponse_Connect{Ok: true}}); err != nil {
		return err
	}
	if err := stream.Send(b.batch); err != nil {
		return err
	}
	res, err := stream.Recv()
	if err != nil {
		return err
	}
	b.results <- res
	<-stream.Context().Done()
	return nil
}

func startFakeBus(t *testing.T, bus *fakeBus) int {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	pb.RegisterRedbusServiceServer(srv, bus)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().(*net.TCPAddr).Port
}

func TestConsumeMessagesPassesMetadataAndReportsResults(t *testing.T) {
	bus := &fakeBus{
		batch: &pb.ConsumeResponse{BatchId: "batch-1", MessageList: []*pb.ConsumeResponse_Message{
			{Id: "m1", Data: []byte("ok"), LastAttempt: true, IdempotencyKey: "ik1", Version: 3, Timestamp: "2026-01-02T03:04:05+03:00"},
			{Id: "m2", Data: []byte("fail"), LastAttempt: true},
			{Id: "m3", Data: []byte("panic"), LastAttempt: true},
			{Id: "m4", Data: []byte("later"), LastAttempt: true},
			{Id: "m5", Data: []byte("fail")},
		}},
		connect: make(chan *pb.ConsumeRequest_Connect, 1),
		results: make(chan *pb.ConsumeRequest, 1),
	}
	port := startFakeBus(t, bus)

	seen := make(chan Message, 5)
	reported := make(chan string, 5)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		done <- New("127.0.0.1", port).ConsumeMessages(ctx, "topic", "group", func(_ context.Context, msg Message) error {
			seen <- msg
			switch string(msg.Data) {
			case "fail":
				return errors.New("failed")
			case "panic":
				panic("boom")
			case "later":
				return fmt.Errorf("wrapped: %w", NewRetryLaterError(errors.New("throttled"), 3*time.Second))
			}
			return nil
		}, WithBatchSize(5), WithConsumeTimeout(10*time.Second), WithRepeatStrategyEven(5, 30),
			WithLastAttemptFailure(func(_ context.Context, msg Message, err error) error {
				reported <- msg.ID + ": " + err.Error()
				return errors.New("callback failed")
			}))
	}()

	connect := <-bus.connect
	require.Equal(t, "topic", connect.Topic)
	require.Equal(t, "group", connect.Group)
	require.Equal(t, int32(5), connect.BatchSize)
	require.Equal(t, int32(10), connect.ConsumeTimeoutSec)
	require.Equal(t, int32(5), connect.RepeatStrategy.MaxAttempts)

	res := <-bus.results
	cancel()
	require.NoError(t, <-done)

	require.Equal(t, "batch-1", res.BatchId)
	byID := map[string]*pb.ConsumeRequest_Result{}
	for _, r := range res.ResultList {
		byID[r.Id] = r
	}
	require.Len(t, byID, 5)
	require.True(t, byID["m1"].Ok)
	require.False(t, byID["m2"].Ok)
	require.Equal(t, "failed", byID["m2"].Message)
	require.False(t, byID["m3"].Ok)
	require.Contains(t, byID["m3"].Message, "boom")
	require.False(t, byID["m4"].Ok)
	require.True(t, byID["m4"].RetryLater)
	require.Equal(t, int32(3), byID["m4"].RetryAfterSec)
	require.Equal(t, "failed", byID["m5"].Message)
	require.Len(t, reported, 1)
	require.Equal(t, "m2: failed", <-reported)

	close(seen)
	for msg := range seen {
		if msg.ID != "m1" {
			continue
		}
		require.Equal(t, "ik1", msg.IdempotencyKey)
		require.True(t, msg.LastAttempt)
		require.Equal(t, int64(3), msg.Version)
		require.True(t, msg.Timestamp.Equal(time.Date(2026, 1, 2, 0, 4, 5, 0, time.UTC)))
		require.Nil(t, msg.Claim)
	}
}

func TestLastAttemptCallbackDeadlineKeepsHandlerError(t *testing.T) {
	failure := errors.New("handler failed")
	callbackDone := make(chan struct{})
	s := session{
		listener: Listener{
			consumeTimeout: 100 * time.Millisecond,
			onLastAttemptFailure: func(ctx context.Context, _ Message, _ error) error {
				defer close(callbackDone)
				<-ctx.Done()
				return ctx.Err()
			},
		},
		handler: func(context.Context, Message) error { return failure },
		log:     slog.Default(),
	}
	require.ErrorIs(t, s.processMessage(context.Background(), &pb.ConsumeResponse_Message{LastAttempt: true}), failure)
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("callback did not finish after deadline")
	}
}

func TestConsumeMessagesRequiresDatabaseForInbox(t *testing.T) {
	err := New("127.0.0.1", 1).ConsumeMessages(context.Background(), "t", "g",
		func(context.Context, Message) error { return nil }, WithInbox(nil, 1))
	require.ErrorContains(t, err, "needs a database")
}
