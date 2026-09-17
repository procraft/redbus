package model

import (
	"testing"
	"time"
)

func TestErrorClass(t *testing.T) {
	for _, tc := range []struct {
		message string
		class   string
	}{
		{
			message: "FAILED_PRECONDITION: Sending task 58964360 don't have any message.",
			class:   "FAILED_PRECONDITION: Sending task <N> don't have any message.",
		},
		{message: "order_123 not found (attempt 2 of 5)", class: "order_<N> not found (attempt <N> of <N>)"},
		{message: "user 550e8400-E29B-41d4-a716-446655440000 is blocked", class: "user <UUID> is blocked"},
		{message: "trace 4bf92f3577b34da6a3ce929d0e0e4736 at 0x1f2e", class: "trace <HEX> at <HEX>"},
		{message: "deadline 2026-09-17T04:37:43.123+03:00 exceeded", class: "deadline <TIME> exceeded"},
		{message: "context deadline exceeded after 1m30.5s", class: "context deadline exceeded after <DURATION>"},
		{message: "amount -12.50 exceeds limit", class: "amount -<N>.<N> exceeds limit"},
		{message: "invalid int64 in sha256 v2 header", class: "invalid int64 in sha256 v2 header"},
		{message: "ошибка задачи58964360", class: "ошибка задачи58964360"},
		{message: "задача 58964360", class: "задача <N>"},
		{message: "unauthenticated", class: "unauthenticated"},
		{message: "", class: ""},
	} {
		t.Run(tc.message, func(t *testing.T) {
			class := ErrorClass(tc.message)
			if class != tc.class {
				t.Fatalf("unexpected class: got %q want %q", class, tc.class)
			}
			if again := ErrorClass(class); again != class {
				t.Fatalf("class must be stable: got %q from %q", again, class)
			}
		})
	}
}

func TestGroupRepeatErrorStatsByClass(t *testing.T) {
	at := time.Date(2026, 9, 17, 4, 0, 0, 0, time.UTC)
	stats := GroupRepeatErrorStatsByClass([]RepeatErrorStat{
		{Error: "invalid link", FailedCount: 2, FirstFailedAt: at, LastFailedAt: at.Add(time.Minute)},
		{Error: "task 1 has no message", FailedCount: 1, FirstFailedAt: at.Add(time.Hour), LastFailedAt: at.Add(time.Hour)},
		{Error: "task 22 has no message", FailedCount: 1, FirstFailedAt: at.Add(3 * time.Hour), LastFailedAt: at.Add(3 * time.Hour)},
		{Error: "task 333 has no message", FailedCount: 1, FirstFailedAt: at.Add(-time.Hour), LastFailedAt: at.Add(2 * time.Hour)},
	})

	if len(stats) != 2 {
		t.Fatalf("unexpected class count: %#v", stats)
	}
	merged := stats[0]
	if merged.Error != "task <N> has no message" || merged.FailedCount != 3 {
		t.Fatalf("largest class must be first and summed: %#v", merged)
	}
	if !merged.FirstFailedAt.Equal(at.Add(-time.Hour)) || !merged.LastFailedAt.Equal(at.Add(3*time.Hour)) {
		t.Fatalf("class must span all failures: %#v", merged)
	}
	if merged.Sample != "task 22 has no message" {
		t.Fatalf("sample must be the most recent failure: %q", merged.Sample)
	}
	if stats[1].Error != "invalid link" || stats[1].Sample != "invalid link" || stats[1].FailedCount != 2 {
		t.Fatalf("single message class must be kept: %#v", stats[1])
	}
}

func TestErrorsOfClassAcceptsClassOrExactMessage(t *testing.T) {
	messages := []string{"task 1 has no message", "task 22 has no message", "task has no message", "invalid link"}
	for _, filter := range []string{"task <N> has no message", "task 7 has no message"} {
		errors := ErrorsOfClass(messages, filter)
		if len(errors) != 2 || errors[0] != "task 1 has no message" || errors[1] != "task 22 has no message" {
			t.Fatalf("unexpected errors for %q: %#v", filter, errors)
		}
	}
}
