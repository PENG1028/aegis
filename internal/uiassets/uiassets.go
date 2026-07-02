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
	"io"
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
		ext := filepath.Ext(path)
		isSPARoute := ext == "" || ext == ".html"

		f, err := sub.Open(path)
		if err == nil {
			fi, statErr := f.Stat()
			if statErr == nil && !fi.IsDir() {
				f.Close()
				// Static file exists — serve it directly.
				w.Header().Set("X-Aegis-Debug", "static:"+path)
				fileServer.ServeHTTP(w, r)
				return
			}
			f.Close()
		}

		if isSPARoute {
			// SPA fallback — serve index.html directly, bypassing fileServer
			// which may redirect directory paths unexpectedly.
			w.Header().Set("X-Aegis-Debug", "spa:"+path)
			indexFile, err := sub.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer indexFile.Close()
			fi, err := indexFile.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", fi.ModTime(), indexFile.(io.ReadSeeker))
			return
		}

		// File not found and not an SPA route — 404.
		w.Header().Set("X-Aegis-Debug", "notfound:"+path)
		http.NotFound(w, r)
	}), nil
}
