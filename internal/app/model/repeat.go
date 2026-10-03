package model

import (
	"fmt"
	"time"

	"github.com/prokraft/redbus/internal/pkg/runtime"
)

type Repeat struct {
	Id         int64
	Topic      TopicName
	Group      GroupName
	ConsumerId ConsumerId
	MessageId  string
	Key        *[]byte
	Data       []byte
	Headers    map[string]string
	Attempt    int
	Strategy   *RepeatStrategy
	Error      string
	// Deferred marks a pending retry whose last delivery the consumer postponed (retryLater):
	// it is waiting for its turn, not failing. A deferred repeat is never finished.
	Deferred   bool
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt *time.Time
}

type RepeatList []*Repeat

func (rl RepeatList) GroupByConsumerId() map[ConsumerId]RepeatList {
	ret := make(map[ConsumerId]RepeatList, len(rl))
	for _, r := range rl {
		if _, ok := ret[r.ConsumerId]; !ok {
			ret[r.ConsumerId] = make(RepeatList, 0, len(rl))
		}
		ret[r.ConsumerId] = append(ret[r.ConsumerId], r)
	}
	return ret
}

type TopicGroup struct {
	Topic TopicName
	Group GroupName
}

func (tg TopicGroup) KafkaGroupId() string {
	return fmt.Sprintf("%s-%s", tg.Group, tg.Topic)
}

func (r *Repeat) SetZeroAttempt(defaultStrategy *RepeatStrategy) {
	var strategy = defaultStrategy
	if r.Strategy != nil {
		strategy = r.Strategy
	}
	r.StartedAt = strategy.GetNextStartedAt(r.Attempt)
	r.Attempt = 0
}

// ApplyNextAttempt records an ordinary failure: it spends an attempt and clears Deferred.
func (r *Repeat) ApplyNextAttempt(defaultStrategy *RepeatStrategy) {
	r.Deferred = false
	var strategy = defaultStrategy
	if r.Strategy != nil {
		strategy = r.Strategy
	}
	if strategy.MaxAttempts <= r.Attempt {
		now := time.Now()
		r.FinishedAt = &now
		return
	}
	r.Attempt++
	r.StartedAt = strategy.GetNextStartedAt(r.Attempt)
}

// ApplyFailure schedules another delivery. A deferred result (retryLater) keeps the attempt, cannot
// exhaust the repeat and marks it Deferred; an ordinary failure spends an attempt and clears the
// mark. A positive consumer delay overrides the configured strategy without changing that strategy.
func (r *Repeat) ApplyFailure(defaultStrategy *RepeatStrategy, retryLater bool, retryAfter time.Duration) {
	if retryLater {
		var strategy = defaultStrategy
		if r.Strategy != nil {
			strategy = r.Strategy
		}
		r.Deferred = true
		r.FinishedAt = nil
		// A malformed/legacy zero delay still uses the configured strategy and cannot busy-loop.
		r.StartedAt = strategy.GetNextStartedAt(r.Attempt + 1)
	} else {
		r.ApplyNextAttempt(defaultStrategy)
	}
	if r.FinishedAt == nil && retryAfter > 0 {
		r.StartedAt = runtime.Now().Add(retryAfter)
	}
}

type TopicGroupList []TopicGroup

func (tg TopicGroupList) String(delimiter string) []string {
	ret := make([]string, 0, len(tg))
	for _, item := range tg {
		ret = append(ret, fmt.Sprintf("%s%s%s", item.Topic, delimiter, item.Group))
	}
	return ret
}

// RepeatStatItem counts the retries of one topic/group. AllCount includes both FailedCount
// (finished) and DeferredCount (pending, postponed by the consumer); LastError ignores deferrals.
type RepeatStatItem struct {
	Topic              string
	Group              string
	AllCount           int
	FailedCount        int
	DeferredCount      int
	LastError          string
	LastDeferredReason string
	Errors             []RepeatErrorStat
}

// RepeatCount is the repository-wide retry summary, split the same way as RepeatStatItem.
type RepeatCount struct {
	All      int
	Failed   int
	Deferred int
}

// RepeatErrorStat describes one error class (see ErrorClass). Error is the class and Sample is
// the exact message of its most recent failure.
type RepeatErrorStat struct {
	Error         string
	Sample        string
	FailedCount   int
	FirstFailedAt time.Time
	LastFailedAt  time.Time
}

type RepeatStat = []RepeatStatItem

// RepeatTriageStat is a read-only snapshot of retries that became terminally failed
// inside one half-open time window. It deliberately excludes active retries.
type RepeatTriageStat struct {
	Since time.Time
	Until time.Time
	List  []RepeatTriageStatItem
}

type RepeatTriageStatItem struct {
	Topic       string
	Group       string
	FailedCount int
	Errors      []RepeatErrorStat
}
