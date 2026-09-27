package outbox

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prokraft/redbus/api/golang/pb"
)

// memStore keeps rows in id order; a transaction sees the committed rows and applies deletes on
// commit.
type memStore struct {
	rows      []row
	commits   int
	rollbacks int
	deleteErr error
	shortBy   int
	onDelete  func(ctx context.Context)
}

func (s *memStore) begin(context.Context) (storeTx, error) { return &memTx{s: s}, nil }

type memTx struct {
	s       *memStore
	deleted map[int64]bool
}

func (t *memTx) fetch(_ context.Context, limit int) ([]row, error) {
	if limit > len(t.s.rows) {
		limit = len(t.s.rows)
	}
	return append([]row(nil), t.s.rows[:limit]...), nil
}

func (t *memTx) delete(ctx context.Context, ids []int64) (int, error) {
	if t.s.onDelete != nil {
		t.s.onDelete(ctx)
	}
	if t.s.deleteErr != nil {
		return 0, t.s.deleteErr
	}
	t.deleted = map[int64]bool{}
	for _, id := range ids {
		t.deleted[id] = true
	}
	return len(ids) - t.s.shortBy, nil
}

func (t *memTx) commit() error {
	t.s.commits++
	kept := t.s.rows[:0]
	for _, r := range t.s.rows {
		if !t.deleted[r.id] {
			kept = append(kept, r)
		}
	}
	t.s.rows = kept
	return nil
}

func (t *memTx) rollback() error {
	t.s.rollbacks++
	return nil
}

func rowOf(id int64, topic string, version int64) row {
	return row{id: id, topic: topic, message: []byte(topic), options: options{Key: "k", IdempotencyKey: "ik", Version: version}}
}

type recorder struct {
	requests []*pb.ProduceBatchRequest
	failOn   int // 1-based call number that fails; 0 never
}

func (r *recorder) publish(_ context.Context, req *pb.ProduceBatchRequest) error {
	r.requests = append(r.requests, req)
	if len(r.requests) == r.failOn {
		return errors.New("bus unavailable")
	}
	return nil
}

func TestFlushPublishesSameTopicPrefixesUntilEmpty(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 7), rowOf(3, "b", 0), rowOf(4, "a", 0), rowOf(5, "a", 0)}}
	var rec recorder
	f := newFlusher(store, rec.publish, WithBatchSize(3))

	require.NoError(t, f.Flush(context.Background()))

	require.Empty(t, store.rows)
	require.Len(t, rec.requests, 3)
	require.Equal(t, "a", rec.requests[0].Topic)
	require.Len(t, rec.requests[0].MessageList, 2)
	// The outbox id stands in for a missing version.
	require.Equal(t, int64(1), rec.requests[0].MessageList[0].Version)
	require.Equal(t, int64(7), rec.requests[0].MessageList[1].Version)
	require.Equal(t, "ik", rec.requests[0].MessageList[0].IdempotencyKey)
	require.Equal(t, "k", rec.requests[0].MessageList[0].Key)
	require.Equal(t, "b", rec.requests[1].Topic)
	require.Len(t, rec.requests[1].MessageList, 1)
	require.Equal(t, "a", rec.requests[2].Topic)
	require.Len(t, rec.requests[2].MessageList, 2)
}

func TestFlushKeepsBatchOnPublishFailure(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "b", 0)}}
	rec := recorder{failOn: 1}
	f := newFlusher(store, rec.publish)

	require.ErrorContains(t, f.Flush(context.Background()), "bus unavailable")
	require.Len(t, store.rows, 2)
	require.Equal(t, 1, store.rollbacks)
	require.Len(t, rec.requests, 1)

	// The next pass delivers everything in order.
	require.NoError(t, f.Flush(context.Background()))
	require.Empty(t, store.rows)
	require.Equal(t, "a", rec.requests[1].Topic)
}

func TestFlushRollsBackOnDeleteMismatch(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 0)}, shortBy: 1}
	var rec recorder
	f := newFlusher(store, rec.publish)

	require.ErrorContains(t, f.Flush(context.Background()), "deleted 1 of 2")
	require.Len(t, store.rows, 2)
	require.Equal(t, 0, store.commits)
}

func TestFlushCommitsBatchPublishedBeforeCancellation(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 0)}}
	ctx, cancel := context.WithCancel(context.Background())
	var ctxErrAtDelete error
	store.onDelete = func(c context.Context) { ctxErrAtDelete = c.Err() }
	f := newFlusher(store, func(context.Context, *pb.ProduceBatchRequest) error {
		cancel() // shutdown right after the bus accepted the batch
		return nil
	})

	_ = f.Flush(ctx)

	require.NoError(t, ctxErrAtDelete, "delete must not run on the cancelled context")
	require.Empty(t, store.rows, "a published batch must be committed, not published again")
	require.Equal(t, 1, store.commits)
}

func TestRunValidatesConfiguration(t *testing.T) {
	var rec recorder
	require.Error(t, newFlusher(&memStore{}, rec.publish, WithBatchSize(0)).Run(context.Background()))
	require.Error(t, newFlusher(&memStore{}, rec.publish, WithSweepInterval(0)).Run(context.Background()))
}

func TestRunSweepsAtStart(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0)}}
	published := make(chan struct{}, 1)
	f := newFlusher(store, func(context.Context, *pb.ProduceBatchRequest) error {
		published <- struct{}{}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- f.Run(ctx) }()
	<-published
	cancel()
	require.NoError(t, <-done)
	require.Empty(t, store.rows)
}
