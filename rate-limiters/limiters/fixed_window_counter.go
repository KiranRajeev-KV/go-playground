package limiters

import (
	"sync"
	"time"
)

const (
	FixedWindowCounterLimit  = 5
	FixedWindowCounterWindow = 60 * time.Second
)

type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Reason     string
}

type FixedWindowCounter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	windows map[string]fixedWindowState
}

type fixedWindowState struct {
	windowStart time.Time
	count       int
}

func NewFixedWindowCounter() *FixedWindowCounter {
	return &FixedWindowCounter{
		limit:   FixedWindowCounterLimit,
		window:  FixedWindowCounterWindow,
		windows: make(map[string]fixedWindowState),
	}
}

func (l *FixedWindowCounter) Allow(key string, now time.Time) Decision {
	windowStart := now.Truncate(l.window)
	windowEnd := windowStart.Add(l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	state, ok := l.windows[key]
	if !ok || !state.windowStart.Equal(windowStart) {
		state = fixedWindowState{
			windowStart: windowStart,
			count:       0,
		}
	}

	if state.count >= l.limit {
		return Decision{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: windowEnd.Sub(now),
			Reason:     "fixed_window_exhausted",
		}
	}

	state.count++
	l.windows[key] = state

	return Decision{
		Allowed:    true,
		Remaining:  l.limit - state.count,
		RetryAfter: 0,
		Reason:     "fixed_window_allow",
	}
}
