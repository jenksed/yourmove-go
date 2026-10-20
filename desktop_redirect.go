package main

import (
	"encoding/json"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// withDesktopPostRedirects adapts PRG navigation to macOS Wails' custom
// URL scheme, which cannot reliably follow a 303 from the asset handler.
// Only the Wails entry point uses this wrapper; browser mode keeps real 303s.
func withDesktopPostRedirects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(&desktopRedirectWriter{ResponseWriter: w, requestPath: r.URL.Path}, r)
	})
}

type desktopRedirectWriter struct {
	http.ResponseWriter
	requestPath string
	redirected  bool
}

func (w *desktopRedirectWriter) WriteHeader(status int) {
	if status != http.StatusSeeOther {
		w.ResponseWriter.WriteHeader(status)
		return
	}

	// Limit this compatibility shim to YourMove's current internal POST
	// destinations. Never render an external or user-controlled destination.
	target := w.Header().Get("Location")
	if !safeDesktopRedirectTarget(target) {
		w.ResponseWriter.WriteHeader(status)
		return
	}

	w.redirected = true
	w.Header().Del("Location")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.ResponseWriter.WriteHeader(http.StatusOK)

	// Both a normal WebView navigation and a non-JS fallback use GET.
	// json.Marshal HTML-escapes the string used in the script element.
	jsTarget, _ := json.Marshal(target)
	htmlTarget := html.EscapeString(target)
	_, _ = w.ResponseWriter.Write([]byte(`<!doctype html><html lang="en"><head><meta charset="utf-8">` +
		`<meta http-equiv="refresh" content="0;url=` + htmlTarget + `">` +
		`<script>window.location.replace(` + string(jsTarget) + `)</script></head>` +
		`<body><p>Continuing to <a href="` + htmlTarget + `">saved jobs</a>…</p></body></html>`))
	log.Printf("desktop navigation: POST %s -> GET %s", w.requestPath, target)
}

func (w *desktopRedirectWriter) Write(p []byte) (int, error) {
	// http.Redirect may write an HTML fallback after WriteHeader. The
	// desktop HTML response has already supplied its own fallback.
	if w.redirected {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}

func safeDesktopRedirectTarget(target string) bool {
	if target == "/jobs" || target == "/resumes" || target == "/enrichment" || target == "/families" || target == "/utilities" || target == "/candidate" {
		return true
	}
	if !strings.HasPrefix(target, "/jobs/") {
		return false
	}
	rawID := strings.TrimPrefix(target, "/jobs/")
	rawID = strings.TrimSuffix(rawID, "/inspection")
	id, err := strconv.ParseInt(rawID, 10, 64)
	return err == nil && id > 0 && (target == "/jobs/"+rawID || target == "/jobs/"+rawID+"/inspection") && strconv.FormatInt(id, 10) == rawID
}
