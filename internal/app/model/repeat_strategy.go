package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sergiusd/redbus/internal/pkg/runtime"
)

type IRepeatCalculator interface {
	GetNextStartedAt(attempt int) time.Time
}

type RepeatStrategy struct {
	Kind        RepeatKind
	MaxAttempts int
	calculator  IRepeatCalculator
}

type repeatStrategyConfig[T RepeatCalculator] struct {
	Config T `json:"config"`
}

func NewRepeatStrategy(maxAttempts int, calculator IRepeatCalculator) *RepeatStrategy {
	return &RepeatStrategy{MaxAttempts: maxAttempts, calculator: calculator}
}

func NewEvenRepeatStrategy(maxAttempts int, interval time.Duration) *RepeatStrategy {
	return &RepeatStrategy{
		MaxAttempts: maxAttempts,
		calculator: RepeatCalculatorEven{
			Interval: interval,
		},
	}
}

func NewProgressiveRepeatStrategy(maxAttempts int, interval time.Duration, multiplier float32) *RepeatStrategy {
	return &RepeatStrategy{
		MaxAttempts: maxAttempts,
		calculator: RepeatCalculatorProgressive{
			Interval:   interval,
			Multiplier: multiplier,
		},
	}
}

func unmarshalRepeatStrategyConfig[T RepeatCalculator](b []byte) (T, error) {
	var tmp repeatStrategyConfig[T]
	err := json.Unmarshal(b, &tmp)
	return tmp.Config, err
}

type RepeatKind string

const (
	RepeatKindEven        RepeatKind = "even"
	RepeatKindProgressive            = "progressive"
)

func (m *RepeatStrategy) UnmarshalJSON(b []byte) error {
	var tmp struct {
		Kind        RepeatKind `json:"kind"`
		MaxAttempts int        `json:"max"`
	}
	err := json.Unmarshal(b, &tmp)

	if err != nil {
		return err
	}

	var calculator IRepeatCalculator
	switch tmp.Kind {
	case RepeatKindEven:
		calculator, err = unmarshalRepeatStrategyConfig[RepeatCalculatorEven](b)
	case RepeatKindProgressive:
		calculator, err = unmarshalRepeatStrategyConfig[RepeatCalculatorProgressive](b)
	default:
		err = fmt.Errorf("Unknown repeat kind: %v", m.Kind)
	}

	if err != nil {
		return err
	}

	m.Kind = tmp.Kind
	m.MaxAttempts = tmp.MaxAttempts
	m.calculator = calculator
	return nil
}

func (m *RepeatStrategy) GetNextStartedAt(attempt int) time.Time {
	return m.calculator.GetNextStartedAt(attempt)
}

type RepeatCalculator interface {
	RepeatCalculatorEven | RepeatCalculatorProgressive
}

type RepeatCalculatorEven struct {
	Interval time.Duration `json:"interval"`
}

func (c RepeatCalculatorEven) GetNextStartedAt(_ int) time.Time {
	return runtime.Now().Add(c.Interval)
}

type RepeatCalculatorProgressive struct {
	Interval   time.Duration `json:"interval"`
	Multiplier float32       `json:"multiplier"`
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
