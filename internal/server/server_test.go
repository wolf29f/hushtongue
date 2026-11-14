package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"live", http.MethodGet, "/api/health/live", http.StatusOK, `{"status":"ok"}`},
		{"unknown route", http.MethodGet, "/api/nope", http.StatusNotFound,
			`{"error":"not_found","message":"Ressource introuvable."}`},
		{"wrong method", http.MethodPost, "/api/health/live", http.StatusMethodNotAllowed,
			`{"error":"method_not_allowed","message":"Méthode non autorisée."}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			routes().ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := strings.TrimSpace(w.Body.String()); got != tt.wantBody {
				t.Errorf("body = %s, want %s", got, tt.wantBody)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusNotFound, codeNotFound, "Ressource introuvable.")

	var body apiError
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if w.Code != http.StatusNotFound || body != (apiError{Code: codeNotFound, Message: "Ressource introuvable."}) {
		t.Errorf("writeError() = %d %+v", w.Code, body)
	}
}
