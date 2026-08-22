package web

import (
	"bytes"
	"embed"
	"net/http"
	pathpkg "path"
	"strings"
	"time"
)

//go:embed *
var FS embed.FS

// Handler serves the public story separately from the account cockpit.
func Handler() http.Handler {
	files := http.FileServer(http.FS(FS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		requestPath := r.URL.Path
		switch requestPath {
		case "/":
			serveEmbeddedDocument(w, r, "landing.html")
		case "/account", "/account/":
			w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
			serveEmbeddedDocument(w, r, "index.html")
		case "/index.html", "/landing.html":
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
		default:
			staticAsset := requestPath == "/robots.txt" || requestPath == "/sitemap.xml" || requestPath == "/geometric-landing.js" || requestPath == "/geometric-renderer.js" || strings.HasPrefix(requestPath, "/assets/") || strings.HasPrefix(requestPath, "/runtime/")
			if staticAsset && pathpkg.Clean(requestPath) == requestPath && !strings.Contains(requestPath, "..") {
				serveEmbeddedPath(files, w, r, requestPath)
				return
			}
			http.NotFound(w, r)
		}
	})
}

func serveEmbeddedDocument(w http.ResponseWriter, r *http.Request, name string) {
	contents, err := FS.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(contents))
}

func serveEmbeddedPath(files http.Handler, w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	clone := r.Clone(r.Context())
	clone.URL.Path = path
	files.ServeHTTP(w, clone)
}
