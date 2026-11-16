package middleware

import (
	"net/http"

	"github.com/wolf29f/hushtongue/internal/server/httpapi"
)

// CrossOrigin answers with a 403 to a request of another site that may
// modify data.
func CrossOrigin(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, http.StatusForbidden, httpapi.CodeCrossOrigin, "Requête d'un autre site refusée.")
	}))
	return protection.Handler(next)
}
