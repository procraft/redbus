package model

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sergiusd/redbus/internal/pkg/runtime"
)

func TestRepeatStrategy(t *testing.T) {
	runtime.SetStatic("2020-01-01T10:00:00Z")
	cs := []struct {
		conf     string
		expected []string
	}{
		{
			conf: fmt.Sprintf(`{"kind": "%s", "max": 5, "config": {"interval": %d}}`, RepeatKindAnnual, 5*time.Second),
			expected: []string{
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:05Z",
			},
		},
		{
			conf: fmt.Sprintf(`{"kind": "%s", "max": 5, "config": {"interval": %d}}`, RepeatKindProgressive, 5*time.Second),
			expected: []string{
				"2020-01-01T10:00:05Z",
				"2020-01-01T10:00:10Z",
				"2020-01-01T10:00:15Z",
				"2020-01-01T10:00:20Z",
			},
		},
		{
			conf: fmt.Sprintf(`{"kind": "%s", "max": 5, "config": {"interval": %d, "multiplier": 2}}`, RepeatKindProgressive, 5*time.Second),
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
		var strategy RepeatStrategy
		err := json.Unmarshal([]byte(c.conf), &strategy)
		assert.Nil(t, err)
		for i, expected := range c.expected {
			attempt := i + 1
			actual := strategy.GetNextStartedAt(attempt).Format(time.RFC3339)
			assert.Equal(t, expected, actual, "conf: %s, attempt: %d", c.conf, attempt)
		}
	}
}
