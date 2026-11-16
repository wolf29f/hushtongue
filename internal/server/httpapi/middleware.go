package httpapi

import (
	"mime"
	"net/http"
)

// Handler serves mux, with its own 404 and 405 responses in the API error
// format. It refuses a request of a route that may modify data without a
// JSON body: a form of another site can't send one without a CORS preflight.
func Handler(mux *http.ServeMux) http.Handler {
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
			WriteError(w, http.StatusUnsupportedMediaType,
				CodeUnsupportedMediaType,
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
