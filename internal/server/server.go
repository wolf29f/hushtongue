// Package server serves the hushtongue API.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

// Server holds the dependencies of the handlers.
type Server struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// New creates the server on top of the database db.
func New(db *pgxpool.Pool, logger *slog.Logger) *Server {
	return &Server{db: db, logger: logger}
}

// HTTPServer returns the HTTP server serving the API on addr.
func (s *Server) HTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelError),
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health/live", s.handleLive)
	mux.HandleFunc("GET /api/health/ready", s.handleReady)
	return http.MaxBytesHandler(jsonFallback(mux), maxBodySize)
}
