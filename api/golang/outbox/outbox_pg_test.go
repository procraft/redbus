package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/internal/pgtest"
	"github.com/procraft/redbus/api/golang/outbox"
	"github.com/procraft/redbus/api/golang/pb"
	"github.com/procraft/redbus/api/golang/producer"
)

type collector struct {
	mu       sync.Mutex
	requests []*pb.ProduceBatchRequest
	ch       chan struct{}
}

func (c *collector) publish(_ context.Context, req *pb.ProduceBatchRequest) error {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	c.mu.Unlock()
	if c.ch != nil {
		c.ch <- struct{}{}
	}
	return nil
}

func TestWriteUsesScalaOptionsFormat(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, outbox.Write(ctx, tx, "topic", []byte("payload"),
		producer.WithKey("key"), producer.WithIdempotencyKey("ik"), producer.WithVersion(5), producer.WithTimestamp(ts)))
	require.NoError(t, outbox.Write(ctx, tx, "topic", nil, producer.WithIdempotencyKey("ik2")))
	require.NoError(t, tx.Commit())

	rows, err := db.Query(`SELECT topic, message, options FROM public.redbus_outbox ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var got []map[string]any
	for rows.Next() {
		var (
			topic   string
			message []byte
			opts    []byte
		)
		require.NoError(t, rows.Scan(&topic, &message, &opts))
		require.Equal(t, "topic", topic)
		m := map[string]any{}
		require.NoError(t, json.Unmarshal(opts, &m))
		got = append(got, m)
	}
	require.Equal(t, map[string]any{
		"key": "key", "idempotencyKey": "ik", "version": float64(5), "timestamp": "2026-01-02T03:04:05Z",
	}, got[0])
	require.Equal(t, "ik2", got[1]["idempotencyKey"])
	require.NotContains(t, got[1], "key")
	require.NotContains(t, got[1], "version")
	require.Contains(t, got[1], "timestamp")
}

func TestFlushDeliversRowsWrittenByScalaSDK(t *testing.T) {
	db := pgtest.New(t)
	_, err := db.Exec(`INSERT INTO public.redbus_outbox (topic, message, options) VALUES
		('a', 'x', '{"idempotencyKey":"1","timestamp":"2026-01-02T03:04:05.123+03:00"}'),
		('a', 'y', '{"key":"k","version":9,"idempotencyKey":"2"}'),
		('b', 'z', '{}')`)
	require.NoError(t, err)

	var c collector
	require.NoError(t, outbox.NewFlusher(db.DB, c.publish, outbox.WithBatchSize(10)).Flush(context.Background()))

	require.Len(t, c.requests, 2)
	require.Equal(t, "a", c.requests[0].Topic)
	require.Len(t, c.requests[0].MessageList, 2)
	require.Equal(t, "2026-01-02T03:04:05.123+03:00", c.requests[0].MessageList[0].Timestamp)
	require.Equal(t, "k", c.requests[0].MessageList[1].Key)
	require.Equal(t, int64(9), c.requests[0].MessageList[1].Version)
	require.Equal(t, "b", c.requests[1].Topic)

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM public.redbus_outbox`).Scan(&n))
	require.Zero(t, n)
}

func TestFlushDeliversOtherTopicsPastFailingTopic(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()
	for i, topic := range []string{"a", "b", "a", "c", "b"} {
		require.NoError(t, outbox.Write(ctx, db, topic, []byte("m"), producer.WithIdempotencyKey(fmt.Sprint(i+1))))
	}
	var sent []string
	publish := func(_ context.Context, req *pb.ProduceBatchRequest) error {
		if req.Topic == "a" {
			return errors.New("[29] Topic Authorization Failed")
		}
		for _, m := range req.MessageList {
			sent = append(sent, req.Topic+m.IdempotencyKey)
		}
		return nil
	}

	require.ErrorContains(t, outbox.NewFlusher(db.DB, publish, outbox.WithBatchSize(10)).Flush(ctx), "Topic Authorization Failed")

	require.Equal(t, []string{"b2", "b5", "c4"}, sent)
	rows, err := db.Query(`SELECT topic, options->>'idempotencyKey' FROM public.redbus_outbox ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var left []string
	for rows.Next() {
		var topic, key string
		require.NoError(t, rows.Scan(&topic, &key))
		left = append(left, topic+key)
	}
	require.Equal(t, []string{"a1", "a3"}, left)
}

func TestConcurrentFlushersPublishEveryRowOnce(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()
	const total = 40
	for i := 0; i < total; i++ {
		topic := "a"
		if i%7 == 0 {
			topic = "b"
		}
		require.NoError(t, outbox.Write(ctx, db, topic, []byte("m"), producer.WithIdempotencyKey(fmt.Sprint(i))))
	}

	var (
		mu   sync.Mutex
		seen = map[string]int{}
	)
	publish := func(_ context.Context, req *pb.ProduceBatchRequest) error {
		time.Sleep(20 * time.Millisecond) // keep the row locks long enough to overlap
		mu.Lock()
		defer mu.Unlock()
		for _, m := range req.MessageList {
			seen[m.IdempotencyKey]++
		}
		return nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, outbox.NewFlusher(db.DB, publish, outbox.WithBatchSize(5)).Flush(ctx))
		}()
	}
	wg.Wait()

	require.Len(t, seen, total)
	for key, n := range seen {
		require.Equal(t, 1, n, "idempotency key %s published %d times", key, n)
	}
	var left int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM public.redbus_outbox`).Scan(&left))
	require.Zero(t, left)
}

func TestRunReactsToNotification(t *testing.T) {
	db := pgtest.New(t)
	c := collector{ch: make(chan struct{}, 10)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := outbox.NewFlusher(db.DB, c.publish, outbox.WithListenDSN(db.DSN), outbox.WithSweepInterval(time.Hour))
	done := make(chan error)
	go func() { done <- f.Run(ctx) }()

	// Let the listener subscribe; the start sweep finds nothing.
	time.Sleep(500 * time.Millisecond)
	require.NoError(t, outbox.Write(ctx, db, "topic", []byte("m")))

	select {
	case <-c.ch:
	case <-time.After(5 * time.Second):
		t.Fatal("the flusher did not react to pg_notify")
	}
	cancel()
	require.NoError(t, <-done)
}
