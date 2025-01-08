package model

import (
	"fmt"
	"time"
)

type Repeat struct {
	Id         int64
	Topic      TopicName
	Group      GroupName
	ConsumerId ConsumerId
	MessageId  string
	Key        *[]byte
	Data       []byte
	Attempt    int
	Strategy   *RepeatStrategy
	Error      string
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

func (r *Repeat) SetZeroAttempt(defaultStrategy *RepeatStrategy) {
	var strategy = defaultStrategy
	if r.Strategy != nil {
		strategy = r.Strategy
	}
	r.StartedAt = strategy.GetNextStartedAt(r.Attempt)
	r.Attempt = 0
}

func (r *Repeat) ApplyNextAttempt(defaultStrategy *RepeatStrategy) {
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

type TopicGroupList []TopicGroup

func (tg TopicGroupList) String(delimiter string) []string {
	ret := make([]string, 0, len(tg))
	for _, item := range tg {
		ret = append(ret, fmt.Sprintf("%s%s%s", item.Topic, delimiter, item.Group))
	}
	return ret
}

type RepeatStatItem struct {
	Topic       string
	Group       string
	AllCount    int
	FailedCount int
	LastError   string
}

type RepeatStat = []RepeatStatItem
