// Package server serves the hushtongue API.
package server

import (
	"log/slog"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 2 * time.Minute

	// maxBodySize is the limit of a request body, sized for the import of a
	// v1 dictionary.
	maxBodySize = 10 << 20 // 10 MB
)

// New returns the HTTP server listening on addr.
func New(addr string, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

func routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health/live", handleLive)
	return http.MaxBytesHandler(jsonFallback(mux), maxBodySize)
}

// handleLive answers as soon as the server runs, whatever the state of the
// database: restarting the server wouldn't fix it.
func handleLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
