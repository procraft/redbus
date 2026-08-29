package model

import (
	"fmt"
	"time"
)

type RepeatStrategy struct {
	Kind              RepeatKind                   `json:"kind"`
	MaxAttempts       int                          `json:"maxAttempts"`
	EvenConfig        *RepeatCalculatorEven        `json:"evenConfig,omitempty"`
	ProgressiveConfig *RepeatCalculatorProgressive `json:"progressiveConfig,omitempty"`
}

func (m *RepeatStrategy) String() string {
	if m == nil {
		return "default"
	}
	switch m.Kind {
	case RepeatKindEven:
		return fmt.Sprintf("even · %d attempts · every %s", m.MaxAttempts, m.EvenConfig.Interval)
	case RepeatKindProgressive:
		return fmt.Sprintf(
			"progressive · %d attempts · every %s × %.2g",
			m.MaxAttempts,
			m.ProgressiveConfig.Interval,
			m.ProgressiveConfig.Multiplier,
		)
	default:
		return string(m.Kind)
	}
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
		return RepeatCalculatorProgressive{Interval: m.ProgressiveConfig.Interval, Multiplier: m.ProgressiveConfig.Multiplier}
	default:
		panic("Unsupported repeat strategy " + m.Kind)
	}
}

func (m *RepeatStrategy) GetNextStartedAt(attempt int) time.Time {
	return m.getCalculator().GetNextStartedAt(attempt)
}
