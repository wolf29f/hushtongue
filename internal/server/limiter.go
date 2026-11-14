package server

import (
	"sync"
	"time"
)

// limiter allows limit attempts per key in each period, from the first
// attempt of the period.
type limiter struct {
	limit  int
	period time.Duration
	now    func() time.Time

	mu      sync.Mutex
	windows map[string]window
}

type window struct {
	start    time.Time
	attempts int
}

func newLimiter(limit int, period time.Duration, now func() time.Time) *limiter {
	return &limiter{limit: limit, period: period, now: now, windows: make(map[string]window)}
}

// allow counts an attempt of key. It returns false, with the time to wait,
// when key has used up its attempts.
func (l *limiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	for k, w := range l.windows {
		if now.Sub(w.start) >= l.period {
			delete(l.windows, k)
		}
	}

	w, ok := l.windows[key]
	if !ok {
		w = window{start: now}
	}
	if w.attempts >= l.limit {
		return false, w.start.Add(l.period).Sub(now)
	}
	w.attempts++
	l.windows[key] = w
	return true, 0
}
