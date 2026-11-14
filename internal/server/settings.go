package server

import "net/http"

type settingsResponse struct {
	Languages languageLabels `json:"languages"`
}

type languageLabels struct {
	Source string `json:"source"`
	Con    string `json:"con"`
}

// handleSettings serves GET /api/settings: the settings of the instance,
// without a session for the login page to show them.
func (s *Server) handleSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, settingsResponse{
		Languages: languageLabels{Source: s.cfg.SourceLangLabel, Con: s.cfg.ConLangLabel},
	})
}
