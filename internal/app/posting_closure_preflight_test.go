package app

// KAN-16 closure preflight qualification. Three deterministic synthetic
// fixtures exercise the actual Go HTTP contracts (availability endpoint,
// browser launch, continuation, legacy transfers) with an injected posting
// checker, so no live ATS traffic is ever required:
//
//   - kan16OpenFixtureURL: supported provider, posting available.
//   - kan16ClosedFixtureURL: supported provider, provider-confirmed closure.
//   - kan16UncertainFixtureURL: supported provider, inconclusive lookup.
//
// All fixtures use isolated synthetic YOURMOVE_DATA_DIR workspaces via
// testWorkspace; the suite never touches real applicant data.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

const (
	kan16OpenFixtureURL      = "https://job-boards.greenhouse.io/kan16open/jobs/42424242"
	kan16ClosedFixtureURL    = "https://job-boards.greenhouse.io/kan16closed/jobs/43434343"
	kan16UncertainFixtureURL = "https://job-boards.greenhouse.io/kan16uncertain/jobs/44444444"
)

// kan16FixtureFetch is the deterministic provider stand-in. Only the exact
// "closed" outcome counts as trusted closure evidence; every other outcome
// must leave the job active.
func kan16FixtureFetch(status, note string) postingFetcher {
	return func(context.Context, string) (platforms.Metadata, string, string) {
		if status == "retrieved" {
			return platforms.Metadata{Title: "Synthetic Open Role", Company: "Fixture Employer", Description: "Synthetic description."}, status, note
		}
		return platforms.Metadata{}, status, note
	}
}

func kan16SeedJob(t *testing.T, a *App, rawURL string) {
	t.Helper()
	if err := a.saveJob(Job{
		URL: rawURL, Host: "job-boards.greenhouse.io",
		Title: "Synthetic role", TitleManual: true,
		Company: "Fixture Employer", CompanyManual: true,
		Description: "Applicant-saved description.", DescriptionManual: true,
		SourceStatus: "not_checked",
	}); err != nil {
		t.Fatal(err)
	}
}

func kan16PostJSON(a *App, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "http://localhost"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	return rec
}

func TestClosurePreflightBlocksLaunchAndAllTransfers(t *testing.T) {
	a, _ := testWorkspace(t)
	kan16SeedJob(t, a, kan16ClosedFixtureURL)
	a.enrichmentFetch = kan16FixtureFetch("closed", "synthetic provider-confirmed closure")
	resume, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nsynthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		return &fakeBrowser{url: kan16ClosedFixtureURL}, nil
	}

	// Single click on View Application: availability reports closed and
	// persists the retirement before anything launches.
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/availability", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("availability: %d %s", rec.Code, rec.Body.String())
	}
	var availability struct {
		State         string `json:"state"`
		Notice        string `json:"notice"`
		Closed        bool   `json:"closed"`
		PostingClosed bool   `json:"postingClosed"`
		ClosureNotice string `json:"closureNotice"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &availability); err != nil {
		t.Fatal(err)
	}
	if availability.State != "closed" || availability.Notice != postingClosedNotice || !availability.Closed || !availability.PostingClosed || availability.ClosureNotice != postingClosedNotice {
		t.Fatalf("availability did not report the persistent closure: %s", rec.Body.String())
	}

	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Stage != StageClosed || job.SourceStatus != "closed" || job.Attention() != "history" {
		t.Fatalf("closure not retired to History: %+v", job)
	}
	// Prep survives the retirement: manual edits, résumé selection, and
	// source evidence stay intact; nothing uploads.
	if job.Title != "Synthetic role" || !job.TitleManual || job.Description != "Applicant-saved description." || job.Company != "Fixture Employer" || job.SelectedResumeID != resume.ID {
		t.Fatalf("prep lost on closure: %+v", job)
	}

	// Pending launch is blocked with the persistent notice.
	if rec := kan16PostJSON(a, "/api/jobs/1/browser", `{"action":"start"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no longer accepting applications") {
		t.Fatalf("launch not blocked: %d %s", rec.Code, rec.Body.String())
	}
	if launches != 0 || a.browsers.state(1).Active {
		t.Fatalf("browser launched for a closed posting: launches=%d", launches)
	}
	// All transfers are blocked with the persistent notice: continuation,
	// legacy résumé handoff, answers, and cover letter.
	if rec := kan16PostJSON(a, "/api/jobs/1/continue", `{"destination":"`+kan16ClosedFixtureURL+`"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no longer accepting applications") {
		t.Fatalf("continue not blocked: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postApp(newMux(a), "/jobs/1/handoff", url.Values{}, "", ""); rec.Code != http.StatusConflict {
		t.Fatalf("resume handoff not blocked: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postApp(newMux(a), "/jobs/1/answers/transfer", url.Values{}, "", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no longer accepting applications") {
		t.Fatalf("answer transfer not blocked: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postApp(newMux(a), "/jobs/1/cover-letter/transfer", url.Values{}, "", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "no longer accepting applications") {
		t.Fatalf("letter transfer not blocked: %d %s", rec.Code, rec.Body.String())
	}
	if launches != 0 {
		t.Fatalf("transfer armed a browser for a closed posting: launches=%d", launches)
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("closure minted %d submission reviews", got)
	}
}

func TestClosurePreflightOpenProceedsAutomatically(t *testing.T) {
	a, _ := testWorkspace(t)
	kan16SeedJob(t, a, kan16OpenFixtureURL)
	a.enrichmentFetch = kan16FixtureFetch("retrieved", "synthetic open posting")
	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		return &fakeBrowser{url: kan16OpenFixtureURL}, nil
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/availability", nil))
	var availability struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &availability); err != nil || availability.State != "open" {
		t.Fatalf("open fixture not available: %d %s %v", rec.Code, rec.Body.String(), err)
	}
	if rec := kan16PostJSON(a, "/api/jobs/1/browser", `{"action":"start"}`); rec.Code != http.StatusOK {
		t.Fatalf("open launch blocked: %d %s", rec.Code, rec.Body.String())
	}
	if launches != 1 {
		t.Fatalf("open launch did not start exactly one browser: %d", launches)
	}
	defer a.browsers.closeAll()
	if job, err := a.jobByID(1); err != nil || job.Stage == StageClosed {
		t.Fatalf("open preflight retired the job: %+v %v", job, err)
	}
	// Continue passes the preflight and reaches its ordinary material
	// validation (no transfer material selected here).
	if rec := kan16PostJSON(a, "/api/jobs/1/continue", `{"destination":"`+kan16OpenFixtureURL+`"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("open continue stopped at preflight: %d %s", rec.Code, rec.Body.String())
	}
}

func TestClosurePreflightUncertainNeverCloses(t *testing.T) {
	for _, outcome := range []struct {
		name   string
		status string
		note   string
	}{
		{"timeout", "failed", "synthetic timeout"},
		{"auth", "failed", "posting API returned HTTP 401"},
		{"missing", "failed", "posting API returned HTTP 500"},
		{"unsupported", "unsupported", "synthetic unsupported provider"},
		{"ambiguous", "failed", "synthetic ambiguous response"},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			a, _ := testWorkspace(t)
			kan16SeedJob(t, a, kan16UncertainFixtureURL)
			a.enrichmentFetch = kan16FixtureFetch(outcome.status, outcome.note)
			launches := 0
			a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
				launches++
				return &fakeBrowser{url: kan16UncertainFixtureURL}, nil
			}
			rec := httptest.NewRecorder()
			newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/availability", nil))
			var availability struct {
				State  string `json:"state"`
				Detail string `json:"detail"`
				Closed bool   `json:"closed"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &availability); err != nil {
				t.Fatal(err)
			}
			if availability.State != "unknown" || availability.Closed || strings.TrimSpace(availability.Detail) == "" {
				t.Fatalf("uncertain outcome misreported: %s", rec.Body.String())
			}
			job, err := a.jobByID(1)
			if err != nil {
				t.Fatal(err)
			}
			if job.Stage == StageClosed || job.Attention() != "focus" {
				t.Fatalf("uncertain outcome retired the job: %+v", job)
			}
			// The ordinary flow stays available: launch proceeds and the
			// existing manual/managed fallback is preserved.
			if rec := kan16PostJSON(a, "/api/jobs/1/browser", `{"action":"start"}`); rec.Code != http.StatusOK {
				t.Fatalf("uncertain launch blocked: %d %s", rec.Code, rec.Body.String())
			}
			defer a.browsers.closeAll()
			if launches != 1 {
				t.Fatalf("uncertain launch did not proceed: %d", launches)
			}
		})
	}
}

func TestClosureRefreshPreservesAppliedAndHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	kan16SeedJob(t, a, kan16OpenFixtureURL)
	if err := a.recordSubmission(1, "applicant submitted on the employer site", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	a.enrichmentFetch = kan16FixtureFetch("closed", "synthetic late closure")
	if err := a.applyPostingRefresh(1, platforms.Metadata{}, "closed", "synthetic late closure", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Stage != StageApplied || job.SubmissionCount != 1 {
		t.Fatalf("closure overwrote Applied: %+v", job)
	}
	if job.SourceStatus != "closed" {
		t.Fatalf("closure evidence not recorded: %+v", job)
	}
	if job.Title != "Synthetic role" || job.Description != "Applicant-saved description." || !job.TitleManual {
		t.Fatalf("closure lost Applied prep: %+v", job)
	}
	submissions, err := a.jobSubmissions(1)
	if err != nil || len(submissions) != 1 {
		t.Fatalf("submission history lost: %+v %v", submissions, err)
	}
	if job.Attention() != "history" {
		t.Fatalf("Applied job left History: %+v", job)
	}
	// A late closure probe against recorded history never reopens or
	// re-blocks the completed application.
	if _, block := a.ensurePostingOpen(context.Background(), 1); block == "" {
		t.Fatal("completed application lost its transfer guard")
	}
	if strings.Contains(closureNoticeFor(job), "moved it to History") {
		t.Fatal("Applied job claims the closure notice")
	}
}

func TestRoutineRecheckEligibilityWindows(t *testing.T) {
	a, _ := testWorkspace(t)
	aged := func(d time.Duration) string { return time.Now().UTC().Add(-d).Format(time.RFC3339) }
	seed := func(url, parkedAt, checkedAt, status string) {
		t.Helper()
		if err := a.saveJob(Job{URL: url, Host: "job-boards.greenhouse.io", Title: "Routine", SourceStatus: status, SourceCheckedAt: checkedAt, ParkedAt: parkedAt}); err != nil {
			t.Fatal(err)
		}
	}
	seed("https://job-boards.greenhouse.io/routine/focus-due/jobs/1", "", aged(25*time.Hour), "retrieved")
	seed("https://job-boards.greenhouse.io/routine/focus-fresh/jobs/2", "", aged(time.Hour), "retrieved")
	seed("https://job-boards.greenhouse.io/routine/saved-due/jobs/3", aged(30*24*time.Hour), aged(8*24*time.Hour), "retrieved")
	seed("https://job-boards.greenhouse.io/routine/saved-fresh/jobs/4", aged(30*24*time.Hour), aged(24*time.Hour), "retrieved")
	seed("https://job-boards.greenhouse.io/routine/never-checked/jobs/5", "", "", "not_checked")
	seed("https://job-boards.greenhouse.io/routine/unsupported/jobs/6", aged(30*24*time.Hour), aged(30*24*time.Hour), "unsupported")
	if err := a.saveJob(Job{URL: "https://job-boards.greenhouse.io/routine/applied/jobs/7", Host: "job-boards.greenhouse.io", Title: "Applied", SourceStatus: "retrieved", SourceCheckedAt: aged(30 * 24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(7, "recorded", reviewedSubmissionToken(t, a, 7)); err != nil {
		t.Fatal(err)
	}
	a.enrichmentFetch = kan16FixtureFetch("retrieved", "synthetic routine recheck")
	snapshot, err := a.startEnrichment("routine")
	if err != nil {
		t.Fatal(err)
	}
	// Eligible: stale Focus, stale Saved, never-checked Focus. Skipped:
	// fresh windows, unsupported sources, Applied-protected rows.
	if snapshot.Total != 3 {
		t.Fatalf("routine eligibility: %+v", snapshot)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	for _, id := range []int64{1, 3, 5} {
		job, err := a.jobByID(id)
		if err != nil || job.SourceStatus != "retrieved" || strings.TrimSpace(job.SourceCheckedAt) == "" {
			t.Fatalf("routine did not checkpoint job %d: %+v %v", id, job, err)
		}
	}
}

func TestRoutineRecheckRetiresConfirmedClosed(t *testing.T) {
	a, _ := testWorkspace(t)
	kan16SeedJob(t, a, kan16ClosedFixtureURL)
	a.enrichmentFetch = kan16FixtureFetch("closed", "synthetic routine closure")
	snapshot, err := a.startEnrichment("routine")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Total != 1 {
		t.Fatalf("routine skipped an eligible Focus job: %+v", snapshot)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Stage != StageClosed || job.Attention() != "history" || closureNoticeFor(job) != postingClosedNotice {
		t.Fatalf("routine did not retire the confirmed closure: %+v", job)
	}
	if job.Title != "Synthetic role" || job.Description != "Applicant-saved description." {
		t.Fatalf("routine lost prep: %+v", job)
	}
}

// An unretired saved Greenhouse error-board redirect carries deterministic
// provider closure evidence without any network: the same redirect intake and
// enrichment already retire. The single-click preflight must reach that
// verdict instead of reporting unknown behind the posting-ownership gate.
// A bare board URL without the redirect stays unknown and active.
func TestClosurePreflightUnretiredBoardRedirectCloses(t *testing.T) {
	a, _ := testWorkspace(t)
	kan16SeedJob(t, a, "https://job-boards.greenhouse.io/kan16redirect?error=true")
	if err := a.saveJob(Job{
		URL: "https://job-boards.greenhouse.io/kan16redirect", Host: "job-boards.greenhouse.io",
		Title: "Bare board", SourceStatus: "not_checked",
	}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/availability", nil))
	var availability struct {
		State         string `json:"state"`
		Notice        string `json:"notice"`
		Closed        bool   `json:"closed"`
		PostingClosed bool   `json:"postingClosed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &availability); err != nil {
		t.Fatal(err)
	}
	if availability.State != "closed" || availability.Notice != postingClosedNotice || !availability.Closed || !availability.PostingClosed {
		t.Fatalf("board redirect did not report the persistent closure: %d %s", rec.Code, rec.Body.String())
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Stage != StageClosed || job.SourceStatus != "closed" || job.Attention() != "history" {
		t.Fatalf("board redirect not retired to History: %+v", job)
	}
	if job.Title != "Synthetic role" || !job.TitleManual || job.Description != "Applicant-saved description." {
		t.Fatalf("prep lost on redirect closure: %+v", job)
	}

	rec = httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/2/availability", nil))
	var control struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &control); err != nil {
		t.Fatal(err)
	}
	if control.State != "unknown" {
		t.Fatalf("bare board without redirect must stay unknown: %d %s", rec.Code, rec.Body.String())
	}
	if job, err := a.jobByID(2); err != nil || job.Stage == StageClosed {
		t.Fatalf("control job retired: %+v %v", job, err)
	}
}
