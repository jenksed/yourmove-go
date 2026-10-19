package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestJobHTTP(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db}
	if err := app.saveJob(Job{
		URL: "https://example.com/job", Host: "example.com",
		Title: "Example role", Description: "First line\n<script>alert(1)</script>",
	}); err != nil {
		t.Fatal(err)
	}
	handler := newMux(app)

	t.Run("detail renders escaped content", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/jobs/1", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", w.Code, body)
		}
		if !strings.Contains(body, "Example role") || !strings.Contains(body, "&lt;script&gt;") {
			t.Fatalf("detail missing expected content: %s", body)
		}
		if strings.Contains(body, "<script>alert(1)</script>") {
			t.Fatal("untrusted description rendered as executable HTML")
		}
	})

	t.Run("list links to details", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jobs", nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `href="/jobs/1"`) {
			t.Fatalf("unexpected jobs list: status=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("unknown IDs are 404", func(t *testing.T) {
		for _, path := range []string{"/jobs/999", "/jobs/0", "/jobs/not-a-number"} {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusNotFound {
				t.Errorf("%s: status = %d", path, w.Code)
			}
		}
	})

	t.Run("unsupported URL remains saveable", func(t *testing.T) {
		form := url.Values{
			"job_url":   {"https://example.com/another"},
			"job_title": {"Manual title"},
		}
		r := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("status = %d: %s", w.Code, w.Body.String())
		}
		got, err := app.jobByID(2)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "Manual title" || got.URL != "https://example.com/another" {
			t.Fatalf("unexpected job: %+v", got)
		}
	})

	t.Run("CSS is served", func(t *testing.T) {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/static/styles.css", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		b, err := io.ReadAll(w.Result().Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "--color-primary") {
			t.Fatal("expected app stylesheet")
		}
	})
}
