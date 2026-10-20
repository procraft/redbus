package outbox

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/pb"
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
	fetches   [][]string // failed topics passed to every fetch
}

func (s *memStore) begin(context.Context) (storeTx, error) { return &memTx{s: s}, nil }

type memTx struct {
	s       *memStore
	deleted map[int64]bool
}

func (t *memTx) fetch(_ context.Context, limit int, failed []string) ([]row, error) {
	t.s.fetches = append(t.s.fetches, append([]string(nil), failed...))
	skipped := map[string]bool{}
	for _, topic := range failed {
		skipped[topic] = true
	}
	var ret []row
	for _, r := range t.s.rows {
		if !skipped[r.topic] && len(ret) < limit {
			ret = append(ret, r)
		}
	}
	return ret, nil
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
	requests  []*pb.ProduceBatchRequest
	failOn    int    // 1-based call number that fails; 0 never
	failTopic string // topic whose every request fails
}

func (r *recorder) publish(_ context.Context, req *pb.ProduceBatchRequest) error {
	r.requests = append(r.requests, req)
	if len(r.requests) == r.failOn || (r.failTopic != "" && req.Topic == r.failTopic) {
		return errors.New("bus unavailable")
	}
	return nil
}

func versions(req *pb.ProduceBatchRequest) []int64 {
	ret := make([]int64, len(req.MessageList))
	for i, m := range req.MessageList {
		ret[i] = m.Version
	}
	return ret
}

func ids(rows []row) []int64 {
	ret := make([]int64, len(rows))
	for i, r := range rows {
		ret[i] = r.id
	}
	return ret
}

func TestFlushPublishesEachTopicOfTheQueueHeadUntilEmpty(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 7), rowOf(3, "b", 0), rowOf(4, "a", 0), rowOf(5, "a", 0)}}
	var rec recorder
	f := newFlusher(store, rec.publish, WithBatchSize(3))

	require.NoError(t, f.Flush(context.Background()))

	require.Empty(t, store.rows)
	require.Len(t, rec.requests, 3)
	require.Equal(t, "a", rec.requests[0].Topic)
	// The outbox id stands in for a missing version.
	require.Equal(t, []int64{1, 7}, versions(rec.requests[0]))
	require.Equal(t, "ik", rec.requests[0].MessageList[0].IdempotencyKey)
	require.Equal(t, "k", rec.requests[0].MessageList[0].Key)
	require.Equal(t, "b", rec.requests[1].Topic)
	require.Equal(t, []int64{3}, versions(rec.requests[1]))
	require.Equal(t, "a", rec.requests[2].Topic)
	require.Equal(t, []int64{4, 5}, versions(rec.requests[2]))
	// The healthy path never excludes a topic.
	for _, failed := range store.fetches {
		require.Empty(t, failed)
	}
}

func TestFlushGroupsInterleavedTopicsIntoOneRequestEach(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "b", 0), rowOf(3, "a", 0), rowOf(4, "b", 0)}}
	var rec recorder
	f := newFlusher(store, rec.publish)

	require.NoError(t, f.Flush(context.Background()))

	require.Empty(t, store.rows)
	require.Len(t, rec.requests, 2)
	require.Equal(t, "a", rec.requests[0].Topic)
	require.Equal(t, []int64{1, 3}, versions(rec.requests[0]))
	require.Equal(t, "b", rec.requests[1].Topic)
	require.Equal(t, []int64{2, 4}, versions(rec.requests[1]))
}

func TestFlushSkipsFailedTopicAndDeliversOthers(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "b", 0), rowOf(3, "a", 0), rowOf(4, "c", 0), rowOf(5, "b", 0)}}
	rec := recorder{failTopic: "a"}
	f := newFlusher(store, rec.publish, WithBatchSize(10))

	failures, err := f.flush(context.Background())

	require.NoError(t, err)
	require.Len(t, failures, 1)
	require.Equal(t, "a", failures[0].topic)
	require.ErrorContains(t, failures[0].err, "bus unavailable")
	// Topic a was tried once, the rows behind it went out, a keeps its rows in id order.
	require.Len(t, rec.requests, 3)
	require.Equal(t, []string{"a", "b", "c"}, []string{rec.requests[0].Topic, rec.requests[1].Topic, rec.requests[2].Topic})
	require.Equal(t, []int64{2, 5}, versions(rec.requests[1]))
	require.Equal(t, []int64{1, 3}, ids(store.rows))
	require.Equal(t, [][]string{nil, {"a"}}, store.fetches, "the next selection excludes the failed topic")
	require.ErrorContains(t, f.Flush(context.Background()), "publish batch a / 1,3")
	require.Equal(t, []int64{1, 3}, ids(store.rows))
}

func TestFlushEndsWhenOnlyFailedTopicIsLeft(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 0)}}
	rec := recorder{failTopic: "a"}
	f := newFlusher(store, rec.publish, WithBatchSize(1))

	require.Error(t, f.Flush(context.Background()))
	require.Len(t, rec.requests, 1, "a failed topic is tried once per pass")
	require.Len(t, store.rows, 2)
}

func TestFlushKeepsBatchOnPublishFailure(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 0)}}
	rec := recorder{failOn: 1}
	f := newFlusher(store, rec.publish)

	require.ErrorContains(t, f.Flush(context.Background()), "bus unavailable")
	require.Len(t, store.rows, 2)
	require.Len(t, rec.requests, 1)

	// The next pass delivers everything in order.
	require.NoError(t, f.Flush(context.Background()))
	require.Empty(t, store.rows)
	require.Equal(t, "a", rec.requests[1].Topic)
}

func TestFlushKeepsBatchAndDeliversOtherTopicOnPublishFailure(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "b", 0)}}
	rec := recorder{failOn: 1}
	f := newFlusher(store, rec.publish)

	require.ErrorContains(t, f.Flush(context.Background()), "bus unavailable")
	require.Equal(t, []int64{1}, ids(store.rows))
	require.Zero(t, store.rollbacks, "a failed topic does not roll back the published one")
	require.Len(t, rec.requests, 2)
}

func TestFlushRollsBackOnDeleteMismatch(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "a", 0)}, shortBy: 1}
	var rec recorder
	f := newFlusher(store, rec.publish)

	require.ErrorContains(t, f.Flush(context.Background()), "deleted 1 of 2")
	require.Len(t, store.rows, 2)
	require.Equal(t, 1, store.rollbacks)
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
