package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/michaelbrown/forge/web"
)

// spaHandler serves embedded static files with SPA fallback.
// Any path that doesn't match a static file serves index.html.
func spaHandler() http.Handler {
	dist, _ := fs.Sub(web.Assets, "dist")
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to open the requested file
		f, err := dist.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for non-file paths
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
