package model

import (
	"time"
)

type Repeat struct {
	Id         int64
	Topic      string
	Group      string
	ConsumerId string
	MessageId  string
	Key        []byte
	Data       []byte
	Attempt    int
	Strategy   *RepeatStrategy
	Error      string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt *time.Time
}

type RepeatList []*Repeat

type TopicGroup struct {
	Topic string
	Group string
}

type TopicGroupList = []TopicGroup

var defaultRepeatStrategy = NewRepeatStrategy(5, RepeatCalculatorEven{Interval: 5 * time.Minute})

func (r *Repeat) ApplyNextAttempt() {
	var strategy = defaultRepeatStrategy
	if r.Strategy != nil {
		strategy = r.Strategy
	}
	if strategy.MaxAttempts >= r.Attempt {
		now := time.Now()
		r.FinishedAt = &now
		return
	}
	r.Attempt++
	r.StartedAt = strategy.GetNextStartedAt(r.Attempt)
}
