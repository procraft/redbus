package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/procraft/redbus/api/golang/pb"
)

const (
	// Channel is the pg_notify channel of the redbus_outbox trigger.
	Channel = "redbus_outbox"
	// DefaultBatchSize is the maximum number of rows fetched and published in one request.
	DefaultBatchSize = 100
	// DefaultSweepInterval is the interval of the periodic sweep.
	DefaultSweepInterval = 30 * time.Second
	// DefaultPublishTimeout bounds one batch request.
	DefaultPublishTimeout = 30 * time.Second
	// completeTimeout bounds the delete after a successful publish.
	completeTimeout = 10 * time.Second
)

// Publisher sends one batch with a single confirmed request, e.g. (*producer.Producer).ProduceBatch.
type Publisher func(ctx context.Context, req *pb.ProduceBatchRequest) error

type FlusherOption func(f *Flusher)

// WithBatchSize sets the maximum rows per query and batch request (default 100).
func WithBatchSize(n int) FlusherOption {
	return func(f *Flusher) { f.batchSize = n }
}

// WithSweepInterval sets the interval of the periodic sweep (default 30 s).
func WithSweepInterval(d time.Duration) FlusherOption {
	return func(f *Flusher) { f.sweepInterval = d }
}

// WithPublishTimeout bounds one batch request (default 30 s).
func WithPublishTimeout(d time.Duration) FlusherOption {
	return func(f *Flusher) { f.publishTimeout = d }
}

// WithListenDSN makes the flusher react to pg_notify('redbus_outbox') from the table trigger, using
// a dedicated lib/pq listener connection to this DSN. Without it only the periodic sweep delivers
// rows.
func WithListenDSN(dsn string) FlusherOption {
	return func(f *Flusher) { f.listenDSN = dsn }
}

// WithLogger sets the logger (default slog.Default()).
func WithLogger(l *slog.Logger) FlusherOption {
	return func(f *Flusher) { f.log = l }
}

// Flusher drains redbus_outbox into the bus.
//
// A pass is triggered by a notification (WithListenDSN) or by the periodic sweep, which also runs
// immediately at start and delivers rows left over from a restart or a missed notification. Only
// one pass runs at a time; a trigger during a pass starts another pass right after it. A pass
// fetches at most batchSize rows in id order, publishes the same-topic prefix with one request and
// deletes those rows, until the table is empty. A publish failure keeps the whole batch, ends the
// pass and is retried on the next trigger.
//
// Rows are selected FOR UPDATE and deleted in the same transaction, so flushers of several
// replicas of one service are serialised instead of publishing the same rows twice.
type Flusher struct {
	store          store
	publish        Publisher
	batchSize      int
	sweepInterval  time.Duration
	publishTimeout time.Duration
	listenDSN      string
	log            *slog.Logger
}

func NewFlusher(db *sql.DB, publish Publisher, opts ...FlusherOption) *Flusher {
	return newFlusher(sqlStore{db: db}, publish, opts...)
}

func newFlusher(s store, publish Publisher, opts ...FlusherOption) *Flusher {
	f := &Flusher{
		store:          s,
		publish:        publish,
		batchSize:      DefaultBatchSize,
		sweepInterval:  DefaultSweepInterval,
		publishTimeout: DefaultPublishTimeout,
		log:            slog.Default(),
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

// Run delivers rows until ctx is cancelled. It returns nil after cancellation, or an error for an
// invalid configuration.
func (f *Flusher) Run(ctx context.Context) error {
	if f.batchSize <= 0 {
		return fmt.Errorf("redbus outbox: batch size must be positive, got %d", f.batchSize)
	}
	if f.sweepInterval <= 0 {
		return fmt.Errorf("redbus outbox: sweep interval must be positive, got %v", f.sweepInterval)
	}

	// Buffer of one: a trigger arriving during a pass is remembered, further ones coalesce.
	trigger := make(chan struct{}, 1)
	notify := func() {
		select {
		case trigger <- struct{}{}:
		default:
		}
	}

	var notifications <-chan *pq.Notification
	if f.listenDSN != "" {
		l := pq.NewListener(f.listenDSN, time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
			if err != nil {
				f.log.Warn("redbus outbox: listener connection", "error", err)
			}
		})
		defer l.Close()
		// Listen blocks while the database is unreachable; sweeps go on meanwhile.
		go func() {
			if err := l.Listen(Channel); err != nil && ctx.Err() == nil {
				f.log.Error("redbus outbox: listen failed, only the periodic sweep delivers rows", "error", err)
			}
		}()
		notifications = l.NotificationChannel()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
				if err := f.Flush(ctx); err != nil && ctx.Err() == nil {
					f.log.Warn("redbus outbox: flush failed, rows stay in outbox until the next pass", "error", err)
				}
			}
		}
	}()

	notify()
	ticker := time.NewTicker(f.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			<-done
			return nil
		case <-ticker.C:
			notify()
		case <-notifications:
			// A nil notification follows a listener reconnect: notifications may have been lost.
			notify()
		}
	}
}

// Flush runs one pass: it publishes bounded batches until the outbox is empty or a batch fails.
func (f *Flusher) Flush(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := f.flushBatch(ctx)
		if err != nil || n == 0 {
			return err
		}
	}
}

// flushBatch publishes and deletes one same-topic batch and returns its size.
//
// The transaction is not bound to ctx: a cancellation between a successful publish and the commit
// would roll back the delete and publish the batch again on the next start. Each step has its own
// bound instead of one deadline for the whole transaction, so a long wait for another replica's
// row locks cannot expire the transaction after a successful publish: the fetch waits at most as
// long as another replica holds its locks, the publish has publishTimeout and the delete
// completeTimeout. The fetch and the publish also stop with ctx.
func (f *Flusher) flushBatch(ctx context.Context) (n int, err error) {
	tx, err := f.store.begin(context.WithoutCancel(ctx))
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.rollback()
		}
	}()

	fetchCtx, cancelFetch := context.WithTimeout(ctx, f.publishTimeout+completeTimeout)
	rows, err := tx.fetch(fetchCtx, f.batchSize)
	cancelFetch()
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, tx.commit()
	}
	topic := rows[0].topic
	req := &pb.ProduceBatchRequest{Topic: topic}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.topic != topic {
			break
		}
		msg := &pb.ProduceBatchMessage{
			Key:            r.options.Key,
			Message:        r.message,
			IdempotencyKey: r.options.IdempotencyKey,
			Timestamp:      r.options.Timestamp,
			Version:        r.options.Version,
		}
		if msg.Version == 0 {
			msg.Version = r.id
		}
		req.MessageList = append(req.MessageList, msg)
		ids = append(ids, r.id)
	}

	publishCtx, cancel := context.WithTimeout(ctx, f.publishTimeout)
	err = f.publish(publishCtx, req)
	cancel()
	if err != nil {
		return 0, fmt.Errorf("publish batch %s / %s: %w", topic, joinIDs(ids), err)
	}
	deleteCtx, cancelDelete := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
	deleted, err := tx.delete(deleteCtx, ids)
	cancelDelete()
	if err != nil {
		return 0, err
	}
	if deleted != len(ids) {
		return 0, fmt.Errorf("deleted %d of %d flushed outbox rows", deleted, len(ids))
	}
	if err = tx.commit(); err != nil {
		return 0, err
	}
	f.log.Debug("redbus outbox: flushed batch", "topic", topic, "ids", joinIDs(ids))
	return len(ids), nil
}

func joinIDs(ids []int64) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(s, ",")
}

type row struct {
	id      int64
	topic   string
	message []byte
	options options
}

// store is the outbox storage seam; rows are returned in id order and stay locked until the
// transaction ends.
type store interface {
	begin(ctx context.Context) (storeTx, error)
}

type storeTx interface {
	fetch(ctx context.Context, limit int) ([]row, error)
	delete(ctx context.Context, ids []int64) (int, error)
	commit() error
	rollback() error
}

type sqlStore struct {
	db *sql.DB
}

func (s sqlStore) begin(ctx context.Context) (storeTx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return sqlTx{tx: tx}, nil
}

type sqlTx struct {
	tx *sql.Tx
}

const fetchSQL = `SELECT id, topic, message, options FROM public.redbus_outbox ORDER BY id LIMIT $1 FOR UPDATE`

func (t sqlTx) fetch(ctx context.Context, limit int) ([]row, error) {
	rs, err := t.tx.QueryContext(ctx, fetchSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var ret []row
	for rs.Next() {
		var (
			r    row
			opts []byte
		)
		if err := rs.Scan(&r.id, &r.topic, &r.message, &opts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(opts, &r.options); err != nil {
			return nil, fmt.Errorf("outbox row %d: invalid options: %w", r.id, err)
		}
		ret = append(ret, r)
	}
	return ret, rs.Err()
}

// delete lists ids explicitly: an id range could cover a row committed after the fetch.
func (t sqlTx) delete(ctx context.Context, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}
	res, err := t.tx.ExecContext(ctx, `DELETE FROM public.redbus_outbox WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (t sqlTx) commit() error { return t.tx.Commit() }

func (t sqlTx) rollback() error {
	if err := t.tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return err
	}
	return nil
}
