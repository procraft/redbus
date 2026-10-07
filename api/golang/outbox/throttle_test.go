package outbox

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/pb"
)

func TestErrorThrottleReportsOncePerIntervalPerKey(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	th := newErrorThrottle(time.Minute, func() time.Time { return now })
	var got []string
	report := func(key string) {
		th.report(key, func(suppressed int) { got = append(got, key+":"+strings.Repeat("+", suppressed)) })
	}

	report("a")
	report("a")
	report("b") // another topic has its own budget
	now = now.Add(59 * time.Second)
	report("a")
	now = now.Add(time.Second)
	report("a") // the interval has passed: emitted with the two suppressed reports
	report("a")

	require.Equal(t, []string{"a:", "b:", "a:++"}, got)
}

type syncBuffer struct {
	ch chan string
}

func (b syncBuffer) Write(p []byte) (int, error) {
	b.ch <- string(bytes.Clone(p))
	return len(p), nil
}

func TestRunLogsTopicFailureAtErrorLevel(t *testing.T) {
	store := &memStore{rows: []row{rowOf(1, "a", 0), rowOf(2, "b", 0)}}
	lines := syncBuffer{ch: make(chan string, 10)}
	log := slog.New(slog.NewTextHandler(lines, nil))
	f := newFlusher(store, func(_ context.Context, req *pb.ProduceBatchRequest) error {
		if req.Topic == "a" {
			return context.DeadlineExceeded
		}
		return nil
	}, WithLogger(log), WithSweepInterval(time.Hour))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- f.Run(ctx) }()

	line := <-lines.ch
	cancel()
	require.NoError(t, <-done)
	require.Contains(t, line, "level=ERROR")
	require.Contains(t, line, "topic=a")
	require.Contains(t, line, "suppressed=0")
}
