package server

import (
	"errors"
	"mime"
	"net/http"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// apiHandler serves mux, with its own 404 and 405 responses in the API error
// format. It refuses a request of a route that may modify data without a
// JSON body: a form of another site can't send one without a CORS preflight.
func apiHandler(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Calling the returned handler directly would skip the path values
		// (r.PathValue) that only mux.ServeHTTP sets
		if _, pattern := mux.Handler(r); pattern == "" {
			mux.ServeHTTP(&fallbackWriter{ResponseWriter: w}, r)
			return
		}
		if r.Method != http.MethodGet &&
			r.Method != http.MethodHead &&
			r.Method != http.MethodOptions &&
			!isJSON(r) {
			writeError(w, http.StatusUnsupportedMediaType,
				codeUnsupportedMediaType,
				"Le corps de la requête doit être en JSON.")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func isJSON(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

// userHandler handles a request of the logged in user.
type userHandler func(w http.ResponseWriter, r *http.Request, user storage.User)

// requireSession answers with a 401 without a session, and calls next with
// the session's user otherwise. It extends the session at most once a day.
func (s *Server) requireSession(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, codeUnauthenticated, "Connexion requise.")
			return
		}
		tokenHash := hashToken(cookie.Value)
		session, err := s.store.Session(r.Context(), tokenHash)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, codeUnauthenticated, "Connexion requise.")
				return
			}
			s.writeInternalError(w, r, err)
			return
		}

		lifetime := sessionLifetime(session.User.Role)
		if lifetime-session.ExpiresIn > sessionExtendAfter {
			if err := s.store.ExtendSession(r.Context(), tokenHash, lifetime); err != nil {
				s.logger.Warn("Extending session failed", "error", err)
			} else {
				s.setSessionCookie(w, cookie.Value, lifetime)
			}
		}
		next(w, r, session.User)
	}
}

// requireGM answers with a 403 when the user isn't the GM.
func (s *Server) requireGM(next userHandler) http.HandlerFunc {
	return s.requireSession(func(w http.ResponseWriter, r *http.Request, user storage.User) {
		if user.Role != storage.RoleGM {
			writeError(w, http.StatusForbidden, codeForbidden, "Réservé au MJ.")
			return
		}
		next(w, r, user)
	})
}
