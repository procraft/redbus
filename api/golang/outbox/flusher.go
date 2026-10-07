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
	// DefaultErrorLogInterval is the minimum interval between two error reports for one topic.
	DefaultErrorLogInterval = time.Minute
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

// WithErrorLogInterval sets the minimum interval between two error reports for one topic
// (default one minute). Reports suppressed in between are counted in the next one.
func WithErrorLogInterval(d time.Duration) FlusherOption {
	return func(f *Flusher) { f.errorLogInterval = d }
}

// WithLogger sets the logger (default slog.Default()).
func WithLogger(l *slog.Logger) FlusherOption {
	return func(f *Flusher) { f.log = l }
}

// Flusher drains redbus_outbox into the bus.
//
// A pass is triggered by a notification (WithListenDSN) or by the periodic sweep, which also runs
// immediately at start and delivers rows left over from a restart or a missed notification. Only
// one pass runs at a time; a trigger during a pass starts another pass right after it.
//
// Order is kept only within a topic. Each step selects at most batchSize rows in id order, skipping
// topics that failed earlier in the pass, publishes the rows of each topic with one request (topics
// in the order of their first row) and deletes the published rows. A topic whose publish fails keeps
// its rows and is excluded for the rest of the pass, so one rejected topic (e.g. missing Kafka ACLs)
// does not stop the others; it is retried on the next trigger. Failures are logged at error level,
// at most once per topic per errorLogInterval.
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

	errorLogInterval time.Duration
	now              func() time.Time
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

		errorLogInterval: DefaultErrorLogInterval,
		now:              time.Now,
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

	errLog := newErrorThrottle(f.errorLogInterval, f.now)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
				failures, err := f.flush(ctx)
				if ctx.Err() != nil {
					continue
				}
				for _, fl := range failures {
					errLog.report(fl.topic, func(suppressed int) {
						f.log.Error("redbus outbox: flush of topic failed, its rows stay in outbox until the next pass",
							"topic", fl.topic, "error", fl.err, "suppressed", suppressed)
					})
				}
				if err != nil {
					errLog.report("", func(suppressed int) {
						f.log.Error("redbus outbox: flush failed, rows stay in outbox until the next pass",
							"error", err, "suppressed", suppressed)
					})
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

// Flush runs one pass: it publishes bounded batches until every row left belongs to a topic that
// failed in this pass. The returned error joins the failure of every such topic and an error that
// ended the pass early (fetch, delete, commit, cancellation).
func (f *Flusher) Flush(ctx context.Context) error {
	failures, err := f.flush(ctx)
	errs := make([]error, 0, len(failures)+1)
	for _, fl := range failures {
		errs = append(errs, fl.err)
	}
	return errors.Join(append(errs, err)...)
}

type topicFailure struct {
	topic string
	err   error
}

// flush runs one pass and returns the topics that failed in it; err is a failure that ended the
// pass. A step either deletes rows or adds every topic of its selection to failed, and failed rows
// are never selected again, so the pass always ends.
func (f *Flusher) flush(ctx context.Context) (failures []topicFailure, err error) {
	var failed []string
	for {
		if err := ctx.Err(); err != nil {
			return failures, err
		}
		stepFailures, n, err := f.flushBatch(ctx, failed)
		failures = append(failures, stepFailures...)
		if err != nil {
			return failures, err
		}
		for _, fl := range stepFailures {
			failed = append(failed, fl.topic)
		}
		if n == 0 && len(stepFailures) == 0 {
			return failures, nil
		}
	}
}

// topicGroup is the rows of one topic within a selection, in id order.
type topicGroup struct {
	topic string
	rows  []row
}

// groupByTopic splits rows (in id order) by topic in the order of each topic's first row.
func groupByTopic(rows []row) []topicGroup {
	var groups []topicGroup
	index := map[string]int{}
	for _, r := range rows {
		i, ok := index[r.topic]
		if !ok {
			i = len(groups)
			index[r.topic] = i
			groups = append(groups, topicGroup{topic: r.topic})
		}
		groups[i].rows = append(groups[i].rows, r)
	}
	return groups
}

// flushBatch runs one step in one transaction: select at most batchSize rows in id order outside
// the failed topics FOR UPDATE, publish each topic's rows with one request, delete the rows of the
// published topics and commit. It returns the topics whose publish failed (their rows stay) and the
// number of deleted rows; an error (fetch, delete, commit) rolls back the whole step and ends the
// pass.
//
// The transaction is not bound to ctx: a cancellation between a successful publish and the commit
// would roll back the delete and publish the batch again on the next start. Each step has its own
// bound instead of one deadline for the whole transaction, so a long wait for another replica's
// row locks cannot expire the transaction after a successful publish: the fetch waits at most as
// long as another replica can hold its locks (one publishTimeout per topic of its selection plus
// the delete), each publish has publishTimeout and the delete completeTimeout. The fetch and the
// publish also stop with ctx.
func (f *Flusher) flushBatch(ctx context.Context, failed []string) (failures []topicFailure, n int, err error) {
	tx, err := f.store.begin(context.WithoutCancel(ctx))
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.rollback()
		}
	}()

	fetchCtx, cancelFetch := context.WithTimeout(ctx, time.Duration(f.batchSize)*f.publishTimeout+completeTimeout)
	rows, err := tx.fetch(fetchCtx, f.batchSize, failed)
	cancelFetch()
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, tx.commit()
	}

	var published []int64
	for _, g := range groupByTopic(rows) {
		ids, perr := f.publishGroup(ctx, g)
		if perr != nil {
			failures = append(failures, topicFailure{topic: g.topic, err: perr})
			continue
		}
		published = append(published, ids...)
	}
	if len(published) == 0 {
		return failures, 0, tx.commit()
	}

	deleteCtx, cancelDelete := context.WithTimeout(context.WithoutCancel(ctx), completeTimeout)
	deleted, err := tx.delete(deleteCtx, published)
	cancelDelete()
	if err != nil {
		return failures, 0, err
	}
	if deleted != len(published) {
		err = fmt.Errorf("deleted %d of %d flushed outbox rows", deleted, len(published))
		return failures, 0, err
	}
	if err = tx.commit(); err != nil {
		return failures, 0, err
	}
	return failures, len(published), nil
}

// publishGroup sends the rows of one topic with one request and returns their ids.
func (f *Flusher) publishGroup(ctx context.Context, g topicGroup) ([]int64, error) {
	req := &pb.ProduceBatchRequest{Topic: g.topic}
	ids := make([]int64, 0, len(g.rows))
	for _, r := range g.rows {
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
	err := f.publish(publishCtx, req)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("publish batch %s / %s: %w", g.topic, joinIDs(ids), err)
	}
	f.log.Debug("redbus outbox: flushed batch", "topic", g.topic, "ids", joinIDs(ids))
	return ids, nil
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

// store is the outbox storage seam. fetch returns up to limit rows whose topic is not in failed, in
// id order; they stay locked until the transaction ends.
type store interface {
	begin(ctx context.Context) (storeTx, error)
}

type storeTx interface {
	fetch(ctx context.Context, limit int, failed []string) ([]row, error)
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

// fetchSQL is the head of the queue outside the topics that failed in this pass. Without failed
// topics it is the plain `ORDER BY id LIMIT n` over the primary key, so the healthy path keeps its
// plan; the exclusion is a list of placeholders rather than an array parameter so that lib/pq and
// pgx stdlib bind it alike.
func fetchSQL(failed int) string {
	where := ""
	if failed > 0 {
		placeholders := make([]string, failed)
		for i := range placeholders {
			placeholders[i] = "$" + strconv.Itoa(i+2)
		}
		where = ` WHERE topic NOT IN (` + strings.Join(placeholders, ",") + `)`
	}
	return `SELECT id, topic, message, options FROM public.redbus_outbox` + where + ` ORDER BY id LIMIT $1 FOR UPDATE`
}

func (t sqlTx) fetch(ctx context.Context, limit int, failed []string) ([]row, error) {
	args := make([]any, 0, len(failed)+1)
	args = append(args, limit)
	for _, topic := range failed {
		args = append(args, topic)
	}
	rs, err := t.tx.QueryContext(ctx, fetchSQL(len(failed)), args...)
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
