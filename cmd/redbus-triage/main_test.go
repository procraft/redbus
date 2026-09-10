package main

import (
	"testing"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/stretchr/testify/require"
)

func TestValidateLoopbackAddress(t *testing.T) {
	require.NoError(t, validateLoopbackAddress("127.0.0.1:6463"))
	require.NoError(t, validateLoopbackAddress("localhost:6463"))
	require.Error(t, validateLoopbackAddress("redbus.example.com:4363"))
}

func TestToResponse(t *testing.T) {
	since := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	until := since.Add(time.Hour)
	actual := toResponse(model.RepeatTriageStat{
		Since: since,
		Until: until,
		List: []model.RepeatTriageStatItem{{
			Topic:       "orders",
			Group:       "billing",
			FailedCount: 2,
			Errors: []model.RepeatErrorStat{{
				Error:         "boom",
				FailedCount:   2,
				FirstFailedAt: since.Add(time.Minute),
				LastFailedAt:  until.Add(-time.Minute),
			}},
		}},
	})

	require.Equal(t, since.UnixMilli(), actual.SinceUnixMs)
	require.Equal(t, until.UnixMilli(), actual.UntilUnixMs)
	require.Equal(t, "orders", actual.List[0].Topic)
	require.Equal(t, "boom", actual.List[0].Errors[0].Error)
}
