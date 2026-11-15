package server

import (
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// errorCode identifies an error for the front to react to: a code never
// changes once the front relies on it.
type errorCode string

const (
	codeInvalidRequest       errorCode = "invalid_request"
	codeUnauthenticated      errorCode = "unauthenticated"
	codeInvalidCredentials   errorCode = "invalid_credentials"
	codeForbidden            errorCode = "forbidden"
	codeCrossOrigin          errorCode = "cross_origin"
	codeNotFound             errorCode = "not_found"
	codeMethodNotAllowed     errorCode = "method_not_allowed"
	codeRequestTooLarge      errorCode = "request_too_large"
	codeUnsupportedMediaType errorCode = "unsupported_media_type"
	codeInvalidLink          errorCode = "invalid_link"
	codeNameTaken            errorCode = "name_taken"
	codeTooManyAttempts      errorCode = "too_many_attempts"
	codeInternal             errorCode = "internal"
	codeDatabaseUnavailable  errorCode = "database_unavailable"
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
	if err := json.MarshalWrite(w, body); err != nil {
		slog.Error("unable to write response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code errorCode, message string) {
	writeJSON(w, status, apiError{Code: code, Message: message})
}

// writeInternalError logs err and answers with a 500.
func (s *Server) writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("Request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, codeInternal, "Erreur interne.")
}

// writeServiceError answers with the response of a service's sentinel
// error, or with a 500 for any other error.
func (s *Server) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, http.StatusNotFound, codeNotFound, "Ressource introuvable.")
	case errors.Is(err, storage.ErrNameTaken):
		writeError(w, http.StatusConflict, codeNameTaken, "Ce nom est déjà pris.")
	default:
		s.writeInternalError(w, r, err)
	}
}

// pathID returns the id of the request's path. It answers with a 404 and
// returns false when the id isn't a number.
func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, codeNotFound, "Ressource introuvable.")
		return 0, false
	}
	return id, true
}

// readJSON decodes the request body into v. It answers with an error and
// returns false when the body is invalid.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.UnmarshalRead(r.Body, v); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, codeRequestTooLarge, "Requête trop volumineuse.")
			return false
		}
		writeError(w, http.StatusBadRequest, codeInvalidRequest, "Requête invalide.")
		return false
	}
	return true
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
