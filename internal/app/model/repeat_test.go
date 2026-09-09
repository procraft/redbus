package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prokraft/redbus/internal/pkg/runtime"
)

func TestRepeatApplyFailurePreservesAttemptAndUsesConsumerDelay(t *testing.T) {
	runtime.SetStatic("2026-09-09T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	repeat := Repeat{Attempt: 5}
	strategy := NewRepeatStrategyEven(5, Duration{Duration: time.Minute})

	repeat.ApplyFailure(strategy, true, 17*time.Minute)

	require.Equal(t, 5, repeat.Attempt)
	require.Nil(t, repeat.FinishedAt)
	require.Equal(t, "2026-09-09T10:17:00Z", repeat.StartedAt.Format(time.RFC3339))
}

func TestRepeatApplyFailurePreservedAttemptFallsBackToStrategyForInvalidDelay(t *testing.T) {
	runtime.SetStatic("2026-09-09T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	repeat := Repeat{Attempt: 2}
	strategy := NewRepeatStrategyEven(5, Duration{Duration: time.Minute})

	repeat.ApplyFailure(strategy, true, 0)

	require.Equal(t, 2, repeat.Attempt)
	require.Nil(t, repeat.FinishedAt)
	require.Equal(t, "2026-09-09T10:01:00Z", repeat.StartedAt.Format(time.RFC3339))
}

func TestRepeatApplyFailureKeepsLegacyAttemptSemantics(t *testing.T) {
	runtime.SetStatic("2026-09-09T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	repeat := Repeat{Attempt: 5}
	strategy := NewRepeatStrategyEven(5, Duration{Duration: time.Minute})

	repeat.ApplyFailure(strategy, false, 17*time.Minute)

	require.Equal(t, 5, repeat.Attempt)
	require.NotNil(t, repeat.FinishedAt)
}
