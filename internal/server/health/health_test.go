package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (p fakePinger) Ping(context.Context) error {
	return p.err
}

func TestLive(t *testing.T) {
	w := httptest.NewRecorder()
	New(fakePinger{err: errors.New("database down")}).Live(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := w.Body.String(); w.Code != http.StatusOK || got != `{"status":"ok"}` {
		t.Errorf("live = %d %s, want 200 ok whatever the database", w.Code, got)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"ready when the database answers", nil, http.StatusOK},
		{"unavailable when the database doesn't answer", errors.New("database down"), http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			New(fakePinger{err: tt.err}).Ready(w, httptest.NewRequest(http.MethodGet, "/", nil))

			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}
