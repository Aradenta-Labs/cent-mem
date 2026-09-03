package ui

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var embeddedDist embed.FS

// DistFS returns the fs.FS subtree corresponding to dist.
func DistFS() (fs.FS, error) {
	return fs.Sub(embeddedDist, "dist")
}

// FileServerHandler returns an http.Handler that serves embedded static assets
// with client-side single page application (SPA) fallback to index.html for unknown routes.
func FileServerHandler() (http.Handler, error) {
	dist, err := DistFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Do not intercept API requests
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Normalize path
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Check if file exists in dist
		f, err := dist.Open(cleanPath)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routes (e.g. /ui/design-system)
		indexFile, err := dist.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}
		defer indexFile.Close()

		stat, err := indexFile.Stat()
		if err != nil {
			http.Error(w, "index.html stat error", http.StatusInternalServerError)
			return
		}

		// Prevent caching index.html so app updates take effect immediately
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
	}), nil
}
