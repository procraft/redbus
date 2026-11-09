package queuehealth

import (
	"context"
	"sync"
	"time"

	"github.com/procraft/redbus/internal/app/model"
)

const RefreshInterval = 30 * time.Second
const RefreshTimeout = 20 * time.Second

type Kafka interface {
	GetQueueSnapshot(context.Context, model.TopicGroupList) ([]model.QueueSnapshot, error)
}
type Groups interface{ GetConsumerTopicGroupList() model.TopicGroupList }
type Retry interface {
	GetQueueStats(context.Context) ([]model.RetryQueueStat, error)
}
type Metrics interface {
	SetQueueSnapshots(string, []model.QueueSnapshot, error)
	SetRetryRecords(int, int, int)
}

// Service refreshes I/O-backed observations outside HTTP scrapes. Known groups
// remain monitored after a consumer disconnects, including when it stops progress.
type Service struct {
	kafka   Kafka
	groups  Groups
	retry   Retry
	metrics Metrics
	mu      sync.Mutex
	known   map[model.TopicGroup]struct{}
}

func New(kafka Kafka, groups Groups, retry Retry, metrics Metrics) *Service {
	return &Service{kafka: kafka, groups: groups, retry: retry, metrics: metrics, known: make(map[model.TopicGroup]struct{})}
}

func (s *Service) monitoredGroups() model.TopicGroupList {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, group := range s.groups.GetConsumerTopicGroupList() {
		s.known[group] = struct{}{}
	}
	result := make(model.TopicGroupList, 0, len(s.known))
	for group := range s.known {
		result = append(result, group)
	}
	return result
}

func (s *Service) RefreshKafka(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, RefreshTimeout)
	defer cancel()
	groups := s.monitoredGroups()
	values, err := s.kafka.GetQueueSnapshot(ctx, groups)
	// Whole-source failures still publish unavailable health for newly seen groups.
	found := make(map[model.TopicGroup]bool, len(values))
	for _, value := range values {
		found[model.TopicGroup{Topic: value.Topic, Group: value.Group}] = true
	}
	for _, group := range groups {
		if !found[group] {
			values = append(values, model.QueueSnapshot{Topic: group.Topic, Group: group.Group})
		}
	}
	s.metrics.SetQueueSnapshots("kafka", values, err)
	return err
}

func (s *Service) RefreshRetry(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, RefreshTimeout)
	defer cancel()
	groups := s.monitoredGroups()
	stats, err := s.retry.GetQueueStats(ctx)
	values := make([]model.QueueSnapshot, 0, len(stats)+len(groups))
	found := make(map[model.TopicGroup]bool, len(stats))
	all, failed, deferred := 0, 0, 0
	if err == nil {
		for _, stat := range stats {
			all, failed, deferred = all+stat.All, failed+stat.Failed, deferred+stat.Deferred
			value := model.QueueSnapshot{Topic: stat.Topic, Group: stat.Group, Valid: true}
			if stat.Oldest != nil {
				value.Oldest = *stat.Oldest
			}
			values = append(values, value)
			found[model.TopicGroup{Topic: stat.Topic, Group: stat.Group}] = true
		}
		s.metrics.SetRetryRecords(all-failed-deferred, deferred, failed)
	}
	for _, group := range groups {
		if !found[group] {
			values = append(values, model.QueueSnapshot{Topic: group.Topic, Group: group.Group, Valid: err == nil})
		}
	}
	s.metrics.SetQueueSnapshots("retry", values, err)
	return err
}
