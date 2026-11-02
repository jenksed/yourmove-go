package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestWorkbenchUISharedViewsAndSubmissionSemantics(t *testing.T) {
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := newTestApp(t, db, "")
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
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
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
	token := reviewedSubmissionToken(t, app, 2)
	if err := app.recordSubmission(2, "Manual confirmation", token); err != nil {
		t.Fatal(err)
	}
	applied := get("/applications")
	if applied.Code != 200 || !strings.Contains(applied.Body.String(), "Implementation") || strings.Contains(applied.Body.String(), "Support &lt;Engineer&gt;") || !strings.Contains(applied.Body.String(), `href="/jobs/2#history"`) {
		t.Fatalf("submitted-only application list wrong: %d %s", applied.Code, applied.Body.String())
	}
	page := summarizeJobs(mustAllJobs(t, app))
	if page.Total != 2 || page.Remaining != 1 || page.Applied != 1 {
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
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := newTestApp(t, db, "")
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

func TestOpportunityRemainingCountTracksConfirmedCompletion(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, j := range []Job{
		{URL: "https://example.test/review"},
		{URL: "https://example.test/marked-ready", Stage: StageReady},
		{URL: "https://example.test/closed", Stage: StageClosed},
	} {
		if err := a.saveJob(j); err != nil {
			t.Fatal(err)
		}
		if j.Stage != "" {
			var id int64
			if err := a.db.QueryRow("SELECT id FROM jobs WHERE url=?", j.URL).Scan(&id); err != nil {
				t.Fatal(err)
			}
			saved, err := a.jobByID(id)
			if err != nil {
				t.Fatal(err)
			}
			saved.Stage = j.Stage
			if err := a.updateJob(saved); err != nil {
				t.Fatal(err)
			}
		}
	}
	before := summarizeJobs(mustAllJobs(t, a))
	if before.Total != 3 || before.Remaining != 2 || before.Applied != 0 {
		t.Fatalf("before: %+v", before)
	}
	if err := a.recordSubmission(2, "Completed", reviewedSubmissionToken(t, a, 2)); err != nil {
		t.Fatal(err)
	}
	after := summarizeJobs(mustAllJobs(t, a))
	if after.Total != 3 || after.Remaining != 1 || after.Applied != 1 {
		t.Fatalf("after: %+v", after)
	}
	if (Job{Stage: StageReady}).StageLabel() != "Marked ready" {
		t.Fatal("manual planning stage implies verified readiness")
	}
}

func TestDashboardTodayCountsJobsAtLocalDayBoundaries(t *testing.T) {
	a, _ := testWorkspace(t)
	zone, err := time.LoadLocation("America/Detroit")
	if err != nil {
		t.Fatal(err)
	}
	// The fall DST day spans 25 hours; midnight boundaries must still be local.
	now := time.Date(2026, 11, 1, 15, 0, 0, 0, zone)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, zone)
	end := start.AddDate(0, 0, 1)
	for i, at := range []time.Time{start.Add(-time.Second), start, end.Add(-time.Second), end} {
		if err := a.saveJob(Job{URL: fmt.Sprintf("https://example.test/day/%d", i)}); err != nil {
			t.Fatal(err)
		}
		id := int64(i + 1)
		if err := a.recordSubmission(id, "", reviewedSubmissionToken(t, a, id)); err != nil {
			t.Fatal(err)
		}
		if _, err := a.db.Exec("UPDATE submissions SET submitted_at=? WHERE job_id=?", at.UTC().Format(time.RFC3339), id); err != nil {
			t.Fatal(err)
		}
	}
	// Re-recording one of today's jobs adds history, not an extra job of progress.
	if err := a.recordSubmission(2, "Second history record", reviewedSubmissionToken(t, a, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec("UPDATE submissions SET submitted_at=? WHERE job_id=2", start.UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	count, err := a.applicationsRecordedToday(now)
	if err != nil || count != 2 {
		t.Fatalf("recorded today=%d err=%v", count, err)
	}
	page := summarizeJobs([]Job{{SubmissionCount: 1}, {Stage: StageReview}, {Stage: StageReady}, {Stage: StageClosed}})
	if page.QueueTotal() != 3 || page.RecordedPercent() != 33 {
		t.Fatalf("closed jobs distorted queue: %+v", page)
	}
	if (opportunityPage{}).RecordedPercent() != 0 {
		t.Fatal("empty queue must have zero progress")
	}
}

func TestOpportunityDashboardEmptyAndCompleted(t *testing.T) {
	a, _ := testWorkspace(t)
	get := func() string {
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs", nil))
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	empty := get()
	if strings.Contains(empty, `class="queue-progress-bar"`) || strings.Contains(empty, "data-job-filters") ||
		strings.Count(empty, `action="/jobs"`) != 1 || !strings.Contains(empty, `class="panel create-panel first-job-form"`) ||
		!strings.Contains(empty, `>Save job</button>`) {
		t.Fatal("empty workspace must lead with one open save form and no empty progress or filters")
	}
	if err := a.saveJob(Job{URL: "https://example.test/complete"}); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	done := get()
	for _, want := range []string{"100% recorded", "CURRENT QUEUE COMPLETE", `data-recorded-today>1</strong>`, `data-remaining-count><strong>0</strong>`} {
		if !strings.Contains(done, want) {
			t.Fatalf("complete dashboard missing %s", want)
		}
	}
}
