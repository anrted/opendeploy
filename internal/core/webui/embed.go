// Package webui serves the production Vue application embedded in Core.
package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed dist
var assets embed.FS

func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic("webui: embedded dist directory is unavailable: " + err.Error())
	}
	files := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		requested := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requested == "." || requested == "" {
			requested = "index.html"
		}
		if _, err := fs.Stat(dist, requested); err != nil {
			requested = "index.html"
		}
		if requested == "index.html" {
			content, err := fs.ReadFile(dist, requested)
			if err != nil {
				http.Error(w, "embedded UI is unavailable", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", mime.TypeByExtension(".html"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(content)
			return
		}

		clone := r.Clone(r.Context())
		clone.URL.Path = "/" + requested
		files.ServeHTTP(w, clone)
	})
}
