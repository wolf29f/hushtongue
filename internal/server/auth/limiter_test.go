package auth

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestLimiter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newLimiter(2, time.Minute)

		for i := range 2 {
			if ok, _ := l.allow("a"); !ok {
				t.Fatalf("attempt %d refused", i+1)
			}
		}
		time.Sleep(20 * time.Second)
		if ok, wait := l.allow("a"); ok || wait != 40*time.Second {
			t.Errorf("third attempt = %v, %v, want refused for 40s", ok, wait)
		}
		if ok, _ := l.allow("b"); !ok {
			t.Error("attempt of another key refused")
		}

		time.Sleep(40 * time.Second)
		if ok, _ := l.allow("a"); !ok {
			t.Error("attempt of the next period refused")
		}

		time.Sleep(time.Minute)
		l.allow("c")
		if len(l.windows) != 1 {
			t.Errorf("%d windows kept, want only c's, the others being over", len(l.windows))
		}
	})
}
