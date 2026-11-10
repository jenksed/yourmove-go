package app

import (
	"context"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

// A representative Recruitee posting must import with the useful public
// information available from its source, including on the bulk-import +
// enrichment path (not only single intake): title, employer, description,
// and the sealed rich source carrying location, workplace mode, and
// employment type. Salary stays honestly unavailable. Manual edits remain
// authoritative and are never overwritten.
func TestRecruiteeBulkEnrichmentPersistsPublicFacts(t *testing.T) {
	a, _ := testWorkspace(t)
	const postingURL = "https://example.recruitee.com/o/platform-engineer"
	const description = "Build safe services. Experience with Go."
	sealed, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity:       platforms.PostingIdentity{Provider: "recruitee", Board: "example.recruitee.com", PostingID: "platform-engineer"},
		DescriptionHTML: "<h2>Responsibilities</h2><p>Build safe services.</p><h2>Job requirements</h2><ul><li>Experience with Go.</li></ul>",
		Location:       "Berlin, Germany",
		WorkplaceType:  "Hybrid",
		EmploymentType: "fulltime_permanent",
	}, description)
	if err != nil {
		t.Fatal(err)
	}
	a.enrichmentFetch = func(_ context.Context, raw string) (platforms.Metadata, string, string) {
		if raw != postingURL {
			return platforms.Metadata{}, "unsupported", "Your job is saved. Copy the description from the employer page and paste it into Edit details."
		}
		return platforms.Metadata{
			Title: "Platform Engineer", Company: "Example Robotics", Description: description,
			// No salary in the public feed: honestly unavailable, never invented.
			Source: platforms.AvailablePostingSource(sealed),
		}, "retrieved", "Posting details returned by a supported source; application availability not verified."
	}
	batch, err := a.importJobBatch(postingURL + "\nhttps://unsupported.example.test/role")
	if err != nil {
		t.Fatal(err)
	}
	if batch.Added != 2 {
		t.Fatalf("batch=%+v", batch)
	}
	if _, err := a.startEnrichment("pending"); err != nil {
		t.Fatal(err)
	}
	snap := waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	if snap.Retrieved != 1 || snap.Unsupported != 1 {
		t.Fatalf("snapshot=%+v", snap)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Title != "Platform Engineer" || job.Company != "Example Robotics" || job.Description != description {
		t.Fatalf("projection lost: %+v", job)
	}
	if job.SalaryText != "" {
		t.Fatalf("invented salary: %q", job.SalaryText)
	}
	state, found, err := loadJobPostingSource(a.db, job)
	if err != nil || !found || state.Invalid || state.Source == nil {
		t.Fatalf("rich source missing: %+v found=%v err=%v", state, found, err)
	}
	if state.Source.Location != "Berlin, Germany" || state.Source.WorkplaceType != "Hybrid" || state.Source.EmploymentType != "fulltime_permanent" {
		t.Fatalf("public facts lost: %+v", state.Source)
	}
	if err := state.Source.Identity.ValidateURL(postingURL); err != nil {
		t.Fatalf("source identity detached: %v", err)
	}

	// Manual edits stay authoritative across a later enrichment pass.
	edited, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	edited.Title = "My edited title"
	if err := a.updateJob(edited); err != nil {
		t.Fatal(err)
	}
	if _, err := a.startEnrichment("failed"); err != nil {
		t.Fatal(err)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	kept, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Title != "My edited title" {
		t.Fatalf("manual edit overwritten: %+v", kept)
	}
	restored, found, err := loadJobPostingSource(a.db, kept)
	if err != nil || !found || restored.Invalid || restored.Source == nil {
		t.Fatalf("source evidence lost after manual edit: %+v", restored)
	}
}

// Retired Recruitee posting with no retrievable metadata (job 20 in the
// acceptance workspace) must show a readable slug fallback, not the entire
// URL, while stored metadata stays empty and source failure is preserved.
// The stale indexed copy ("Technical Support Engineer – Scientific
// Instruments" with en-dash) is never used as verified metadata, manual
// titles are never overwritten, and the posting is not auto-closed.
func TestRecruiteeUnavailableTitleFallbackFromSlug(t *testing.T) {
	a, _ := testWorkspace(t)
	const postingURL = "https://rbr.recruitee.com/o/technical-support-engineer-scientific-instruments"
	// Exact persisted state from the acceptance workspace: empty title,
	// failed source check, review stage.
	if err := a.saveJob(Job{
		URL: postingURL, Host: "rbr.recruitee.com", Title: "",
		SourceStatus: "failed", SourceNote: "Recruitee posting absent from public feed",
		SourceCheckedAt: "2026-11-10T06:12:23Z",
	}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Title != "" || job.TitleManual {
		t.Fatalf("stored title must stay empty without manual entry: %+v", job)
	}
	if job.SourceStatus != "failed" || job.SourceNote != "Recruitee posting absent from public feed" {
		t.Fatalf("source failure not preserved: %+v", job)
	}
	if job.Stage != StageReview {
		t.Fatalf("unavailable posting auto-classified: stage=%q", job.Stage)
	}

	out := toOpportunityJSON(job)
	if out.Title != "" {
		t.Fatalf("verified Title must stay empty, got %q", out.Title)
	}
	if out.DisplayTitle == postingURL {
		t.Fatalf("DisplayTitle shows entire URL: %q", out.DisplayTitle)
	}
	if strings.Contains(out.DisplayTitle, "https://") || strings.Contains(out.DisplayTitle, "rbr.recruitee.com/o/") {
		t.Fatalf("DisplayTitle leaks URL: %q", out.DisplayTitle)
	}
	// Readable slug fallback, not the stale indexed copy with en-dash.
	if out.DisplayTitle != "Technical Support Engineer Scientific Instruments" {
		t.Fatalf("DisplayTitle=%q, want slug fallback %q", out.DisplayTitle, "Technical Support Engineer Scientific Instruments")
	}
	if out.DisplayTitle == "Technical Support Engineer – Scientific Instruments" {
		t.Fatal("stale third-party search result used as verified metadata")
	}
	if out.URL != postingURL {
		t.Fatalf("posting URL not preserved: %q", out.URL)
	}
	if out.SourceStatus != "failed" {
		t.Fatalf("source failure not preserved in JSON: %+v", out)
	}
	if out.Provider != "recruitee" {
		t.Fatalf("provider lost: %+v", out)
	}

	workspace := toWorkspaceJob(job)
	if workspace.DisplayTitle != out.DisplayTitle {
		t.Fatalf("workspace DisplayTitle=%q, opportunities DisplayTitle=%q", workspace.DisplayTitle, out.DisplayTitle)
	}
	if workspace.Title != "" {
		t.Fatalf("workspace verified Title must stay empty: %+v", workspace)
	}

	// Verified and manual titles still pass through unchanged.
	verified := Job{URL: "https://deephealth.recruitee.com/o/technical-support-engineer-1", Host: "deephealth.recruitee.com", Title: "Technical Support Engineer"}
	if got := toOpportunityJSON(verified).DisplayTitle; got != "Technical Support Engineer" {
		t.Fatalf("verified title overwritten: %q", got)
	}
	manual := Job{URL: postingURL, Host: "rbr.recruitee.com", Title: "My edited title", TitleManual: true}
	if got := toOpportunityJSON(manual).DisplayTitle; got != "My edited title" {
		t.Fatalf("manual title overwritten: %q", got)
	}
}
