// Package redbus is the recommended entry point of the Go SDK: a client configured by Settings
// with direct and transactional-outbox produce, the outbox flusher and consumers with inbox dedup,
// for raw payloads and for protobuf messages.
package redbus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/prokraft/redbus/api/golang/consumer"
	"github.com/prokraft/redbus/api/golang/inbox"
	"github.com/prokraft/redbus/api/golang/outbox"
	"github.com/prokraft/redbus/api/golang/producer"
)

// ErrNoDatabase is returned when an outbox or inbox feature is used without WithDB.
var ErrNoDatabase = errors.New("redbus: the client has no database (WithDB)")

type Option func(c *Client)

// WithDB sets the service database that holds redbus_inbox and redbus_outbox. It is required for
// the flusher and for the inbox modes. A pgx pool can be passed through pgx's stdlib package.
func WithDB(db *sql.DB) Option {
	return func(c *Client) { c.db = db }
}

// WithLogger sets the logger of the client, its consumers and the flusher (default slog.Default()).
func WithLogger(log *slog.Logger) Option {
	return func(c *Client) { c.log = log }
}

// WithListenDSN lets the flusher react to pg_notify('redbus_outbox') through a dedicated lib/pq
// connection; without it the flusher relies on its periodic sweep only.
func WithListenDSN(dsn string) Option {
	return func(c *Client) { c.listenDSN = dsn }
}

// WithFlusherOptions passes further options to the outbox flusher (sweep interval, timeouts).
func WithFlusherOptions(opts ...outbox.FlusherOption) Option {
	return func(c *Client) { c.flusherOptions = append(c.flusherOptions, opts...) }
}

// WithUnavailableTimeout sets the consumers' pause before a reconnect (default 60 s).
func WithUnavailableTimeout(d time.Duration) Option {
	return func(c *Client) { c.unavailableTimeout = d }
}

// Client is a bus client configured by Settings. A disabled side is a no-op: Produce answers
// false, ProduceTx writes no row, Consume returns nil at once and StartFlusher starts nothing.
type Client struct {
	settings           Settings
	db                 *sql.DB
	log                *slog.Logger
	listenDSN          string
	flusherOptions     []outbox.FlusherOption
	unavailableTimeout time.Duration

	bus         transport // nil when both sides are disabled
	mu          sync.Mutex
	flusherOn   bool
	stopFlusher context.CancelFunc
	wg          sync.WaitGroup
}

func New(settings Settings, opts ...Option) (*Client, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	c := &Client{settings: settings, log: slog.Default()}
	for _, o := range opts {
		o(c)
	}
	if settings.Enabled() {
		t, err := newGRPCTransport(c)
		if err != nil {
			return nil, err
		}
		c.bus = t
		c.log.Info("redbus: connect to service", "addr", fmt.Sprintf("%s:%d", settings.Host, settings.Port))
	}
	return c, nil
}

func (c *Client) Settings() Settings {
	return c.settings
}

// Produce publishes directly over gRPC. It reports false without an error when the producer is
// disabled; a rejected message fails with producer.ErrRejected.
func (c *Client) Produce(ctx context.Context, topic string, message []byte, opts ...producer.OptionFn) (bool, error) {
	if c.bus == nil || !c.settings.ProducerEnabled {
		return false, nil
	}
	if err := c.bus.produce(ctx, topic, message, opts...); err != nil {
		return false, err
	}
	return true, nil
}

// ProduceProto is Produce for a protobuf message.
func (c *Client) ProduceProto(ctx context.Context, topic string, message proto.Message, opts ...producer.OptionFn) (bool, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return false, fmt.Errorf("redbus: marshal %T: %w", message, err)
	}
	return c.Produce(ctx, topic, data, opts...)
}

// ProduceTx is the transactional outbox: it inserts the message into redbus_outbox through tx
// (normally the caller's *sql.Tx) and the flusher delivers it after commit. It writes nothing when
// the producer is disabled.
func (c *Client) ProduceTx(ctx context.Context, tx outbox.Execer, topic string, message []byte, opts ...producer.OptionFn) error {
	if !c.settings.ProducerEnabled {
		return nil
	}
	return outbox.Write(ctx, tx, topic, message, opts...)
}

// ProduceProtoTx is ProduceTx for a protobuf message.
func (c *Client) ProduceProtoTx(ctx context.Context, tx outbox.Execer, topic string, message proto.Message, opts ...producer.OptionFn) error {
	if !c.settings.ProducerEnabled {
		return nil
	}
	data, err := proto.Marshal(message)
	if err != nil {
		return fmt.Errorf("redbus: marshal %T: %w", message, err)
	}
	return outbox.Write(ctx, tx, topic, data, opts...)
}

// StartFlusher starts the outbox flusher in the background until ctx is cancelled or Close is
// called; Close stops it and waits for it. Only the first call per client starts it. Rows are locked while they are published, so
// flushers of several replicas of a service do not publish the same rows twice.
func (c *Client) StartFlusher(ctx context.Context) error {
	if c.bus == nil || !c.settings.ProducerEnabled {
		return nil
	}
	if c.db == nil {
		return ErrNoDatabase
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.flusherOn {
		return nil
	}
	c.flusherOn = true
	c.log.Info("redbus: start outbox flusher", "batchSize", c.settings.batchSize())
	opts := []outbox.FlusherOption{
		outbox.WithBatchSize(c.settings.batchSize()),
		outbox.WithLogger(c.log),
		outbox.WithListenDSN(c.listenDSN),
	}
	opts = append(opts, c.flusherOptions...)
	ctx, c.stopFlusher = context.WithCancel(ctx)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.bus.runFlusher(ctx, c.db, opts...); err != nil {
			c.log.Error("redbus: outbox flusher stopped", "error", err)
		}
	}()
	return nil
}

// Consume consumes topic as group with the given inbox mode until ctx is cancelled (see
// consumer.Service.ConsumeMessages). It returns nil at once when the consumer is disabled. The
// inbox lives in the client's database; options given later override it.
func (c *Client) Consume(
	ctx context.Context,
	topic, group string,
	mode inbox.Mode,
	handler consumer.Handler,
	opts ...consumer.OptionFn,
) error {
	if c.bus == nil || !c.settings.ConsumerEnabled {
		c.log.Info("redbus: consumer is disabled", "topic", topic, "group", group)
		return nil
	}
	if mode != inbox.Disabled && c.db == nil {
		return ErrNoDatabase
	}
	c.log.Info("redbus: consume", "topic", topic, "group", group, "inbox", mode.String())
	var inboxDB inbox.DB
	if c.db != nil {
		inboxDB = c.db
	}
	all := append([]consumer.OptionFn{consumer.WithInbox(inboxDB, mode)}, opts...)
	return c.bus.consume(ctx, topic, group, handler, all...)
}

// ConsumeProto is Client.Consume that decodes every payload as T. A payload that is not a valid T
// is logged and acknowledged, because a retry cannot fix it.
//
//	err := redbus.ConsumeProto(ctx, client, "topic", "group", inbox.Transactional,
//		func(ctx context.Context, req *pb.Request, msg consumer.Message) error { … })
func ConsumeProto[T any, PT interface {
	*T
	proto.Message
}](
	ctx context.Context,
	c *Client,
	topic, group string,
	mode inbox.Mode,
	handler func(ctx context.Context, message PT, msg consumer.Message) error,
	opts ...consumer.OptionFn,
) error {
	return c.Consume(ctx, topic, group, mode, decoding(topic, group, c.log, handler), opts...)
}

func decoding[T any, PT interface {
	*T
	proto.Message
}](topic, group string, log *slog.Logger, handler func(ctx context.Context, message PT, msg consumer.Message) error) consumer.Handler {
	return func(ctx context.Context, msg consumer.Message) error {
		message := PT(new(T))
		if err := proto.Unmarshal(msg.Data, message); err != nil {
			log.Error("redbus.receive.invalid",
				"topic", topic, "group", group, "id", msg.ID, "payloadBytes", len(msg.Data), "action", "drop", "error", err)
			return nil
		}
		return handler(ctx, message, msg)
	}
}

// Close stops the flusher, waits for it and releases the gRPC connection. Stop consumers first
// by cancelling their context: they use the same connection.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.stopFlusher != nil {
		c.stopFlusher()
	}
	c.mu.Unlock()
	c.wg.Wait()
	if c.bus == nil {
		return nil
	}
	return c.bus.close()
}
