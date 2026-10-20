package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkbenchUISharedViewsAndSubmissionSemantics(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := &App{db: db}
	for _, job := range []Job{
		{URL: "https://example.com/first", Host: "example.com", Title: "Support <Engineer>"},
		{URL: "https://example.com/second", Host: "example.com", Title: "Implementation"},
	} {
		if err := app.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	mux := newMux(app)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	opportunities := get("/jobs")
	if opportunities.Code != http.StatusOK || !strings.Contains(opportunities.Body.String(), `href="/applications"`) ||
		!strings.Contains(opportunities.Body.String(), `data-job-search`) || !strings.Contains(opportunities.Body.String(), "Support &lt;Engineer&gt;") ||
		strings.Contains(opportunities.Body.String(), "Support <Engineer>") {
		t.Fatalf("opportunity workspace not rendered safely: %d %s", opportunities.Code, opportunities.Body.String())
	}
	if page := get("/jobs/1"); page.Code != 200 || !strings.Contains(page.Body.String(), `data-workspace-panel="prepare"`) || !strings.Contains(page.Body.String(), `name="submission_token"`) || !strings.Contains(page.Body.String(), `action="/jobs/1/edit"`) {
		t.Fatalf("job workspace has missing presentation or existing forms: %d %s", page.Code, page.Body.String())
	}
	empty := get("/applications")
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), "No applications recorded yet") {
		t.Fatalf("unexpected history before confirmation: %d %s", empty.Code, empty.Body.String())
	}
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.recordSubmission(2, "Manual confirmation", token); err != nil {
		t.Fatal(err)
	}
	applied := get("/applications")
	if applied.Code != 200 || !strings.Contains(applied.Body.String(), "Implementation") || strings.Contains(applied.Body.String(), "Support &lt;Engineer&gt;") || !strings.Contains(applied.Body.String(), `href="/jobs/2#history"`) {
		t.Fatalf("submitted-only application list wrong: %d %s", applied.Code, applied.Body.String())
	}
	page := summarizeJobs(mustAllJobs(t, app))
	if page.Total != 2 || page.Ready != 0 || page.Applied != 1 {
		t.Fatalf("summary is inaccurate: %+v", page)
	}
	for _, path := range []string{"/static/styles.css", "/static/ui.js", "/static/yourmove-wordmark-primary.svg"} {
		if w := get(path); w.Code != 200 {
			t.Errorf("asset %s not served: %d", path, w.Code)
		}
	}
}

func mustAllJobs(t *testing.T, app *App) []Job {
	t.Helper()
	jobs, err := app.allJobs()
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

func TestWorkbenchKeepsBrowserAndDesktopPostRedirectContracts(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &App{db: db}
	if err := a.saveJob(Job{URL: "https://example.com/one", Host: "example.com", Title: "One"}); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"title": {"Updated"}, "company": {"Example"}, "stage": {"ready"}, "priority": {"normal"}}
	for _, tc := range []struct {
		handler http.Handler
		desktop bool
	}{{newMux(a), false}, {newDesktopMux(a), true}} {
		origin := "http://localhost"
		if tc.desktop {
			origin = "wails://localhost"
		}
		rec := postApp(tc.handler, "/jobs/1/edit", form, origin, "1")
		if tc.desktop {
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), `window.location.replace("/jobs/1")`) {
				t.Fatalf("desktop navigation regressed: %d %s", rec.Code, rec.Body.String())
			}
		} else if rec.Code != 303 || rec.Header().Get("Location") != "/jobs/1" {
			t.Fatalf("browser navigation regressed: %d %q", rec.Code, rec.Header().Get("Location"))
		}
	}
}
