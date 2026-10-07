package outbox

import (
	"sync"
	"time"
)

// errorThrottle lets one report per key through every interval and counts the ones it suppresses
// in between, so a topic that fails on every pg_notify logs once a minute instead of on every pass.
type errorThrottle struct {
	interval time.Duration
	now      func() time.Time

	mu   sync.Mutex
	keys map[string]*throttleState
}

type throttleState struct {
	last       time.Time
	suppressed int
}

func newErrorThrottle(interval time.Duration, now func() time.Time) *errorThrottle {
	return &errorThrottle{interval: interval, now: now, keys: map[string]*throttleState{}}
}

// report calls emit with the number of reports suppressed since the previous emitted one, or
// counts this report as suppressed.
func (t *errorThrottle) report(key string, emit func(suppressed int)) {
	t.mu.Lock()
	now := t.now()
	st, ok := t.keys[key]
	if ok && now.Sub(st.last) < t.interval {
		st.suppressed++
		t.mu.Unlock()
		return
	}
	if !ok {
		st = &throttleState{}
		t.keys[key] = st
	}
	suppressed := st.suppressed
	st.last, st.suppressed = now, 0
	t.mu.Unlock()
	emit(suppressed)
}
