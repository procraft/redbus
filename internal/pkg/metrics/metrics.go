package metrics

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const namespace = "redbus"

type Metrics struct {
	registry *prometheus.Registry

	activeConsumerMu     sync.Mutex
	activeConsumerStates map[consumerKey]string

	producedMessages           *prometheus.CounterVec
	produceDuration            *prometheus.HistogramVec
	consumedMessages           *prometheus.CounterVec
	consumerProcessingDuration *prometheus.HistogramVec
	consumerBatchSize          *prometheus.HistogramVec
	activeConsumers            *prometheus.GaugeVec
	consumerConnections        *prometheus.CounterVec
	kafkaConsumerReconnects    *prometheus.CounterVec
	retryEnqueued              *prometheus.CounterVec
	retryAttempts              *prometheus.CounterVec
	retrySkipped               *prometheus.CounterVec
	retryRecords               *prometheus.GaugeVec
	repeaterRuns               *prometheus.CounterVec
	repeaterIterationDuration  prometheus.Histogram
	repeaterIterationMessages  prometheus.Histogram
	grpcServerRequests         *prometheus.CounterVec
	grpcServerDuration         *prometheus.HistogramVec
	grpcServerActiveStreams    *prometheus.GaugeVec
}

type consumerKey struct {
	topic string
	group string
	id    string
}

type DBPoolStatsProvider interface {
	Stat() *pgxpool.Stat
}

type dbPoolCollector struct {
	pool        DBPoolStatsProvider
	connections *prometheus.Desc
}

func (c dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.connections
}

func (c dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	values := map[string]int32{
		"acquired": stat.AcquiredConns(),
		"idle":     stat.IdleConns(),
		"total":    stat.TotalConns(),
		"max":      stat.MaxConns(),
	}
	for state, value := range values {
		ch <- prometheus.MustNewConstMetric(c.connections, prometheus.GaugeValue, float64(value), state)
	}
}

func New() *Metrics {
	m := &Metrics{
		registry:             prometheus.NewRegistry(),
		activeConsumerStates: make(map[consumerKey]string),
		producedMessages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "produced_messages_total",
			Help:      "Number of messages sent to Kafka.",
		}, []string{"topic", "result"}),
		produceDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "produce_duration_seconds",
			Help:      "Time spent sending a message to Kafka.",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"topic"}),
		consumedMessages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "consumed_messages_total",
			Help:      "Number of messages returned by a Redbus consumer.",
		}, []string{"topic", "group", "result"}),
		consumerProcessingDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "consumer_processing_duration_seconds",
			Help:      "Time spent waiting for a consumer to process one batch.",
			Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"topic", "group"}),
		consumerBatchSize: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "consumer_batch_size",
			Help:      "Number of messages in a batch sent to a consumer.",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 8),
		}, []string{"topic", "group"}),
		activeConsumers: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_consumers",
			Help:      "Current Redbus consumer streams by state.",
		}, []string{"topic", "group", "state"}),
		consumerConnections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "consumer_connections_total",
			Help:      "Number of consumer connection attempts.",
		}, []string{"topic", "group", "result"}),
		kafkaConsumerReconnects: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "kafka_consumer_reconnects_total",
			Help:      "Number of Kafka consumer reconnect cycles.",
		}, []string{"topic", "group", "reason"}),
		retryEnqueued: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retry_enqueued_total",
			Help:      "Number of messages submitted to the retry repository.",
		}, []string{"topic", "group", "result"}),
		retryAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retry_attempts_total",
			Help:      "Number of retry delivery attempts.",
		}, []string{"topic", "group", "outcome"}),
		retrySkipped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retry_skipped_total",
			Help:      "Number of due retries skipped before delivery.",
		}, []string{"topic", "group", "reason"}),
		retryRecords: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "retry_records",
			Help:      "Current retry repository records by state.",
		}, []string{"state"}),
		repeaterRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "repeater_runs_total",
			Help:      "Number of repeater iterations.",
		}, []string{"result"}),
		repeaterIterationDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "repeater_iteration_duration_seconds",
			Help:      "Duration of one repeater iteration.",
			Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}),
		repeaterIterationMessages: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "repeater_iteration_messages",
			Help:      "Number of due retry records selected by an iteration.",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
		}),
		grpcServerRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "grpc_server_requests_total",
			Help:      "Number of completed gRPC calls.",
		}, []string{"service", "method", "code"}),
		grpcServerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "grpc_server_duration_seconds",
			Help:      "Duration of completed gRPC calls.",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"service", "method"}),
		grpcServerActiveStreams: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "grpc_server_active_streams",
			Help:      "Current active gRPC streams.",
		}, []string{"service", "method"}),
	}
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.producedMessages,
		m.produceDuration,
		m.consumedMessages,
		m.consumerProcessingDuration,
		m.consumerBatchSize,
		m.activeConsumers,
		m.consumerConnections,
		m.kafkaConsumerReconnects,
		m.retryEnqueued,
		m.retryAttempts,
		m.retrySkipped,
		m.retryRecords,
		m.repeaterRuns,
		m.repeaterIterationDuration,
		m.repeaterIterationMessages,
		m.grpcServerRequests,
		m.grpcServerDuration,
		m.grpcServerActiveStreams,
	)
	m.SetRetryRecords(0, 0)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) RegisterDBPool(pool DBPoolStatsProvider) {
	m.registry.MustRegister(dbPoolCollector{
		pool: pool,
		connections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "db_pool", "connections"),
			"Current PostgreSQL pool connections by state.",
			[]string{"state"},
			nil,
		),
	})
}

func (m *Metrics) ObserveProduce(topic, result string, duration time.Duration) {
	m.producedMessages.WithLabelValues(topic, result).Inc()
	m.produceDuration.WithLabelValues(topic).Observe(duration.Seconds())
}

func (m *Metrics) ObserveConsumed(topic, group, result string, count int) {
	if count > 0 {
		m.consumedMessages.WithLabelValues(topic, group, result).Add(float64(count))
	}
}

func (m *Metrics) ObserveConsumerBatch(topic, group string, size int, duration time.Duration) {
	m.consumerBatchSize.WithLabelValues(topic, group).Observe(float64(size))
	m.consumerProcessingDuration.WithLabelValues(topic, group).Observe(duration.Seconds())
}

func (m *Metrics) ObserveConsumerConnection(topic, group, result string) {
	m.consumerConnections.WithLabelValues(topic, group, result).Inc()
}

func (m *Metrics) AddConsumer(topic, group, id, state string) {
	m.activeConsumerMu.Lock()
	defer m.activeConsumerMu.Unlock()
	key := consumerKey{topic: topic, group: group, id: id}
	if previous, ok := m.activeConsumerStates[key]; ok {
		m.activeConsumers.WithLabelValues(topic, group, previous).Dec()
	}
	m.activeConsumerStates[key] = state
	m.activeConsumers.WithLabelValues(topic, group, state).Inc()
}

func (m *Metrics) RemoveConsumer(topic, group, id string) {
	m.activeConsumerMu.Lock()
	defer m.activeConsumerMu.Unlock()
	key := consumerKey{topic: topic, group: group, id: id}
	state, ok := m.activeConsumerStates[key]
	if !ok {
		return
	}
	delete(m.activeConsumerStates, key)
	m.activeConsumers.WithLabelValues(topic, group, state).Dec()
}

func (m *Metrics) ChangeConsumerState(topic, group, id, current string) {
	m.activeConsumerMu.Lock()
	defer m.activeConsumerMu.Unlock()
	key := consumerKey{topic: topic, group: group, id: id}
	previous, ok := m.activeConsumerStates[key]
	if !ok || previous == current {
		return
	}
	m.activeConsumers.WithLabelValues(topic, group, previous).Dec()
	m.activeConsumers.WithLabelValues(topic, group, current).Inc()
	m.activeConsumerStates[key] = current
}

func (m *Metrics) ObserveKafkaReconnect(topic, group, reason string) {
	m.kafkaConsumerReconnects.WithLabelValues(topic, group, reason).Inc()
}

func (m *Metrics) ObserveRetryEnqueued(topic, group, result string) {
	m.retryEnqueued.WithLabelValues(topic, group, result).Inc()
}

func (m *Metrics) ObserveRetryAttempt(topic, group, outcome string) {
	m.retryAttempts.WithLabelValues(topic, group, outcome).Inc()
}

func (m *Metrics) ObserveRetrySkipped(topic, group, reason string) {
	m.retrySkipped.WithLabelValues(topic, group, reason).Inc()
}

func (m *Metrics) SetRetryRecords(pending, exhausted int) {
	m.retryRecords.WithLabelValues("pending").Set(float64(pending))
	m.retryRecords.WithLabelValues("exhausted").Set(float64(exhausted))
}

func (m *Metrics) ObserveRepeaterRun(result string, messages int, duration time.Duration) {
	m.repeaterRuns.WithLabelValues(result).Inc()
	m.repeaterIterationMessages.Observe(float64(messages))
	m.repeaterIterationDuration.Observe(duration.Seconds())
}

func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		startedAt := time.Now()
		response, err := handler(ctx, req)
		service, method := splitFullMethod(info.FullMethod)
		m.grpcServerRequests.WithLabelValues(service, method, status.Code(err).String()).Inc()
		m.grpcServerDuration.WithLabelValues(service, method).Observe(time.Since(startedAt).Seconds())
		return response, err
	}
}

func (m *Metrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startedAt := time.Now()
		service, method := splitFullMethod(info.FullMethod)
		m.grpcServerActiveStreams.WithLabelValues(service, method).Inc()
		defer m.grpcServerActiveStreams.WithLabelValues(service, method).Dec()
		err := handler(srv, stream)
		m.grpcServerRequests.WithLabelValues(service, method, status.Code(err).String()).Inc()
		m.grpcServerDuration.WithLabelValues(service, method).Observe(time.Since(startedAt).Seconds())
		return err
	}
}

func splitFullMethod(fullMethod string) (string, string) {
	trimmed := strings.TrimPrefix(fullMethod, "/")
	service, method, found := strings.Cut(trimmed, "/")
	if !found {
		return "unknown", trimmed
	}
	return service, method
}
