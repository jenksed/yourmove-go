package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopPOSTRedirectsToGetNavigation(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	app := &App{db: db}
	handler := newDesktopMux(app)
	post := func(path string, data url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "http://localhost"+path, strings.NewReader(data.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "wails://localhost")
		r.Header.Set("X-Wails-Window-Id", "1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	w := post("/jobs", url.Values{"job_url": {"https://example.com/wails-test"}, "job_title": {"Test job"}})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs")`) || w.Header().Get("Location") != "" {
		t.Fatalf("add-job navigation response: status=%d location=%q body=%s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	jobs, err := app.allJobs()
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Test job" {
		t.Fatalf("add-job persistence: jobs=%+v err=%v", jobs, err)
	}

	w = post("/jobs/1/edit", url.Values{
		"title": {"Updated title"}, "company": {"Example & Company"},
		"stage": {StageReady}, "priority": {PriorityHigh},
	})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs/1")`) {
		t.Fatalf("edit navigation response: status=%d body=%s", w.Code, w.Body.String())
	}
	job, err := app.jobByID(1)
	if err != nil || job.Title != "Updated title" || job.Company != "Example & Company" {
		t.Fatalf("edit persistence: job=%+v err=%v", job, err)
	}

	w = post("/jobs/1/refresh", url.Values{})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `window.location.replace("/jobs/1")`) {
		t.Fatalf("refresh navigation response: status=%d body=%s", w.Code, w.Body.String())
	}

	// The browser route continues to issue a real 303 for PRG.
	r := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/refresh", nil)
	w = httptest.NewRecorder()
	newMux(app).ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/jobs/1" {
		t.Fatalf("browser redirect changed: status=%d location=%s", w.Code, w.Header().Get("Location"))
	}
}

func TestDesktopRedirectShimLeavesOtherResponsesUntouched(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		status       int
	}{
		{"external", "https://untrusted.test", http.StatusSeeOther},
		{"protocol relative", "//untrusted.test", http.StatusSeeOther},
		{"non numeric job", "/jobs/bad", http.StatusSeeOther},
		{"query", "/jobs/1?next=bad", http.StatusSeeOther},
		{"valid job", "/jobs/1", http.StatusOK},
		{"prepare", "/jobs/1#prepare", http.StatusOK},
		{"prepare noncanonical id", "/jobs/01#prepare", http.StatusSeeOther},
		{"prepare query", "/jobs/1?next=bad#prepare", http.StatusSeeOther},
		{"inspection prepare", "/jobs/1/inspection#prepare", http.StatusSeeOther},
		{"inspection", "/jobs/1/inspection", http.StatusOK},
		{"inspection external suffix", "/jobs/1/inspection/more", http.StatusSeeOther},
		{"inspection query", "/jobs/1/inspection?redirect=x", http.StatusSeeOther},
		{"resume library", "/resumes", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := withDesktopPostRedirects(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, tc.target, http.StatusSeeOther)
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "http://localhost/jobs", nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d", w.Code, tc.status)
			}
		})
	}
}

func TestContinuationRedirectTargetsStayLocalAndCanonical(t *testing.T) {
	for _, target := range []string{"/jobs/1?draft=2#prepare", "/jobs/1?transfer_notice=Cover+letter%3A+changed#prepare", "/jobs/1?draft=2&transfer_notice=Review+the+letter#prepare"} {
		if !safeDesktopRedirectTarget(target) {
			t.Fatal("rejected continuation", target)
		}
	}
	for _, target := range []string{"//evil.test/jobs/1?draft=2#prepare", "https://evil.test/jobs/1?draft=2#prepare", "/jobs/01?draft=2#prepare", "/jobs/1?draft=0#prepare", "/jobs/1?draft=2&draft=3#prepare", "/jobs/1?transfer_notice=one&transfer_notice=two#prepare", "/jobs/1?next=https%3A%2F%2Fevil.test#prepare", "/jobs/1?draft=2#other"} {
		if safeDesktopRedirectTarget(target) {
			t.Fatal("accepted unsafe target", target)
		}
	}
}
