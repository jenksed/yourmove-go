package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestWailsOriginScopedToDesktopHandler(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{db: db}
	form := url.Values{"job_url": {"https://example.com/test"}, "job_title": {"Window test"}}
	send := func(handler http.Handler, origin, id string) int {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/jobs", strings.NewReader(form.Encode()))
		r.Host = "localhost"
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", origin)
		if id != "" {
			r.Header.Set("X-Wails-Window-Id", id)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	for _, tc := range []struct {
		name       string
		handler    http.Handler
		origin, id string
		want       int
	}{
		{"browser rejects Wails origin", newMux(app), "wails://localhost", "1", http.StatusForbidden},
		{"desktop rejects untrusted origin", newDesktopMux(app), "https://evil.example", "1", http.StatusForbidden},
		{"desktop rejects unmarked Wails origin", newDesktopMux(app), "wails://localhost", "", http.StatusForbidden},
		{"desktop accepts marked Wails origin", newDesktopMux(app), "wails://localhost", "1", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := send(tc.handler, tc.origin, tc.id); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}
