// Package frontend serves the built front: its files, and index.html for any
// other path, left to the front's router.
package frontend

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Handler serves the files of fsys, and index.html for any other path. The
// files of assets/, whose names hold a hash of their content, are cached for
// good.
func Handler(fsys fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if info, err := fs.Stat(fsys, name); err != nil || info.IsDir() {
			name = "index.html"
		} else if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFileFS(w, r, fsys, name)
	}
}
