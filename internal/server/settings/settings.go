// Package settings tells the settings of the instance.
package settings

import (
	"net/http"

	"github.com/wolf29f/hushtongue/internal/config"
	"github.com/wolf29f/hushtongue/internal/server/httpapi"
)

// Handlers serves the settings route.
type Handlers struct {
	sourceLangLabel string
	conLangLabel    string
}

// New creates the handlers of the settings in cfg.
func New(cfg config.Config) *Handlers {
	return &Handlers{sourceLangLabel: cfg.SourceLangLabel, conLangLabel: cfg.ConLangLabel}
}

type response struct {
	Languages languageLabels `json:"languages"`
}

type languageLabels struct {
	Source string `json:"source"`
	Con    string `json:"con"`
}

// Get serves GET /api/settings: the settings of the instance, without a
// session for the login page to show them.
func (h *Handlers) Get(w http.ResponseWriter, _ *http.Request) {
	httpapi.WriteJSON(w, http.StatusOK, response{
		Languages: languageLabels{Source: h.sourceLangLabel, Con: h.conLangLabel},
	})
}
