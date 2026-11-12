package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/internal/pkg/runtime"
)

func TestRepeatLastAttemptUsesEffectiveStrategy(t *testing.T) {
	defaultStrategy := NewRepeatStrategyEven(5, Duration{Duration: time.Minute})
	repeat := Repeat{Attempt: 4}
	require.False(t, repeat.IsLastAttempt(defaultStrategy))
	repeat.ApplyNextAttempt(defaultStrategy)
	require.True(t, repeat.IsLastAttempt(defaultStrategy))
	repeat.ApplyFailure(defaultStrategy, true, time.Minute)
	require.True(t, repeat.IsLastAttempt(defaultStrategy))
	require.Nil(t, repeat.FinishedAt)

	repeat.Strategy = NewRepeatStrategyEven(6, Duration{Duration: time.Minute})
	require.False(t, repeat.IsLastAttempt(defaultStrategy))
	repeat.ApplyNextAttempt(defaultStrategy)
	require.True(t, repeat.IsLastAttempt(defaultStrategy))
	repeat.ApplyNextAttempt(defaultStrategy)
	require.NotNil(t, repeat.FinishedAt)
}

func TestRepeatApplyFailurePreservesAttemptAndUsesConsumerDelay(t *testing.T) {
	runtime.SetStatic("2026-09-09T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	repeat := Repeat{Attempt: 5}
	strategy := NewRepeatStrategyEven(5, Duration{Duration: time.Minute})

	repeat.ApplyFailure(strategy, true, 17*time.Minute)

	require.Equal(t, 5, repeat.Attempt)
	require.Nil(t, repeat.FinishedAt)
	require.True(t, repeat.Deferred)
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
	require.False(t, repeat.Deferred)
}

func TestRepeatOrdinaryFailureClearsDeferred(t *testing.T) {
	runtime.SetStatic("2026-09-09T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	repeat := Repeat{Attempt: 2, Deferred: true}
	strategy := NewRepeatStrategyEven(5, Duration{Duration: time.Minute})

	repeat.ApplyFailure(strategy, false, 0)

	require.Equal(t, 3, repeat.Attempt)
	require.Nil(t, repeat.FinishedAt)
	require.False(t, repeat.Deferred)
}

func TestRepeatEmptyResultClearsDeferred(t *testing.T) {
	repeat := Repeat{Attempt: 2, Deferred: true}

	repeat.ApplyNextAttempt(NewRepeatStrategyEven(5, Duration{Duration: time.Minute}))

	require.Equal(t, 3, repeat.Attempt)
	require.False(t, repeat.Deferred)
}
