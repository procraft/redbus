package model

import (
	"github.com/prokraft/redbus/internal/pkg/runtime"
	"time"
)

type RepeatCalculatorProgressive struct {
	Interval   time.Duration `json:"interval"`
	Multiplier float32       `json:"multiplier"`
}

func NewRepeatStrategyStrategy(maxAttempts int, interval time.Duration, multiplier float32) *RepeatStrategy {
	return &RepeatStrategy{
		Kind:        RepeatKindProgressive,
		MaxAttempts: maxAttempts,
		ProgressiveConfig: &RepeatCalculatorProgressive{
			Interval:   interval,
			Multiplier: multiplier,
		},
	}
}

func (c RepeatCalculatorProgressive) GetNextStartedAt(attempt int) time.Time {
	var m float32 = 1
	if c.Multiplier > 1 {
		m = c.Multiplier
	}
	var n float32 = 1.0
	for i := 2; i <= attempt; i++ {
		n = n*m + 1
	}
	return runtime.Now().Add(c.Interval * time.Duration(n))
}
