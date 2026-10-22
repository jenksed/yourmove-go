package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestWailsOriginScopedToDesktopHandler(t *testing.T) {
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
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
		{"desktop bridge accepts Wails origin without relying on header", newDesktopMux(app), "wails://localhost", "", http.StatusOK},
		{"desktop accepts Wails origin with window metadata", newDesktopMux(app), "wails://localhost", "1", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := send(tc.handler, tc.origin, tc.id); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestLocalRequestHostsAndOrigins(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin, site string
		want                     int
	}{
		{"localhost navigation", "localhost:3000", "", "none", 200},
		{"IPv4 form origin", "127.0.0.1:3000", "http://127.0.0.1:3000", "same-origin", 200},
		{"IPv6 form origin", "[::1]:3000", "http://[::1]:3000", "same-origin", 200},
		{"localhost without port", "localhost", "http://localhost", "", 200},
		{"IPv4 without port", "127.0.0.1", "", "", 200},
		{"IPv6 without port", "[::1]", "", "", 200},
		{"unknown host", "evil.example:3000", "", "", 403},
		{"matching hostile origin", "evil.example:3000", "http://evil.example:3000", "same-origin", 403},
		{"localhost suffix", "localhost.evil.example", "", "", 403},
		{"userinfo", "evil.example@localhost", "", "", 403},
		{"empty host", "", "", "", 403},
		{"empty port", "localhost:", "", "", 403},
		{"invalid port", "localhost:invalid", "", "", 403},
		{"out of range port", "localhost:65536", "", "", 403},
		{"bracketed hostname", "[localhost]", "", "", 403},
		{"host with path", "localhost/private", "", "", 403},
		{"foreign origin on GET", "localhost:3000", "https://evil.example", "", 403},
		{"opaque origin", "localhost:3000", "null", "", 403},
		{"different local port", "localhost:3000", "http://localhost:4000", "same-site", 403},
		{"origin with path", "localhost:3000", "http://localhost:3000/", "", 403},
		{"cross-site read without origin", "localhost:3000", "", "cross-site", 403},
		{"forged Wails metadata", "localhost", "wails://localhost", "", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://localhost/candidate", nil)
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.site)
			req.Header.Set("X-Wails-Window-Id", "1")
			called := false
			handler := protectLocalRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.want || called != (tc.want == http.StatusOK) {
				t.Fatalf("status=%d called=%v, want status=%d", w.Code, called, tc.want)
			}
		})
	}
}

func TestUnexpectedHostsCannotReadOrWriteWorkspace(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/private", Title: "Private opportunity"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.importResume("private.docx", bytes.NewReader(sampleDOCX(t))); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []struct {
		name    string
		handler http.Handler
	}{
		{"browser", newMux(a)},
		{"desktop", newDesktopMux(a)},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
			for _, path := range []string{"/jobs", "/candidate", "/resumes", "/resumes/1/download", "/jobs/1", "/jobs/1/browser-status", "/applications"} {
				t.Run(mode.name+"/"+method+path, func(t *testing.T) {
					form := url.Values{"job_url": {"https://example.com/injected"}, "job_title": {"Injected"}}
					req := httptest.NewRequest(method, "http://evil.example:3000"+path, strings.NewReader(form.Encode()))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					req.Header.Set("Origin", "http://evil.example:3000")
					req.Header.Set("Sec-Fetch-Site", "same-origin")
					req.Header.Set("X-Wails-Window-Id", "1")
					w := httptest.NewRecorder()
					mode.handler.ServeHTTP(w, req)
					if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "Private opportunity") {
						t.Fatalf("private route status=%d body=%s", w.Code, w.Body.String())
					}
				})
			}
		}
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Private opportunity" {
		t.Fatalf("workspace mutated: jobs=%+v err=%v", jobs, err)
	}
}

func TestBrowserCannotForgeNativePickerAccess(t *testing.T) {
	a, _ := testWorkspace(t)
	if _, err := a.importResume("private.docx", bytes.NewReader(sampleDOCX(t))); err != nil {
		t.Fatal(err)
	}
	pickerCalls := 0
	a.pickResume = func() (string, error) {
		pickerCalls++
		return "", nil
	}
	for _, path := range []string{"/resumes/import-native", "/resumes/1/revise-native"} {
		for _, origin := range []string{"", "http://localhost", "wails://localhost"} {
			w := postApp(newMux(a), path, url.Values{}, origin, "1")
			if w.Code != http.StatusForbidden {
				t.Fatalf("browser %s origin=%q status=%d", path, origin, w.Code)
			}
		}
		if pickerCalls != 0 {
			t.Fatal("browser request opened the native picker")
		}
	}
	for _, path := range []string{"/resumes/import-native", "/resumes/1/revise-native"} {
		w := postApp(newDesktopMux(a), path, url.Values{}, "wails://localhost", "")
		if w.Code != http.StatusOK {
			t.Fatalf("native bridge %s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	if pickerCalls != 2 {
		t.Fatalf("native picker called %d times, want 2", pickerCalls)
	}
}

func TestDuplicateOriginsRejectedAtBothEntryPoints(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, handler := range []http.Handler{newMux(a), newDesktopMux(a)} {
		req := httptest.NewRequest(http.MethodGet, "http://localhost/jobs", nil)
		req.Header.Add("Origin", "wails://localhost")
		req.Header.Add("Origin", "https://evil.example")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("duplicate origins status=%d", w.Code)
		}
	}
}
