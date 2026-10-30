package app

import (
	"io/fs"
	"net/http"
	"strings"
)

// loadFrontendIndex returns the built Vite index.html when a production
// frontend is embedded. Nil means the Go-rendered /jobs page remains.
// The embedded FS root is the repository root (frontend/dist/index.html);
// tests may supply a subtree FS containing dist/index.html or index.html.
func loadFrontendIndex(fsys fs.FS) []byte {
	if fsys == nil {
		return nil
	}
	for _, path := range []string{"frontend/dist/index.html", "dist/index.html", "index.html"} {
		data, err := fs.ReadFile(fsys, path)
		if err == nil && len(data) > 0 {
			return data
		}
	}
	return nil
}

// frontendDistFS returns the servable dist subtree for /app/ assets.
func (a *App) frontendDistFS() fs.FS {
	if a.resources == nil || a.resources.frontendDist == nil {
		return nil
	}
	for _, root := range []string{"frontend/dist", "dist", "."} {
		sub, err := fs.Sub(a.resources.frontendDist, root)
		if err != nil {
			continue
		}
		if _, err := fs.Stat(sub, "index.html"); err == nil {
			return sub
		}
	}
	return nil
}

// serveOpportunities prefers the React bundle when embedded, keeping the
// existing Go form POST on /jobs for compatibility and no-JS fallback.
// All other job routes remain Go-rendered.
func (a *App) serveOpportunities(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && len(a.resources.frontendIndex) > 0 {
		// Never cache the entry; hashed Vite assets under /app/ are immutable.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(a.resources.frontendIndex)
		return
	}
	a.jobForm(w, r)
}

func (a *App) serveFrontendAssets() http.Handler {
	sub := a.frontendDistFS()
	if sub == nil {
		return http.NotFoundHandler()
	}
	server := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".html") {
			w.Header().Set("Cache-Control", "no-store")
		}
		server.ServeHTTP(w, r)
	})
}
