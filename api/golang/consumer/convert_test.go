package consumer

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestToPBResultListConvertsRetryLaterError(t *testing.T) {
	result := toPBResultList([]ProcessResult{{
		id:  "message-id",
		err: NewRetryLaterError(errors.New("provider throttled"), 1500*time.Millisecond),
	}}, slog.Default())[0]

	require.False(t, result.Ok)
	require.Equal(t, "provider throttled", result.Message)
	require.True(t, result.RetryLater)
	require.Equal(t, int32(2), result.RetryAfterSec)
}

func TestToPBResultListKeepsOrdinaryErrorsBackwardCompatible(t *testing.T) {
	result := toPBResultList([]ProcessResult{{id: "message-id", err: errors.New("failed")}}, slog.Default())[0]

	require.False(t, result.Ok)
	require.False(t, result.RetryLater)
	require.Zero(t, result.RetryAfterSec)
}

func TestToPBResultListLogsDeferralBelowErrorLevel(t *testing.T) {
	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))

	toPBResultList([]ProcessResult{
		{id: "deferred-id", err: NewRetryLaterError(errors.New("busy"), time.Second)},
		{id: "failed-id", err: errors.New("broken")},
	}, log)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "level=DEBUG")
	require.Contains(t, lines[0], "redbus: process payload deferred")
	require.Contains(t, lines[0], "id=deferred-id")
	require.Contains(t, lines[0], "delay=1s")
	require.Contains(t, lines[1], "level=WARN")
	require.Contains(t, lines[1], "redbus: process payload error")
	require.Contains(t, lines[1], "id=failed-id")
}
