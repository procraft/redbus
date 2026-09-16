package producer

import (
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"

	"github.com/prokraft/redbus/internal/app/model"
)

type writerStub struct {
	calls   int
	batches [][]kafka.Message
	err     error
	write   func(context.Context, []kafka.Message) error
}

func (w *writerStub) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	w.calls++
	w.batches = append(w.batches, append([]kafka.Message(nil), messages...))
	if w.write != nil {
		return w.write(ctx, messages)
	}
	return w.err
}

func (w *writerStub) Close() error { return nil }

func TestProduceBatchRejectsEmptyBatchBeforeKafka(t *testing.T) {
	writer := &writerStub{}
	producer := &Producer{writer: writer, topic: "send-email-report"}

	err := producer.ProduceBatch(context.Background(), nil)

	require.EqualError(t, err, `produce batch for topic "send-email-report" is empty`)
	require.Zero(t, writer.calls)
}

func TestProduceBatchPropagatesAmbiguousKafkaErrorWithoutRetrying(t *testing.T) {
	writeErr := kafka.WriteErrors{nil, errors.New("partition unavailable")}
	writer := &writerStub{err: writeErr}
	producer := &Producer{writer: writer, topic: "send-email-report"}

	err := producer.ProduceBatch(context.Background(), []model.ProduceMessage{
		{Key: "first", Message: []byte("payload-1")},
		{Key: "second", Message: []byte("payload-2")},
	})

	var propagated kafka.WriteErrors
	require.Error(t, err)
	require.ErrorAs(t, err, &propagated)
	require.Equal(t, 1, propagated.Count())
	require.Equal(t, 1, writer.calls)
}

func TestProduceBatchPassesContextCancellationWithoutRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := &writerStub{write: func(ctx context.Context, _ []kafka.Message) error {
		return ctx.Err()
	}}
	producer := &Producer{writer: writer, topic: "send-email-report"}

	err := producer.ProduceBatch(ctx, []model.ProduceMessage{{Message: []byte("payload")}})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, writer.calls)
}

func TestProduceBatchWritesOneOrderedKafkaBatch(t *testing.T) {
	writer := &writerStub{}
	producer := &Producer{writer: writer, topic: "send-email-report"}

	err := producer.ProduceBatch(context.Background(), []model.ProduceMessage{
		{Key: "first", Message: []byte("payload-1")},
		{Key: "second", Message: []byte("payload-2")},
		{Key: "third", Message: []byte("payload-3")},
	})

	require.NoError(t, err)
	require.Equal(t, 1, writer.calls)
	require.Len(t, writer.batches, 1)
	require.Equal(t, []string{"first", "second", "third"}, []string{
		string(writer.batches[0][0].Key),
		string(writer.batches[0][1].Key),
		string(writer.batches[0][2].Key),
	})
	require.Equal(t, []string{"payload-1", "payload-2", "payload-3"}, []string{
		string(writer.batches[0][0].Value),
		string(writer.batches[0][1].Value),
		string(writer.batches[0][2].Value),
	})
}
