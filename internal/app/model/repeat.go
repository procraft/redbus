package model

import (
	"time"
)

type Repeat struct {
	Id             int64
	Topic          string
	Group          string
	ConsumerId     string
	MessageId      string
	Key            []byte
	Data           []byte
	Attempt        int
	RepeatStrategy *RepeatStrategy
	Error          string
	CreatedAt      time.Time
	StartedAt      time.Time
	FinishedAt     *time.Time
}

type RepeatList []*Repeat

var defaultRepeatStrategy = NewRepeatStrategy(5, RepeatCalculatorAnnual{Interval: 5 * time.Minute})

func (r *Repeat) ApplyNextAttempt() {
	var strategy = defaultRepeatStrategy
	if r.RepeatStrategy != nil {
		strategy = r.RepeatStrategy
	}
	if strategy.MaxAttempts >= r.Attempt {
		now := time.Now()
		r.FinishedAt = &now
		return
	}
	r.Attempt++
	r.StartedAt = strategy.GetNextStartedAt(r.Attempt)
}
