// Package uiassets embeds the production UI build (ui/dist) into the binary.
//
// Build requirement: ui/dist must be copied to internal/uiassets/dist before
// compilation. The Makefile handles this automatically via the build-linux and
// build targets.
//
//	Makefile:
//	  build-ui: cd ui && npm run build
//	  build-linux: build-ui && cp -r ui/dist internal/uiassets/dist && go build ...
package uiassets

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed dist
var embedded embed.FS

// FS returns the embedded UI file system rooted at "dist".
// Callers can use it with http.FileServer.
func FS() (http.FileSystem, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}

// Handler returns an http.Handler that serves the embedded UI assets.
//
// For SPA routing (React Router), any request that does not match a static
// file is served index.html so the client-side router can handle it.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalize path — the mux may pass through any unmatched path.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open as a static file.
		// If the path has no extension (SPA route like /routes, /nodes),
		// serve index.html so React Router takes over.
		ext := filepath.Ext(path)
		isSPARoute := ext == "" || ext == ".html"

		f, err := sub.Open(path)
		if err == nil {
			f.Close()
			// Static file exists — serve it directly.
			fileServer.ServeHTTP(w, r)
			return
		}

		if isSPARoute {
			// SPA fallback — rewrite to index.html.
			r.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found and not an SPA route — 404.
		http.NotFound(w, r)
	}), nil
}
