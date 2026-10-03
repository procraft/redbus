package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prokraft/redbus/internal/app/model"
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
	require.NotNil(t, actual.Queue)
	require.Empty(t, actual.Queue)
}

func TestToQueue(t *testing.T) {
	stat := model.RepeatStat{
		{Topic: "orders", Group: "billing", AllCount: 5, FailedCount: 1, DeferredCount: 1, LastError: "boom"},
		{Topic: "orders", Group: "mailing", AllCount: 9, FailedCount: 0, DeferredCount: 9, LastDeferredReason: "busy"},
		{Topic: "orders", Group: "audit", AllCount: 4, FailedCount: 4},
		{Topic: "events", Group: "billing", AllCount: 9, FailedCount: 0, DeferredCount: 0},
		{Topic: "events", Group: "mailing", AllCount: 1, FailedCount: 3, DeferredCount: 2},
	}

	all := toQueue(stat, "", "")
	require.Equal(t, []queueItem{
		{Topic: "events", Group: "billing", PendingCount: 9},
		{Topic: "orders", Group: "mailing", DeferredCount: 9, LastDeferredReason: "busy"},
		{Topic: "orders", Group: "billing", PendingCount: 3, DeferredCount: 1, FailedCount: 1, LastError: "boom"},
		// Inconsistent counters from a snapshot race are clamped instead of going negative.
		{Topic: "events", Group: "mailing", PendingCount: 0, DeferredCount: 2, FailedCount: 3},
	}, all)

	byTopic := toQueue(stat, "orders", "")
	require.Len(t, byTopic, 2)
	require.Equal(t, "mailing", byTopic[0].Group)
	require.Equal(t, "billing", byTopic[1].Group)

	byGroup := toQueue(stat, "", "billing")
	require.Len(t, byGroup, 2)
	require.Equal(t, "events", byGroup[0].Topic)

	require.Empty(t, toQueue(stat, "orders", "audit"))
	require.NotNil(t, toQueue(nil, "", ""))
}

func TestToQueueTreatsLegacyBusAsPending(t *testing.T) {
	actual := toQueue(model.RepeatStat{{Topic: "orders", Group: "billing", AllCount: 3, FailedCount: 1}}, "", "")
	require.Equal(t, []queueItem{{Topic: "orders", Group: "billing", PendingCount: 2, FailedCount: 1}}, actual)
}
