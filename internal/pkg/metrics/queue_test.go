package metrics

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/stretchr/testify/require"
)

func queueExposition(m *Metrics) string {
	response := httptest.NewRecorder()
	m.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	return response.Body.String()
}

func TestQueueAgesGrowBetweenSnapshotsAndExpire(t *testing.T) {
	m := New()
	now := time.Unix(1000, 0)
	m.queues.now = func() time.Time { return now }
	value := model.QueueSnapshot{Topic: "email", Group: "operational", Lag: 5, Oldest: now.Add(-time.Minute), Valid: true}
	m.SetQueueSnapshots("kafka", []model.QueueSnapshot{value}, nil)
	require.Contains(t, queueExposition(m), `redbus_kafka_consumer_head_age_seconds{group="operational",topic="email"} 60`)
	now = now.Add(time.Minute)
	body := queueExposition(m)
	require.Contains(t, body, `redbus_kafka_consumer_head_age_seconds{group="operational",topic="email"} 120`)
	require.Contains(t, body, `redbus_queue_snapshot_success{group="operational",source="kafka",topic="email"} 1`)
	now = now.Add(time.Minute + time.Second)
	body = queueExposition(m)
	require.NotContains(t, body, "redbus_kafka_consumer_head_age_seconds{")
	require.NotContains(t, body, "redbus_kafka_consumer_lag_messages{")
	require.Contains(t, body, `redbus_queue_snapshot_success{group="operational",source="kafka",topic="email"} 0`)
	require.Contains(t, body, `redbus_queue_snapshot_timestamp_seconds{group="operational",source="kafka",topic="email"} 1000`)
}

func TestQueueErrorsDoNotMasqueradeAsEmptyAndPartialSuccessSurvives(t *testing.T) {
	m := New()
	now := time.Unix(1000, 0)
	m.queues.now = func() time.Time { return now }
	valid := model.QueueSnapshot{Topic: "email", Group: "auth", Valid: true}
	invalid := model.QueueSnapshot{Topic: "email", Group: "operational", Valid: false}
	m.SetQueueSnapshots("kafka", []model.QueueSnapshot{valid, invalid}, errors.New("partial"))
	body := queueExposition(m)
	require.Contains(t, body, `redbus_kafka_consumer_head_age_seconds{group="auth",topic="email"} 0`)
	require.Contains(t, body, `redbus_kafka_consumer_lag_messages{group="auth",topic="email"} 0`)
	require.NotContains(t, body, `redbus_kafka_consumer_head_age_seconds{group="operational"`)
	require.Contains(t, body, `redbus_queue_snapshot_success{group="operational",source="kafka",topic="email"} 0`)
	now = now.Add(time.Second)
	m.SetQueueSnapshots("kafka", nil, errors.New("unavailable"))
	body = queueExposition(m)
	require.NotContains(t, body, "redbus_kafka_consumer_lag_messages{")
	require.Contains(t, body, `redbus_queue_snapshot_timestamp_seconds{group="auth",source="kafka",topic="email"} 1000`)
}

func TestRetrySuccessfulEmptyScanClearsOldestButErrorHidesValue(t *testing.T) {
	m := New()
	now := time.Unix(1000, 0)
	m.queues.now = func() time.Time { return now }
	m.SetQueueSnapshots("retry", []model.QueueSnapshot{{Topic: "email", Group: "operational", Oldest: now.Add(-time.Hour), Valid: true}}, nil)
	require.Contains(t, queueExposition(m), `redbus_retry_oldest_age_seconds{group="operational",topic="email"} 3600`)
	m.SetQueueSnapshots("retry", nil, nil)
	require.Contains(t, queueExposition(m), `redbus_retry_oldest_age_seconds{group="operational",topic="email"} 0`)
	m.SetQueueSnapshots("retry", nil, errors.New("database unavailable"))
	require.False(t, strings.Contains(queueExposition(m), "redbus_retry_oldest_age_seconds{"))
}

func TestQueueScrapeAndRefreshCanRunConcurrently(t *testing.T) {
	m := New()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 30 {
			m.SetQueueSnapshots("kafka", []model.QueueSnapshot{{Topic: "email", Group: "operational", Oldest: time.Now().Add(-time.Minute), Lag: 1, Valid: true}}, nil)
		}
	}()
	for range 30 {
		_ = queueExposition(m)
	}
	<-done
}
