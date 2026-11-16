package httpapi

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusNotFound, CodeNotFound, "Ressource introuvable.")

	want := `{"error":"not_found","message":"Ressource introuvable."}`
	if got := w.Body.String(); w.Code != http.StatusNotFound || got != want {
		t.Errorf("WriteError() = %d %s, want 404 %s", w.Code, got, want)
	}
}

func assertError(t *testing.T, w *httptest.ResponseRecorder, status int, code ErrorCode) {
	t.Helper()
	var body Error
	if err := json.UnmarshalRead(w.Body, &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if w.Code != status || body.Code != code {
		t.Errorf("response = %d %s, want %d %s", w.Code, body.Code, status, code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func TestWriteServiceError(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   ErrorCode
	}{
		{storage.ErrNotFound, http.StatusNotFound, CodeNotFound},
		{storage.ErrNameTaken, http.StatusConflict, CodeNameTaken},
		{errors.New("boom"), http.StatusInternalServerError, CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteServiceError(w, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)

			assertError(t, w, tt.status, tt.code)
		})
	}
}

func TestPathID(t *testing.T) {
	mux := http.NewServeMux()
	var got int
	mux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		if id, ok := PathID(w, r); ok {
			got = id
		}
	})

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/12", nil))
	if got != 12 {
		t.Errorf("PathID() = %d, want 12", got)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/abc", nil))
	assertError(t, w, http.StatusNotFound, CodeNotFound)
}

func TestReadJSON(t *testing.T) {
	type request struct {
		Name string `json:"name"`
	}

	t.Run("decodes the body", func(t *testing.T) {
		var body request
		ok := ReadJSON(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name": "mj"}`)), &body)
		if !ok || body.Name != "mj" {
			t.Errorf("ReadJSON() = %v %+v, want mj", ok, body)
		}
	})

	t.Run("refuses invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		var body request
		if ReadJSON(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`)), &body) {
			t.Fatal("ReadJSON() = true, want false")
		}
		assertError(t, w, http.StatusBadRequest, CodeInvalidRequest)
	})

	t.Run("refuses a body over the limit", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name": "mj"}`))
		r.Body = http.MaxBytesReader(w, r.Body, 4)
		var body request
		if ReadJSON(w, r, &body) {
			t.Fatal("ReadJSON() = true, want false")
		}
		assertError(t, w, http.StatusRequestEntityTooLarge, CodeRequestTooLarge)
	})
}
