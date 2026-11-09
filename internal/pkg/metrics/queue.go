package metrics

import (
	"sync"
	"time"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/prometheus/client_golang/prometheus"
)

const QueueSnapshotMaxAge = 2 * time.Minute

type queueKey struct{ source, topic, group string }
type queueObservation struct {
	value       model.QueueSnapshot
	at          time.Time
	lastSuccess time.Time
}

// queueCollector does no I/O. Failed or stale data expose health only, never fabricated zeroes.
type queueCollector struct {
	mu                                         sync.RWMutex
	values                                     map[queueKey]queueObservation
	now                                        func() time.Time
	lag, headAge, retryAge, success, timestamp *prometheus.Desc
}

func newQueueCollector() *queueCollector {
	desc := func(name, help string, labels []string) *prometheus.Desc {
		return prometheus.NewDesc("redbus_"+name, help, labels, nil)
	}
	return &queueCollector{
		values: make(map[queueKey]queueObservation), now: time.Now,
		lag:       desc("kafka_consumer_lag_messages", "Cached uncommitted Kafka positions, including all classes read by this group.", []string{"topic", "group"}),
		headAge:   desc("kafka_consumer_head_age_seconds", "Age of the oldest partition head Kafka record timestamp, not an email delivery-class age.", []string{"topic", "group"}),
		retryAge:  desc("retry_oldest_age_seconds", "Age since first failed delivery of the oldest pending or deferred retry; excludes exhausted records.", []string{"topic", "group"}),
		success:   desc("queue_snapshot_success", "Whether this group's latest background observation succeeded and is at most 120 seconds old.", []string{"source", "topic", "group"}),
		timestamp: desc("queue_snapshot_timestamp_seconds", "Unix timestamp of the last successful queue snapshot.", []string{"source", "topic", "group"}),
	}
}

func (c *queueCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.lag, c.headAge, c.retryAge, c.success, c.timestamp} {
		ch <- d
	}
}

func (c *queueCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := c.now()
	for key, observation := range c.values {
		valid := observation.value.Valid && now.Sub(observation.at) <= QueueSnapshotMaxAge
		health := 0.0
		if valid {
			health = 1
		}
		ch <- prometheus.MustNewConstMetric(c.success, prometheus.GaugeValue, health, key.source, key.topic, key.group)
		if !observation.lastSuccess.IsZero() {
			ch <- prometheus.MustNewConstMetric(c.timestamp, prometheus.GaugeValue, float64(observation.lastSuccess.UnixNano())/1e9, key.source, key.topic, key.group)
		}
		if !valid {
			continue
		}
		age := 0.0
		if !observation.value.Oldest.IsZero() {
			age = max(0, now.Sub(observation.value.Oldest).Seconds())
		}
		if key.source == "kafka" {
			ch <- prometheus.MustNewConstMetric(c.lag, prometheus.GaugeValue, float64(observation.value.Lag), key.topic, key.group)
			ch <- prometheus.MustNewConstMetric(c.headAge, prometheus.GaugeValue, age, key.topic, key.group)
		} else {
			ch <- prometheus.MustNewConstMetric(c.retryAge, prometheus.GaugeValue, age, key.topic, key.group)
		}
	}
}

// SetQueueSnapshots replaces observations atomically. An absent retry group is
// empty after a successful whole-table scan, unavailable after a failed scan.
func (m *Metrics) SetQueueSnapshots(source string, values []model.QueueSnapshot, err error) {
	c := m.queues
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	seen := make(map[queueKey]bool, len(values))
	for _, value := range values {
		key := queueKey{source, string(value.Topic), string(value.Group)}
		observation := c.values[key]
		observation.value, observation.at = value, now
		if value.Valid {
			observation.lastSuccess = now
		}
		c.values[key], seen[key] = observation, true
	}
	for key, observation := range c.values {
		if key.source != source || seen[key] {
			continue
		}
		observation.value = model.QueueSnapshot{Valid: err == nil && source == "retry"}
		observation.at = now
		if observation.value.Valid {
			observation.lastSuccess = now
		}
		c.values[key] = observation
	}
}
