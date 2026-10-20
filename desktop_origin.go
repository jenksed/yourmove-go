package main

import "net/http"

// Only desktop Wails has this asset-server origin. Browser mode keeps its
// existing Origin restriction unchanged, including its CSRF rejection.
func newDesktopMux(app *App) http.Handler {
	return withDesktopPostRedirects(withWailsOrigin(newMux(app)))
}

func withWailsOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.Header.Get("Origin") == "wails://localhost" &&
			r.Host == "localhost" && r.Header.Get("X-Wails-Window-Id") != "" {
			r = r.Clone(r.Context())
			r.Header.Set("Origin", "http://localhost")
		}
		next.ServeHTTP(w, r)
	})
}
