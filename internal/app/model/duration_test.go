package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDurationUnmarshalText(t *testing.T) {
	var duration Duration
	require.NoError(t, duration.UnmarshalText([]byte("5s")))
	require.Equal(t, 5*time.Second, duration.Duration)
}
