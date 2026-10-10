package redbus

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/procraft/redbus/api/golang/consumer"
	"github.com/procraft/redbus/api/golang/inbox"
	"github.com/procraft/redbus/api/golang/outbox"
	"github.com/procraft/redbus/api/golang/pb"
	"github.com/procraft/redbus/api/golang/producer"
)

type fakeTransport struct {
	produced     [][]byte
	produceErr   error
	consumeOpts  int
	delivered    []consumer.Message
	handlerErrs  []error
	flusherRuns  atomic.Int32
	consumeCalls int
}

func (f *fakeTransport) produce(_ context.Context, _ string, message []byte, _ ...producer.OptionFn) error {
	f.produced = append(f.produced, message)
	return f.produceErr
}

func (f *fakeTransport) consume(ctx context.Context, _, _ string, handler consumer.Handler, opts ...consumer.OptionFn) error {
	f.consumeCalls++
	f.consumeOpts = len(opts)
	for _, m := range f.delivered {
		f.handlerErrs = append(f.handlerErrs, handler(ctx, m))
	}
	return nil
}

func (f *fakeTransport) runFlusher(ctx context.Context, _ *sql.DB, _ ...outbox.FlusherOption) error {
	f.flusherRuns.Add(1)
	<-ctx.Done()
	return nil
}

func (f *fakeTransport) close() error { return nil }

func newTestClient(settings Settings, bus *fakeTransport, db *sql.DB) *Client {
	c := &Client{settings: settings, db: db, log: slog.Default()}
	if settings.Enabled() {
		c.bus = bus
	}
	return c
}

func TestDisabledSidesAreNoOps(t *testing.T) {
	bus := &fakeTransport{}
	c := newTestClient(Settings{}, bus, nil)
	ctx := context.Background()

	sent, err := c.ProduceProto(ctx, "t", &pb.ProduceResponse{Ok: true})
	require.NoError(t, err)
	require.False(t, sent)
	require.NoError(t, c.ProduceTx(ctx, nil, "t", []byte("x")))
	require.NoError(t, c.StartFlusher(ctx))
	require.NoError(t, c.Consume(ctx, "t", "g", inbox.Transactional, nil))
	require.NoError(t, c.Close())

	// The producer side alone does not start consumers.
	c = newTestClient(Settings{Port: 1, ProducerEnabled: true}, bus, nil)
	require.NoError(t, c.Consume(ctx, "t", "g", inbox.Disabled, nil))
	require.Zero(t, bus.consumeCalls)
}

func TestProduceProtoEncodes(t *testing.T) {
	bus := &fakeTransport{}
	c := newTestClient(Settings{Port: 1, ProducerEnabled: true}, bus, nil)
	sent, err := c.ProduceProto(context.Background(), "t", &pb.ProduceResponse{Ok: true})
	require.NoError(t, err)
	require.True(t, sent)
	var decoded pb.ProduceResponse
	require.NoError(t, proto.Unmarshal(bus.produced[0], &decoded))
	require.True(t, decoded.Ok)

	bus.produceErr = producer.ErrRejected
	sent, err = c.Produce(context.Background(), "t", []byte("x"))
	require.ErrorIs(t, err, producer.ErrRejected)
	require.False(t, sent)
}

func TestConsumeProtoDecodesAndDropsInvalidPayload(t *testing.T) {
	valid, err := proto.Marshal(&pb.ProduceResponse{Ok: true})
	require.NoError(t, err)
	bus := &fakeTransport{delivered: []consumer.Message{
		{ID: "valid", Data: valid},
		{ID: "invalid", Data: []byte{0xff, 0xff, 0xff}},
		{ID: "failing", Data: valid},
	}}
	c := newTestClient(Settings{Port: 1, ConsumerEnabled: true}, bus, nil)

	var got []string
	err = ConsumeProto(context.Background(), c, "t", "g", inbox.Disabled,
		func(_ context.Context, m *pb.ProduceResponse, msg consumer.Message) error {
			require.True(t, m.Ok)
			got = append(got, msg.ID)
			if msg.ID == "failing" {
				return errors.New("failed")
			}
			return nil
		}, consumer.WithBatchSize(5))
	require.NoError(t, err)

	require.Equal(t, []string{"valid", "failing"}, got)
	require.NoError(t, bus.handlerErrs[0])
	require.NoError(t, bus.handlerErrs[1], "an undecodable payload is acknowledged")
	require.EqualError(t, bus.handlerErrs[2], "failed")
	require.Equal(t, 2, bus.consumeOpts, "the inbox option is prepended to the caller's options")
}

func TestInboxAndFlusherNeedDatabase(t *testing.T) {
	bus := &fakeTransport{}
	c := newTestClient(Settings{Port: 1, ProducerEnabled: true, ConsumerEnabled: true}, bus, nil)
	require.ErrorIs(t, c.Consume(context.Background(), "t", "g", inbox.OnlyOnce, nil), ErrNoDatabase)
	require.ErrorIs(t, c.StartFlusher(context.Background()), ErrNoDatabase)
}

func TestStartFlusherOnce(t *testing.T) {
	bus := &fakeTransport{}
	c := newTestClient(Settings{Port: 1, ProducerEnabled: true}, bus, &sql.DB{})
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, c.StartFlusher(ctx))
	require.NoError(t, c.StartFlusher(ctx))
	cancel()
	require.NoError(t, c.Close())
	require.Equal(t, int32(1), bus.flusherRuns.Load())
}

// A deferred Close runs before a deferred cancel of the application context: it must stop the
// flusher itself instead of waiting for that context forever.
func TestCloseStopsFlusherWithoutCancel(t *testing.T) {
	bus := &fakeTransport{}
	c := newTestClient(Settings{Port: 1, ProducerEnabled: true}, bus, &sql.DB{})
	require.NoError(t, c.StartFlusher(context.Background()))

	closed := make(chan error)
	go func() { closed <- c.Close() }()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close hangs while the flusher context is alive")
	}
}

func TestSettingsValidate(t *testing.T) {
	require.NoError(t, Settings{}.Validate())
	require.Error(t, Settings{ProducerEnabled: true}.Validate())
	require.Error(t, Settings{Port: 1, ConsumerEnabled: true, OutboxBatchSize: -1}.Validate())
	require.Equal(t, outbox.DefaultBatchSize, Settings{}.batchSize())
}

type recordingExecer struct{ execs int }

func (r *recordingExecer) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	r.execs++
	return nil, nil
}

func TestMaxMessageBytesFromSettings(t *testing.T) {
	bus := &fakeTransport{}
	c := newTestClient(Settings{Port: 1, ProducerEnabled: true, MaxMessageBytes: 10}, bus, nil)
	ctx := context.Background()

	sent, err := c.Produce(ctx, "t", make([]byte, 10))
	require.NoError(t, err)
	require.True(t, sent)
	sent, err = c.Produce(ctx, "t", make([]byte, 11))
	require.Equal(t, &producer.MessageTooLargeError{Topic: "t", Size: 11, Limit: 10}, err)
	require.False(t, sent)
	require.Len(t, bus.produced, 1)

	tx := &recordingExecer{}
	require.NoError(t, c.ProduceTx(ctx, tx, "t", make([]byte, 10)))
	require.ErrorIs(t, c.ProduceTx(ctx, tx, "t", make([]byte, 11)), producer.ErrMessageTooLarge)
	require.Equal(t, 1, tx.execs)
}

func TestProduceProtoChecksSerializedSize(t *testing.T) {
	bus := &fakeTransport{}
	// A ProduceRequest with only Topic set serializes to 2 + len(Topic) bytes.
	message := &pb.ProduceRequest{Topic: "12345678"}
	require.Equal(t, 10, proto.Size(message))
	c := newTestClient(Settings{Port: 1, ProducerEnabled: true, MaxMessageBytes: 10}, bus, nil)
	ctx := context.Background()

	_, err := c.ProduceProto(ctx, "t", message)
	require.NoError(t, err)
	tx := &recordingExecer{}
	require.NoError(t, c.ProduceProtoTx(ctx, tx, "t", message))

	c = newTestClient(Settings{Port: 1, ProducerEnabled: true, MaxMessageBytes: 9}, bus, nil)
	_, err = c.ProduceProto(ctx, "t", message)
	require.ErrorIs(t, err, producer.ErrMessageTooLarge)
	require.ErrorIs(t, c.ProduceProtoTx(ctx, tx, "t", message), producer.ErrMessageTooLarge)
	require.Len(t, bus.produced, 1)
	require.Equal(t, 1, tx.execs)
}

func TestMaxMessageBytesDefaultAndValidation(t *testing.T) {
	require.Equal(t, producer.DefaultMaxMessageBytes, Settings{}.maxMessageBytes())
	require.Equal(t, 1000, Settings{MaxMessageBytes: 1000}.maxMessageBytes())
	require.ErrorContains(t, Settings{MaxMessageBytes: -1}.Validate(), "maxMessageBytes must not be negative")
}
