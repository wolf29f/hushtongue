package server

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := newLimiter(2, time.Minute, func() time.Time { return now })

	for i := range 2 {
		if ok, _ := l.allow("a"); !ok {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	now = now.Add(20 * time.Second)
	if ok, wait := l.allow("a"); ok || wait != 40*time.Second {
		t.Errorf("third attempt = %v, %v, want refused for 40s", ok, wait)
	}
	if ok, _ := l.allow("b"); !ok {
		t.Error("attempt of another key refused")
	}

	now = now.Add(40 * time.Second)
	if ok, _ := l.allow("a"); !ok {
		t.Error("attempt of the next period refused")
	}

	now = now.Add(time.Minute)
	l.allow("c")
	if len(l.windows) != 1 {
		t.Errorf("%d windows kept, want only c's, the others being over", len(l.windows))
	}
}
