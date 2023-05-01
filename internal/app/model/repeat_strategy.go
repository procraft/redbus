package model

import (
	"time"
)

type RepeatStrategy struct {
	Kind              RepeatKind                   `json:"kind"`
	MaxAttempts       int                          `json:"maxAttempts"`
	EvenConfig        *RepeatCalculatorEven        `json:"evenConfig,omitempty"`
	ProgressiveConfig *RepeatCalculatorProgressive `json:"progressiveConfig,omitempty"`
}

type RepeatKind string

const (
	RepeatKindEven        RepeatKind = "even"
	RepeatKindProgressive            = "progressive"
)

type IRepeatCalculator interface {
	GetNextStartedAt(attempt int) time.Time
}

func (m *RepeatStrategy) getCalculator() IRepeatCalculator {
	switch m.Kind {
	case RepeatKindEven:
		return RepeatCalculatorEven{Interval: m.EvenConfig.Interval}
	case RepeatKindProgressive:
		return RepeatCalculatorProgressive{Interval: m.EvenConfig.Interval, Multiplier: m.ProgressiveConfig.Multiplier}
	default:
		panic("Unsupported repeat strategy " + m.Kind)
	}
}

func (m *RepeatStrategy) GetNextStartedAt(attempt int) time.Time {
	return m.getCalculator().GetNextStartedAt(attempt)
}
