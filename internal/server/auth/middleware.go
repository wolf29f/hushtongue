package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/wolf29f/hushtongue/internal/server/httpapi"
	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// UserHandler handles a request of the logged in user.
type UserHandler func(w http.ResponseWriter, r *http.Request, user storage.User)

// RequireSession answers with a 401 without a session, and calls next with
// the session's user otherwise. It extends the session at most once a day.
func (h *Handlers) RequireSession(next UserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			httpapi.WriteError(w, http.StatusUnauthorized, httpapi.CodeUnauthenticated, "Connexion requise.")
			return
		}
		tokenHash := HashToken(cookie.Value)
		session, err := h.store.Session(r.Context(), tokenHash)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				httpapi.WriteError(w, http.StatusUnauthorized, httpapi.CodeUnauthenticated, "Connexion requise.")
				return
			}
			httpapi.WriteInternalError(w, r, err)
			return
		}

		lifetime := sessionLifetime(session.User.Role)
		if lifetime-session.ExpiresIn > sessionExtendAfter {
			if err := h.store.ExtendSession(r.Context(), tokenHash, lifetime); err != nil {
				slog.Warn("Extending session failed", "error", err)
			} else {
				h.setSessionCookie(w, cookie.Value, lifetime)
			}
		}
		next(w, r, session.User)
	}
}

// RequireGM answers with a 403 when the user isn't the GM.
func (h *Handlers) RequireGM(next UserHandler) http.HandlerFunc {
	return h.RequireSession(func(w http.ResponseWriter, r *http.Request, user storage.User) {
		if user.Role != storage.RoleGM {
			httpapi.WriteError(w, http.StatusForbidden, httpapi.CodeForbidden, "Réservé au MJ.")
			return
		}
		next(w, r, user)
	})
}
