package producer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/procraft/redbus/api/golang/pb"
)

// ErrRejected is returned when the bus answered a produce request with ok = false.
var ErrRejected = errors.New("redbus: bus rejected the message")

type Producer struct {
	conn            *grpc.ClientConn
	client          pb.RedbusServiceClient
	maxMessageBytes int
}

// ProducerOptionFn configures a Producer in New.
type ProducerOptionFn = func(p *Producer)

// WithMaxMessageBytes sets the payload limit of Produce; zero or less means DefaultMaxMessageBytes.
func WithMaxMessageBytes(limit int) ProducerOptionFn {
	return func(p *Producer) { p.maxMessageBytes = limit }
}

func New(host string, port int, opts ...ProducerOptionFn) (*Producer, error) {
	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", host, port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("Can not connect with databus %w", err)
	}
	p := &Producer{
		conn:   conn,
		client: pb.NewRedbusServiceClient(conn),
	}
	for _, o := range opts {
		o(p)
	}
	return p, nil
}

// Produce publishes one message directly over gRPC. It fails with ErrRejected when the bus did
// not accept the message, and with *MessageTooLargeError (errors.Is ErrMessageTooLarge) without
// calling the bus when the payload is above the producer's limit (DefaultMaxMessageBytes unless
// WithMaxMessageBytes).
func (p *Producer) Produce(ctx context.Context, topic string, message []byte, options ...OptionFn) error {
	if err := CheckMessageSize(topic, message, p.maxMessageBytes); err != nil {
		return err
	}
	resp, err := p.client.Produce(ctx, NewRequest(topic, message, options...))
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return ErrRejected
	}
	return nil
}

// ProduceBatch publishes messages of one topic with one confirmed request; either the whole batch
// is accepted or the call fails. A failure is ambiguous (Kafka may have written a part), so the
// consumer must stay idempotent when the caller retries.
func (p *Producer) ProduceBatch(ctx context.Context, req *pb.ProduceBatchRequest) error {
	resp, err := p.client.ProduceBatch(ctx, req)
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return ErrRejected
	}
	return nil
}

// Close releases the gRPC connection.
func (p *Producer) Close() error {
	return p.conn.Close()
}

// NewRequest builds the request Produce sends: a random idempotency key and the current timestamp,
// then the options in order.
func NewRequest(topic string, message []byte, options ...OptionFn) *pb.ProduceRequest {
	req := &pb.ProduceRequest{
		Topic:          topic,
		Message:        message,
		IdempotencyKey: uuid.NewString(),
		Timestamp:      formatTimestamp(time.Now()),
	}
	for _, o := range options {
		o(req)
	}
	return req
}

func formatTimestamp(t time.Time) string {
	return t.Format(time.RFC3339)
}
