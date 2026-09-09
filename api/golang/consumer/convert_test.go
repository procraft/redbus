package consumer

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestToPBResultListConvertsRetryLaterError(t *testing.T) {
	result := toPBResultList([]ProcessResult{{
		id:  "message-id",
		err: NewRetryLaterError(errors.New("provider throttled"), 1500*time.Millisecond),
	}})[0]

	require.False(t, result.Ok)
	require.Equal(t, "provider throttled", result.Message)
	require.True(t, result.PreserveAttempt)
	require.Equal(t, int32(2), result.RetryAfterSec)
}

func TestToPBResultListKeepsOrdinaryErrorsBackwardCompatible(t *testing.T) {
	result := toPBResultList([]ProcessResult{{id: "message-id", err: errors.New("failed")}})[0]

	require.False(t, result.Ok)
	require.False(t, result.PreserveAttempt)
	require.Zero(t, result.RetryAfterSec)
}
