// Package httpapi holds what the API's handlers share: the JSON requests and
// responses, the error format and the logged in user. It logs with the
// default slog logger.
package httpapi

import (
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wolf29f/hushtongue/internal/services/storage"
)

// ErrorCode identifies an error for the front to react to: a code never
// changes once the front relies on it.
type ErrorCode string

// Error codes of the API.
const (
	CodeInvalidRequest       ErrorCode = "invalid_request"
	CodeUnauthenticated      ErrorCode = "unauthenticated"
	CodeInvalidCredentials   ErrorCode = "invalid_credentials"
	CodeForbidden            ErrorCode = "forbidden"
	CodeCrossOrigin          ErrorCode = "cross_origin"
	CodeNotFound             ErrorCode = "not_found"
	CodeMethodNotAllowed     ErrorCode = "method_not_allowed"
	CodeRequestTooLarge      ErrorCode = "request_too_large"
	CodeUnsupportedMediaType ErrorCode = "unsupported_media_type"
	CodeInvalidLink          ErrorCode = "invalid_link"
	CodeNameTaken            ErrorCode = "name_taken"
	CodeTooManyAttempts      ErrorCode = "too_many_attempts"
	CodeInternal             ErrorCode = "internal"
	CodeDatabaseUnavailable  ErrorCode = "database_unavailable"
)

// Error is the body of every error response: Code is for the front to react
// to, Message is shown to the user.
type Error struct {
	Code    ErrorCode `json:"error"`
	Message string    `json:"message"`
}

// WriteJSON answers with status and body encoded in JSON.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.MarshalWrite(w, body); err != nil {
		slog.Error("Writing response failed", "error", err)
	}
}

// WriteError answers with status and an Error.
func WriteError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	WriteJSON(w, status, Error{Code: code, Message: message})
}

// WriteInternalError logs err and answers with a 500.
func WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("Request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	WriteError(w, http.StatusInternalServerError, CodeInternal, "Erreur interne.")
}

// WriteServiceError answers with the response of a service's sentinel error,
// or with a 500 for any other error.
func WriteServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		WriteError(w, http.StatusNotFound, CodeNotFound, "Ressource introuvable.")
	case errors.Is(err, storage.ErrNameTaken):
		WriteError(w, http.StatusConflict, CodeNameTaken, "Ce nom est déjà pris.")
	default:
		WriteInternalError(w, r, err)
	}
}

// PathID returns the id of the request's path. It answers with a 404 and
// returns false when the id isn't a number.
func PathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		WriteError(w, http.StatusNotFound, CodeNotFound, "Ressource introuvable.")
		return 0, false
	}
	return id, true
}

// ReadJSON decodes the request body into v. It answers with an error and
// returns false when the body is invalid.
func ReadJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.UnmarshalRead(r.Body, v); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			WriteError(w, http.StatusRequestEntityTooLarge, CodeRequestTooLarge, "Requête trop volumineuse.")
			return false
		}
		WriteError(w, http.StatusBadRequest, CodeInvalidRequest, "Requête invalide.")
		return false
	}
	return true
}
