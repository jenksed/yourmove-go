package main

import (
	"context"
	"testing"

	"example.com/yourmove-lite/internal/platforms"
)

func TestClosedPostingIntakeAndRefresh(t *testing.T) {
	a, _ := testWorkspace(t)
	meta, status, note := fetchPostingDetails(context.Background(), "https://job-boards.greenhouse.io/tailscale?error=true")
	if status != "closed" || note == "" || meta != (platforms.Metadata{}) {
		t.Fatalf("closed source: %+v %s %s", meta, status, note)
	}
	if err := a.saveJob(Job{URL: "https://job-boards.greenhouse.io/tailscale?error=true", Host: "job-boards.greenhouse.io", Title: "Customer Support Engineer (Tier 1)", SourceStatus: status, SourceNote: note}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil || job.Stage != StageClosed || job.SourceStatusLabel() != "Closed — no longer hiring" {
		t.Fatalf("closed intake: %+v %v", job, err)
	}
	if err := a.saveJob(Job{URL: "https://job-boards.greenhouse.io/tailscale/jobs/123", Host: "job-boards.greenhouse.io", Title: "Saved title", Description: "Saved description", SourceStatus: "failed"}); err != nil {
		t.Fatal(err)
	}
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return meta, status, note
	}
	if _, err := a.startEnrichment("failed"); err != nil {
		t.Fatal(err)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	job, err = a.jobByID(2)
	if err != nil || job.Stage != StageClosed || job.SourceStatus != "closed" || job.Title != "Saved title" || job.Description != "Saved description" || job.SourceCheckedAt == "" {
		t.Fatalf("closed refresh: %+v %v", job, err)
	}
	if err := a.applyPostingRefresh(2, platforms.Metadata{}, "retrieved", "ok", "later"); err != nil {
		t.Fatal(err)
	}
	job, err = a.jobByID(2)
	if err != nil || job.Stage != StageClosed {
		t.Fatalf("refresh reopened closed job: %+v %v", job, err)
	}
}
