package model

import (
	"time"

	"github.com/prokraft/redbus/internal/pkg/runtime"
)

type RepeatCalculatorEven struct {
	Interval Duration `json:"interval"`
}

func NewRepeatStrategyEven(maxAttempts int, interval Duration) *RepeatStrategy {
	return &RepeatStrategy{
		Kind:        RepeatKindEven,
		MaxAttempts: maxAttempts,
		EvenConfig: &RepeatCalculatorEven{
			Interval: interval,
		},
	}
}

func (c RepeatCalculatorEven) GetNextStartedAt(_ int) time.Time {
	return runtime.Now().Add(c.Interval.Duration)
}
