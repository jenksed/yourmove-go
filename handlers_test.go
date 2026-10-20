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

func TestEditJobHTTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{db: db}
	if err := app.saveJob(Job{URL: "https://example.com/job", Host: "example.com", Title: "Original"}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(app)

	form := url.Values{
		"title": {"Support Engineer"}, "company": {"Example & Co"},
		"description": {"Production troubleshooting <script>alert(1)</script>"},
		"stage":       {StageReady}, "priority": {PriorityHigh},
	}
	post := func(values url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/jobs/1/edit", strings.NewReader(values.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := post(form); w.Code != http.StatusSeeOther {
		t.Fatalf("edit status = %d: %s", w.Code, w.Body.String())
	}
	job, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Title != "Support Engineer" || job.Company != "Example & Co" || job.Stage != StageReady || job.Priority != PriorityHigh || job.URL != "https://example.com/job" {
		t.Fatalf("edited job = %+v", job)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/jobs/1", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Example &amp; Co") || strings.Contains(w.Body.String(), "<script>alert(1)</script>") {
		t.Fatalf("rendered job: %d %s", w.Code, w.Body.String())
	}

	bad := url.Values{"title": {"Invalid"}, "stage": {"applied"}, "priority": {PriorityNormal}}
	if w := post(bad); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid stage status = %d", w.Code)
	}
	unchanged, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Title != job.Title || unchanged.Stage != job.Stage {
		t.Fatalf("invalid edit mutated job: %+v", unchanged)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := (&App{db: reopened}).jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Title != job.Title || persisted.Company != job.Company || persisted.Stage != job.Stage || persisted.Priority != job.Priority {
		t.Fatalf("job lost changes after reopening SQLite: %+v", persisted)
	}
}

func TestCrossOriginPOSTBlocked(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mux := newMux(&App{db: db})
	form := url.Values{"job_url": {"https://example.com/job"}}
	r := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin POST status = %d", w.Code)
	}
	jobs, err := (&App{db: db}).allJobs()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 0 {
		t.Fatalf("cross-origin POST inserted %d jobs", len(jobs))
	}
}

func TestRefreshUnsupportedSourceIsNonDestructive(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{db: db}
	if err := app.saveJob(Job{URL: "https://example.com/job", Host: "example.com", Title: "Manual role", SalaryText: "$85,000 (manual)", SalaryOrigin: "manual"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/jobs/1/refresh", nil)
	rec := httptest.NewRecorder()
	newMux(app).ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	job, err := app.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Title != "Manual role" || job.SalaryText != "$85,000 (manual)" || job.SourceStatus != "unsupported" || job.SourceCheckedAt == "" {
		t.Fatalf("unsupported refresh damaged saved data: %+v", job)
	}
}
