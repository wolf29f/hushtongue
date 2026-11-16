package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /read", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, "read")
	})
	mux.HandleFunc("POST /write", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, http.StatusOK, "written")
	})
	handler := Handler(mux)

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		status      int
		code        ErrorCode
	}{
		{"unknown route", http.MethodGet, "/nope", "", http.StatusNotFound, CodeNotFound},
		{"wrong method", http.MethodPost, "/read", "", http.StatusMethodNotAllowed, CodeMethodNotAllowed},
		{"mutation without JSON", http.MethodPost, "/write", "text/plain", http.StatusUnsupportedMediaType, CodeUnsupportedMediaType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(tt.method, tt.path, nil)
			r.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			assertError(t, w, tt.status, tt.code)
		})
	}

	t.Run("serves a mutation in JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/write", nil)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})

	t.Run("serves a read without Content-Type", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/read", nil))

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})
}

func TestIsJSON(t *testing.T) {
	tests := map[string]bool{
		"application/json":                  true,
		"application/json; charset=utf-8":   true,
		"Application/JSON":                  true,
		"text/plain":                        false,
		"application/x-www-form-urlencoded": false,
		"":                                  false,
	}
	for contentType, want := range tests {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Content-Type", contentType)
		if got := isJSON(r); got != want {
			t.Errorf("isJSON(%q) = %v, want %v", contentType, got, want)
		}
	}
}
