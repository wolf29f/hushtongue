// Package middleware wraps the API's handlers: JSON responses for the mux,
// protection against cross-site requests and the sessions of the logged in
// users.
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server/auth"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// sessionExtendAfter spaces out the writes extending a session.
const sessionExtendAfter = 24 * time.Hour

// Store is the storage of the sessions the middlewares need.
type Store interface {
	Session(ctx context.Context, tokenHash []byte) (storage.Session, error)
	ExtendSession(ctx context.Context, tokenHash []byte, lifetime time.Duration) error
}

// Sessions checks the session of each request.
type Sessions struct {
	secureCookie bool
	store        Store
}

// New creates the middlewares on top of store.
func New(cfg config.Config, store Store) *Sessions {
	return &Sessions{secureCookie: cfg.SecureCookies(), store: store}
}

// RequireSession answers with a 401 without a session. Otherwise, it puts
// the session's user in the request context, read by httpapi.User, and calls
// next. It extends the session at most once a day.
func (s *Sessions) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookie)
		if err != nil {
			httpapi.WriteError(w, http.StatusUnauthorized, httpapi.CodeUnauthenticated, "Connexion requise.")
			return
		}
		tokenHash := auth.HashToken(cookie.Value)
		session, err := s.store.Session(r.Context(), tokenHash)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				httpapi.WriteError(w, http.StatusUnauthorized, httpapi.CodeUnauthenticated, "Connexion requise.")
				return
			}
			httpapi.WriteInternalError(w, r, err)
			return
		}

		lifetime := auth.SessionLifetime(session.User.Role)
		if lifetime-session.ExpiresIn > sessionExtendAfter {
			if err := s.store.ExtendSession(r.Context(), tokenHash, lifetime); err != nil {
				slog.Warn("Extending session failed", "error", err)
			} else {
				auth.SetSessionCookie(w, cookie.Value, lifetime, s.secureCookie)
			}
		}
		next.ServeHTTP(w, r.WithContext(httpapi.WithUser(r.Context(), session.User)))
	})
}

// RequireGM answers with a 403 when the user isn't the GM.
func (s *Sessions) RequireGM(next http.Handler) http.Handler {
	return s.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpapi.User(r.Context()).Role != storage.RoleGM {
			httpapi.WriteError(w, http.StatusForbidden, httpapi.CodeForbidden, "Réservé au MJ.")
			return
		}
		next.ServeHTTP(w, r)
	}))
}
