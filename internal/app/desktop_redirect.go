package app

import (
	"context"
	"encoding/json"
	"html"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Mount only in Wails' in-process AssetOptions.Handler, never on a TCP server.
// This separate entry point is the trust boundary; a window-ID header is not
// authentication and cannot grant browser requests native capabilities.
func newDesktopMux(app *App) http.Handler {
	return withDesktopPostRedirects(withWailsOrigin(newMux(app)))
}

type desktopRequestKey struct{}

func isDesktopRequest(r *http.Request) bool {
	trusted, _ := r.Context().Value(desktopRequestKey{}).(bool)
	return trusted
}

func withWailsOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if r.Host != "localhost" || len(r.Header.Values("Origin")) > 1 ||
			(origin != "" && origin != "wails://localhost" && origin != "http://localhost") {
			http.Error(w, "Desktop application request required", http.StatusForbidden)
			return
		}
		r = r.Clone(context.WithValue(r.Context(), desktopRequestKey{}, true))
		if origin == "wails://localhost" {
			r.Header.Set("Origin", "http://localhost")
		}
		next.ServeHTTP(w, r)
	})
}

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
	if !safeDesktopRedirectTarget(target) || (strings.Contains(target, "?question=") &&
		w.requestPath != strings.Split(target, "?")[0]+"/answers/save") {
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
	for _, notice := range []string{"in-use", "master", "archived", "restored", "revised", "duplicate", "failed"} {
		if target == "/resumes?notice="+notice {
			return true
		}
	}
	if target == "/jobs" || target == "/resumes" || target == "/enrichment" || target == "/families" || target == "/utilities" || target == "/candidate" || target == "/ai" {
		return true
	}
	// Question saves use one exact hash-shaped presentation key. The save handler
	// validates membership; the fresh GET independently rejects stale/foreign keys.
	if parsed, err := url.Parse(target); err == nil && parsed.Fragment == "prepare" {
		query, err := url.ParseQuery(parsed.RawQuery)
		id, idErr := strconv.ParseInt(strings.TrimPrefix(parsed.Path, "/jobs/"), 10, 64)
		if err == nil && idErr == nil && id > 0 && parsed.Path == "/jobs/"+strconv.FormatInt(id, 10) &&
			parsed.Scheme == "" && parsed.Host == "" && parsed.User == nil &&
			len(query) == 1 && len(query["question"]) == 1 && questionReturnKey.MatchString(query.Get("question")) &&
			target == parsed.Path+"?question="+query.Get("question")+"#prepare" {
			return true
		}
	}
	// Cover letter actions return to the Prepare tab with an allowlisted notice.
	if parsed, err := url.Parse(target); err == nil && parsed.Fragment == "prepare" {
		// Combined transfer results may retain the exact draft and a local,
		// escaped per-item error. Accept only canonical internal Prepare URLs.
		query, queryErr := url.ParseQuery(parsed.RawQuery)
		if queryErr == nil && len(query) > 0 && query.Encode() == parsed.RawQuery && len(parsed.RawQuery) <= 8192 &&
			parsed.Scheme == "" && parsed.Host == "" && parsed.User == nil {
			valid := true
			for key, values := range query {
				if len(values) != 1 {
					valid = false
					break
				}
				switch key {
				case "draft":
					draft, err := strconv.ParseInt(values[0], 10, 64)
					valid = err == nil && draft > 0 && strconv.FormatInt(draft, 10) == values[0]
				case "transfer_notice":
					valid = values[0] != "" && len(values[0]) <= 4096
				default:
					valid = false
				}
				if !valid {
					break
				}
			}
			id, err := strconv.ParseInt(strings.TrimPrefix(parsed.Path, "/jobs/"), 10, 64)
			if valid && err == nil && id > 0 && parsed.Path == "/jobs/"+strconv.FormatInt(id, 10) {
				return true
			}
		}
		code := parsed.Query().Get("cover")
		switch code {
		case "generated", "saved", "transferring", "sources", "model", "failed", "invalid", "cli", "login", "cli-failed",
			"key", "auth", "rate", "credits", "timeout", "output", "provider-unavailable", "offline", "model-missing":
			allowedQuery := parsed.RawQuery == "cover="+code
			if draft := parsed.Query().Get("draft"); draft != "" {
				id, e := strconv.ParseInt(draft, 10, 64)
				allowedQuery = e == nil && id > 0 && strconv.FormatInt(id, 10) == draft &&
					parsed.RawQuery == "cover="+code+"&draft="+draft
			}
			if allowedQuery && parsed.Scheme == "" && parsed.Host == "" && parsed.User == nil &&
				strings.HasPrefix(parsed.Path, "/jobs/") {
				id, err := strconv.ParseInt(strings.TrimPrefix(parsed.Path, "/jobs/"), 10, 64)
				if err == nil && id > 0 && parsed.Path == "/jobs/"+strconv.FormatInt(id, 10) {
					return true
				}
			}
		}
	}
	if !strings.HasPrefix(target, "/jobs/") {
		return false
	}
	rawID := strings.TrimPrefix(target, "/jobs/")
	for _, fragment := range []string{"#prepare", "#history"} {
		if strings.HasSuffix(rawID, fragment) {
			rawID = strings.TrimSuffix(rawID, fragment)
			id, err := strconv.ParseInt(rawID, 10, 64)
			return err == nil && id > 0 && target == "/jobs/"+strconv.FormatInt(id, 10)+fragment
		}
	}
	rawID = strings.TrimSuffix(rawID, "/inspection")
	id, err := strconv.ParseInt(rawID, 10, 64)
	return err == nil && id > 0 && (target == "/jobs/"+rawID || target == "/jobs/"+rawID+"/inspection") && strconv.FormatInt(id, 10) == rawID
}
