package producer

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/procraft/redbus/api/golang/pb"
)

type fakeBus struct {
	pb.RedbusServiceClient
	produced [][]byte
}

func (f *fakeBus) Produce(_ context.Context, in *pb.ProduceRequest, _ ...grpc.CallOption) (*pb.ProduceResponse, error) {
	f.produced = append(f.produced, in.GetMessage())
	return &pb.ProduceResponse{Ok: true}, nil
}

func TestDefaultMaxMessageBytesIs256KiB(t *testing.T) {
	require.Equal(t, 262144, DefaultMaxMessageBytes)
}

func TestCheckMessageSizeBoundary(t *testing.T) {
	require.NoError(t, CheckMessageSize("t", make([]byte, 10), 10))

	err := CheckMessageSize("t", make([]byte, 11), 10)
	require.ErrorIs(t, err, ErrMessageTooLarge)
	var tooLarge *MessageTooLargeError
	require.True(t, errors.As(err, &tooLarge))
	require.Equal(t, MessageTooLargeError{Topic: "t", Size: 11, Limit: 10}, *tooLarge)
	require.Contains(t, err.Error(), "topic t is 11 bytes, above the limit of 10 bytes")

	// Zero means the default limit.
	require.NoError(t, CheckMessageSize("t", make([]byte, DefaultMaxMessageBytes), 0))
	require.ErrorIs(t, CheckMessageSize("t", make([]byte, DefaultMaxMessageBytes+1), 0), ErrMessageTooLarge)
}

func TestProduceRejectsTooLargeWithoutCallingTheBus(t *testing.T) {
	bus := &fakeBus{}
	p := &Producer{client: bus}
	WithMaxMessageBytes(10)(p)
	ctx := context.Background()

	require.NoError(t, p.Produce(ctx, "t", bytes.Repeat([]byte{1}, 10)))
	require.ErrorIs(t, p.Produce(ctx, "t", bytes.Repeat([]byte{1}, 11)), ErrMessageTooLarge)
	require.Len(t, bus.produced, 1)
	require.Len(t, bus.produced[0], 10)
}

func TestProduceAppliesDefaultLimit(t *testing.T) {
	bus := &fakeBus{}
	p := &Producer{client: bus}
	ctx := context.Background()

	require.NoError(t, p.Produce(ctx, "t", make([]byte, DefaultMaxMessageBytes)))
	require.ErrorIs(t, p.Produce(ctx, "t", make([]byte, DefaultMaxMessageBytes+1)), ErrMessageTooLarge)
	require.Len(t, bus.produced, 1)
}
