package model

import (
	"github.com/sergiusd/redbus/internal/pkg/runtime"
	"time"
)

type RepeatCalculatorEven struct {
	Interval time.Duration `json:"interval"`
}

func NewRepeatStrategyEven(maxAttempts int, interval time.Duration) *RepeatStrategy {
	return &RepeatStrategy{
		Kind:        RepeatKindEven,
		MaxAttempts: maxAttempts,
		EvenConfig: &RepeatCalculatorEven{
			Interval: interval,
		},
	}
}

func (c RepeatCalculatorEven) GetNextStartedAt(_ int) time.Time {
	return runtime.Now().Add(c.Interval)
}
