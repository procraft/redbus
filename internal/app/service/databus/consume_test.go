package databus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/pb"
	"github.com/procraft/redbus/internal/app/model"
	"github.com/procraft/redbus/internal/config"
	"github.com/procraft/redbus/internal/pkg/stream"
)

type consumerStub struct {
	model.IConsumer
	consume     func(ctx context.Context, processor func(context.Context, model.MessageList) error) error
	state       model.ConsumerState
	callCount   atomic.Int32
	closedCount atomic.Int32
}

func (c *consumerStub) GetHosts() []string        { return []string{"kafka:9092"} }
func (c *consumerStub) GetTopic() model.TopicName { return "orders" }
func (c *consumerStub) GetGroup() model.GroupName { return "billing" }
func (c *consumerStub) GetID() model.ConsumerId   { return "worker-1" }
func (c *consumerStub) GetState() model.ConsumerState {
	if c.state == 0 {
		return model.ConsumerStateConnected
	}
	return c.state
}
func (c *consumerStub) SetState(state model.ConsumerState) { c.state = state }
func (c *consumerStub) Close() (bool, error) {
	c.closedCount.Add(1)
	return true, nil
}
func (c *consumerStub) Reconnect(context.Context) error { return nil }
func (c *consumerStub) Consume(ctx context.Context, processor func(context.Context, model.MessageList) error) error {
	c.callCount.Add(1)
	return c.consume(ctx, processor)
}

type connStoreStub struct {
	IConnStore
	added   atomic.Int32
	removed atomic.Int32
}

func (s *connStoreStub) AddConsumer(model.IConsumer, pb.RedbusService_ConsumeServer, *model.RepeatStrategy, stream.AbortFn, model.ConsumeLimits) {
	s.added.Add(1)
}
func (s *connStoreStub) RemoveConsumer(model.IConsumer) { s.removed.Add(1) }

type metricsStub struct{ IMetrics }

func (metricsStub) AddConsumer(string, string, string, string)         {}
func (metricsStub) RemoveConsumer(string, string, string)              {}
func (metricsStub) ChangeConsumerState(string, string, string, string) {}
func (metricsStub) ObserveKafkaReconnect(string, string, string)       {}
func (metricsStub) ObserveConsumerConnection(string, string, string)   {}

func newTestBus(connStore IConnStore) *DataBus {
	conf := &config.Config{}
	conf.Kafka.FailTimeout = model.NewDuration(time.Millisecond)
	return New(conf, connStore, nil, nil, metricsStub{})
}

// Ошибка обработчика раньше сравнивалась с локальной копией ошибки, поэтому ветка никогда не
// срабатывала: consumer оставался CONNECTED, а клиент не получал ни onError, ни onCompleted.
func TestConsumeReturnsHandlerErrorAndStopsConsuming(t *testing.T) {
	handlerErr := fmt.Errorf("%w: result id %q not in batch", model.ErrHandler, "1/2008")
	c := &consumerStub{consume: func(context.Context, func(context.Context, model.MessageList) error) error {
		return handlerErr
	}}
	connStore := &connStoreStub{}
	bus := newTestBus(connStore)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bus.Consume(ctx, c, nil, nil, model.ConsumeLimits{}, nil, nil, cancel)

	require.ErrorIs(t, err, model.ErrHandler)
	require.Equal(t, int32(1), c.callCount.Load(), "после ошибки обработчика чтение не возобновляется")
	require.Equal(t, int32(1), c.closedCount.Load())
	require.Equal(t, int32(1), connStore.added.Load())
	require.Equal(t, int32(1), connStore.removed.Load())
}

// Ошибка чтения kafka обрабатывается иначе: consumer переподключается и продолжает работу.
func TestConsumeReconnectsOnKafkaErrorAndFinishesWithoutError(t *testing.T) {
	var attempts atomic.Int32
	c := &consumerStub{}
	c.consume = func(context.Context, func(context.Context, model.MessageList) error) error {
		if attempts.Add(1) == 1 {
			return fmt.Errorf("Failed to read kafka message: broken pipe")
		}
		return nil
	}
	bus := newTestBus(&connStoreStub{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := bus.Consume(ctx, c, nil, nil, model.ConsumeLimits{}, nil, nil, cancel)

	require.NoError(t, err)
	require.Equal(t, int32(2), c.callCount.Load())
}

type stateRecorder struct {
	metricsStub
	mu     sync.Mutex
	states []string
}

func (r *stateRecorder) AddConsumer(_, _, _, state string) { r.record(state) }
func (r *stateRecorder) ChangeConsumerState(_, _, _, state string) {
	r.record(state)
}
func (r *stateRecorder) record(state string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n := len(r.states); n == 0 || r.states[n-1] != state {
		r.states = append(r.states, state)
	}
}

// Kafka отказывает в доступе к топику не сразу, а после входа в группу. Consumer в таком цикле
// не должен ни разу числиться connected, иначе алерт по reconnecting сбрасывается.
func TestConsumeNeverReportsConnectedWhileKafkaKeepsFailing(t *testing.T) {
	var attempts atomic.Int32
	c := &consumerStub{state: model.ConsumerStateConnecting}
	c.consume = func(context.Context, func(context.Context, model.MessageList) error) error {
		if attempts.Add(1) <= 3 {
			time.Sleep(5 * time.Millisecond)
			return fmt.Errorf("Failed to read kafka message: [29] Topic Authorization Failed")
		}
		return nil
	}
	rec := &stateRecorder{}
	bus := newTestBus(&connStoreStub{})
	bus.metrics = rec
	bus.stableAfter = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, bus.Consume(ctx, c, nil, nil, model.ConsumeLimits{}, nil, nil, cancel))
	require.Equal(t, []string{"connecting", "reconnecting"}, rec.states)
}

func TestConsumeReportsConnectedOnFirstBatchOrAfterStableRead(t *testing.T) {
	for name, tc := range map[string]struct {
		stableAfter time.Duration
		batch       bool
	}{
		"first batch":   {stableAfter: time.Hour, batch: true},
		"stable reader": {stableAfter: time.Millisecond, batch: false},
	} {
		t.Run(name, func(t *testing.T) {
			c := &consumerStub{state: model.ConsumerStateConnecting}
			c.consume = func(ctx context.Context, processor func(context.Context, model.MessageList) error) error {
				if tc.batch {
					return processor(ctx, model.MessageList{{Id: "0/1"}})
				}
				time.Sleep(50 * time.Millisecond)
				return nil
			}
			rec := &stateRecorder{}
			bus := newTestBus(&connStoreStub{})
			bus.metrics = rec
			bus.stableAfter = tc.stableAfter
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			handler := func(context.Context, model.MessageList) error { return nil }

			require.NoError(t, bus.Consume(ctx, c, nil, nil, model.ConsumeLimits{}, nil, handler, cancel))
			require.Equal(t, []string{"connecting", "connected", "reconnecting"}, rec.states)
		})
	}
}
