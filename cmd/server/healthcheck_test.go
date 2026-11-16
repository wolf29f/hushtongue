package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalAddr(t *testing.T) {
	tests := map[string]string{
		":8080":          "127.0.0.1:8080",
		"0.0.0.0:8080":   "127.0.0.1:8080",
		"[::]:8080":      "127.0.0.1:8080",
		"127.0.0.1:9000": "127.0.0.1:9000",
		"[::1]:9000":     "[::1]:9000",
	}
	for addr, want := range tests {
		if got := localAddr(addr); got != want {
			t.Errorf("localAddr(%q) = %q, want %q", addr, got, want)
		}
	}
}

func TestHealthcheck(t *testing.T) {
	t.Run("succeeds on 200", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer srv.Close()

		if err := healthcheck(strings.TrimPrefix(srv.URL, "http://")); err != nil {
			t.Errorf("healthcheck() failed: %v", err)
		}
	})

	t.Run("fails on 503", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		if err := healthcheck(strings.TrimPrefix(srv.URL, "http://")); err == nil {
			t.Error("healthcheck() succeeded, want an error")
		}
	})
}
