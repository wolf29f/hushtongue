package frontend

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":       {Data: []byte("index")},
		"favicon.svg":      {Data: []byte("icon")},
		"assets/app-1a.js": {Data: []byte("app")},
	}
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
		wantCache  string
	}{
		{"root", http.MethodGet, "/", http.StatusOK, "index", ""},
		{"page of the front", http.MethodGet, "/login/abc", http.StatusOK, "index", ""},
		{"file", http.MethodGet, "/favicon.svg", http.StatusOK, "icon", ""},
		{"asset", http.MethodGet, "/assets/app-1a.js", http.StatusOK, "app", "public, max-age=31536000, immutable"},
		{"directory", http.MethodGet, "/assets/", http.StatusOK, "index", ""},
		{"head", http.MethodHead, "/login/abc", http.StatusOK, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			Handler(fsys).ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := w.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
		})
	}
}
