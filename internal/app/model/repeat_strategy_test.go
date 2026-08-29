package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prokraft/redbus/internal/pkg/runtime"
)

func TestRepeatStrategy(t *testing.T) {
	runtime.SetStatic("2020-01-01T10:00:00Z")
	t.Cleanup(runtime.ResetNowFn)

	cs := []struct {
		name     string
		conf     string
		expected []string
	}{
		{
			name: "even",
			conf: `{"kind":"even","maxAttempts":5,"evenConfig":{"interval":"5s"}}`,
			expected: []string{
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:05Z",
			},
		},
		{
			name: "progressive with default multiplier",
			conf: `{"kind":"progressive","maxAttempts":5,"progressiveConfig":{"interval":"5s"}}`,
			expected: []string{
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:10Z",
				"2020-01-01T10:00:15Z",
				"2020-01-01T10:00:20Z",
			},
		},
		{
			name: "progressive with multiplier",
			conf: `{"kind":"progressive","maxAttempts":5,"progressiveConfig":{"interval":"5s","multiplier":2}}`,
			expected: []string{
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:15Z",
				"2020-01-01T10:00:35Z",
				"2020-01-01T10:01:15Z",
				"2020-01-01T10:02:35Z",
			},
		},
	}
	for _, c := range cs {
		t.Run(c.name, func(t *testing.T) {
			var strategy RepeatStrategy
			require.NoError(t, json.Unmarshal([]byte(c.conf), &strategy))
			require.Equal(t, 5, strategy.MaxAttempts)

			for i, expected := range c.expected {
				attempt := i + 1
				actual := strategy.GetNextStartedAt(attempt).Format(time.RFC3339)
				require.Equal(t, expected, actual, "attempt: %d", attempt)
			}
		})
	}
}
