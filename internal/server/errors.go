package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorCode identifies an error for the front to react to: a code never
// changes once the front relies on it.
type errorCode string

const (
	codeNotFound         errorCode = "not_found"
	codeMethodNotAllowed errorCode = "method_not_allowed"
)

// apiError is the body of every error response: Code is for the front to
// react to, Message is shown to the user.
type apiError struct {
	Code    errorCode `json:"error"`
	Message string    `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("unable to write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code errorCode, message string) {
	writeJSON(w, status, apiError{Code: code, Message: message})
}

// jsonFallback writes the mux's own 404 and 405 responses in the API error
// format.
func jsonFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Calling the returned handler directly would skip the path values
		// (r.PathValue) that only mux.ServeHTTP sets
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(&fallbackWriter{ResponseWriter: w}, r)
	})
}

// fallbackWriter replaces the plain text body of an error with a JSON one.
type fallbackWriter struct {
	http.ResponseWriter
	replaced bool
}

func (f *fallbackWriter) WriteHeader(status int) {
	switch status {
	case http.StatusNotFound:
		f.replaced = true
		writeError(f.ResponseWriter, status, codeNotFound, "Ressource introuvable.")
	case http.StatusMethodNotAllowed:
		f.replaced = true
		writeError(f.ResponseWriter, status, codeMethodNotAllowed, "Méthode non autorisée.")
	default:
		f.ResponseWriter.WriteHeader(status)
	}
}

func (f *fallbackWriter) Write(b []byte) (int, error) {
	if f.replaced {
		return len(b), nil
	}
	return f.ResponseWriter.Write(b)
}
