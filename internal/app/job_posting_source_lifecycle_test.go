package app

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

func richTestMetadata(t *testing.T, canonical, salary string) platforms.Metadata {
	t.Helper()
	identity, err := platforms.AshbyPostingIdentity(testAshbyPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	source, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity: identity, DescriptionHTML: "<h2>Role</h2><p>" + canonical + "</p>",
		DescriptionPlain: canonical, Location: "Remote", WorkplaceType: "Remote",
		EmploymentType: "FullTime", SalarySummary: salary,
	}, canonical)
	if err != nil {
		t.Fatal(err)
	}
	return platforms.Metadata{
		Title: "Platform Engineer", Company: "Synthetic Co", Description: canonical,
		SalaryText: salary, SalaryOrigin: "employer", Source: platforms.AvailablePostingSource(source),
	}
}

func postSyntheticJob(t *testing.T, a *App, postingURL string) *httptest.ResponseRecorder {
	t.Helper()
	return postApp(newMux(a), "/jobs", url.Values{"job_url": {postingURL}}, "", "")
}

func TestAshbyIntakeStoresSourceAtomicallyAndDuplicateDoesNotReplaceIt(t *testing.T) {
	a, _ := testWorkspace(t)
	fetches := 0
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		fetches++
		if fetches == 1 {
			return richTestMetadata(t, "First description", "€120K - €150K"), "retrieved", "source available"
		}
		return richTestMetadata(t, "Duplicate must not replace", "€1 - €2"), "retrieved", "different source"
	}
	first := postSyntheticJob(t, a, testAshbyPostingURL)
	if first.Code != 303 || first.Header().Get("Location") != "/jobs" {
		t.Fatalf("intake response = %d %q %s", first.Code, first.Header().Get("Location"), first.Body.String())
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Invalid || state.Source == nil || state.Source.DescriptionPlain != "First description" {
		t.Fatalf("source after intake: found=%v state=%+v err=%v", found, state, err)
	}
	firstDigest := state.SourceContentSHA256

	duplicate := postSyntheticJob(t, a, testAshbyPostingURL)
	if duplicate.Code != 303 || fetches != 1 {
		t.Fatalf("duplicate intake fetched or failed: code=%d fetches=%d", duplicate.Code, fetches)
	}
	job, err = a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err = loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Source == nil || state.SourceContentSHA256 != firstDigest || job.Description != "First description" {
		t.Fatalf("duplicate intake replaced canonical/source: job=%+v state=%+v err=%v", job, state, err)
	}
}

func TestAshbyIntakeSourceWriteFailureRollsBackNewJob(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return richTestMetadata(t, "Must not partially save", "€120K - €150K"), "retrieved", "source available"
	}
	if _, err := a.db.Exec(`CREATE TRIGGER reject_posting_source BEFORE INSERT ON job_posting_sources BEGIN SELECT RAISE(ABORT, 'injected source failure'); END`); err != nil {
		t.Fatal(err)
	}
	response := postSyntheticJob(t, a, testAshbyPostingURL)
	if response.Code != 500 {
		t.Fatalf("intake response = %d, want source-write failure", response.Code)
	}
	var jobs, sources int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM job_posting_sources`).Scan(&sources); err != nil {
		t.Fatal(err)
	}
	if jobs != 0 || sources != 0 {
		t.Fatalf("failed intake left partial state: jobs=%d sources=%d", jobs, sources)
	}
}

func TestPostingRefreshSeparatesLatestAttemptFromLatestSuccessAndPreservesManualValues(t *testing.T) {
	a, _ := testWorkspace(t)
	initial := richTestMetadata(t, "First canonical", "€120K - €150K")
	if err := a.saveJob(Job{URL: testAshbyPostingURL, Host: "jobs.ashbyhq.com", Title: "Platform Engineer", Description: initial.Description, SalaryText: initial.SalaryText, SalaryOrigin: initial.SalaryOrigin}); err != nil {
		t.Fatal(err)
	}
	if err := a.applyPostingRefresh(1, initial, "retrieved", "first success", "2026-10-24T12:00:00Z"); err != nil {
		t.Fatal(err)
	}

	failed := richTestMetadata(t, "Failed retrieval must be ignored", "$1 - $2")
	if err := a.applyPostingRefresh(1, failed, "failed", "offline", "2026-10-24T12:01:00Z"); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Invalid || state.Source == nil || state.Source.DescriptionPlain != "First canonical" || state.LatestSuccessAt != "2026-10-24T12:00:00Z" {
		t.Fatalf("failed attempt replaced latest success: job=%+v source=%+v found=%v err=%v", job, state, found, err)
	}
	if job.SourceCheckedAt != "2026-10-24T12:01:00Z" || job.Description != "First canonical" || job.SalaryText != "€120K - €150K" {
		t.Fatalf("failed attempt changed canonical values or check time: %+v", job)
	}

	canonicalOnly := platforms.Metadata{Description: "Second canonical"}
	if err := a.applyPostingRefresh(1, canonicalOnly, "retrieved", "rich source absent", "2026-10-24T12:02:00Z"); err != nil {
		t.Fatal(err)
	}
	job, err = a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err = loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Invalid || state.Source != nil || state.Availability != platforms.PostingSourceUnavailable || state.UnavailableReason != platforms.PostingSourceAbsent || state.LatestSuccessAt != "2026-10-24T12:02:00Z" {
		t.Fatalf("canonical-only success did not supersede old rich source: state=%+v found=%v err=%v", state, found, err)
	}
	if job.Description != "Second canonical" || job.SalaryText != "€120K - €150K" || !state.DescriptionConfirmed || state.SalaryConfirmed {
		t.Fatalf("absent salary confirmation or canonical retention is wrong: job=%+v state=%+v", job, state)
	}

	job.Description = "Applicant corrected description"
	job.SalaryText = "Applicant corrected salary"
	if err := a.updateJob(job); err != nil {
		t.Fatal(err)
	}
	manualRefresh := richTestMetadata(t, "Third employer description", "$160K - $190K")
	if err := a.applyPostingRefresh(1, manualRefresh, "retrieved", "fresh source", "2026-10-24T12:03:00Z"); err != nil {
		t.Fatal(err)
	}
	job, err = a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	state, found, err = loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Invalid || state.Source == nil {
		t.Fatalf("manual refresh lost retained employer source: state=%+v found=%v err=%v", state, found, err)
	}
	if job.Description != "Applicant corrected description" || job.SalaryText != "Applicant corrected salary" || state.DescriptionConfirmed || state.SalaryConfirmed || state.Source.CanonicalDescriptionSHA256 == platforms.CanonicalDescriptionDigest(job.Description) {
		t.Fatalf("refresh overwrote/manual-bound applicant values incorrectly: job=%+v state=%+v", job, state)
	}
}

func TestPostingRefreshSourceWriteFailureRollsBackCanonicalUpdate(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPostingURL, Host: "jobs.ashbyhq.com", Title: "Original", Description: "Original description", SalaryText: "Original salary", SourceStatus: "failed", SourceNote: "old note", SourceCheckedAt: "2026-10-24T11:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`CREATE TRIGGER reject_posting_source BEFORE INSERT ON job_posting_sources BEGIN SELECT RAISE(ABORT, 'injected source failure'); END`); err != nil {
		t.Fatal(err)
	}
	metadata := richTestMetadata(t, "Replacement description", "Replacement salary")
	err := a.applyPostingRefresh(1, metadata, "retrieved", "new note", "2026-10-24T12:00:00Z")
	if err == nil {
		t.Fatal("source write failure was ignored")
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	var sourceCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM job_posting_sources WHERE job_id=1`).Scan(&sourceCount); err != nil {
		t.Fatal(err)
	}
	if job.Title != "Original" || job.Description != "Original description" || job.SalaryText != "Original salary" || job.SourceStatus != "failed" || job.SourceNote != "old note" || job.SourceCheckedAt != "2026-10-24T11:00:00Z" || sourceCount != 0 {
		t.Fatalf("refresh write failure left partial canonical/source state: job=%+v sources=%d", job, sourceCount)
	}
}

func TestDeletingJobDeletesPostingSourceInSameLifecycle(t *testing.T) {
	a, _ := testWorkspace(t)
	canonical := "Saved description"
	jobID := saveTestJob(t, a, canonical)
	if err := saveJobPostingSource(a.db, jobID, testAshbyPostingURL, canonical, testPostingSuccessAt,
		platforms.AvailablePostingSource(sealedTestPostingSource(t, canonical)), true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteJobs([]int64{jobID}, false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM job_posting_sources WHERE job_id=?`, jobID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("deleted job retained %d posting source rows", count)
	}
}
