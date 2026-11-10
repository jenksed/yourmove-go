package app

import (
	"context"
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
