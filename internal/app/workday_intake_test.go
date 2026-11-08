package app

import (
	"context"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

func TestWorkdayFailedIntakeRefreshRecoversMetadataWithoutReplacingApplicantWork(t *testing.T) {
	a, _ := testWorkspace(t)
	const postingURL = "https://crowdstrike.wd5.myworkdayjobs.com/crowdstrikecareers/job/usa---remote/technical-support_r30773?utm_source=chatgpt.com"
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "failed", "Workday posting identity mismatch"
	}
	id, duplicate, err := a.savePostingURL(context.Background(), postingURL, "")
	if err != nil || duplicate || id <= 0 {
		t.Fatalf("failed Workday intake id=%d duplicate=%v err=%v", id, duplicate, err)
	}
	saved, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SourceStatus != "failed" || !strings.Contains(saved.SourceNote, "identity mismatch") {
		t.Fatalf("intake did not retain observed failure: %+v", saved)
	}

	resume, err := a.importResume("selected.pdf", strings.NewReader("%PDF-1.4 synthetic resume"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(id, resume.ID); err != nil {
		t.Fatal(err)
	}
	saved, err = a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	saved.Title = "My edited title"
	saved.Description = "My saved application notes"
	saved.Stage = StageReview
	saved.Priority = PriorityHigh
	if err := a.updateJob(saved); err != nil {
		t.Fatal(err)
	}
	_ = reviewedSubmissionToken(t, a, id)
	preRefreshReviewCount := reviewCount(t, a)

	a.enrichmentFetch = func(_ context.Context, rawURL string) (platforms.Metadata, string, string) {
		if rawURL != postingURL {
			t.Errorf("refresh URL = %q, want original posting URL", rawURL)
		}
		return platforms.Metadata{Title: "Technical Support", Company: "CrowdStrike, Inc.", Description: "Employer description returned by Workday."}, "retrieved", "posting details returned"
	}
	saved, err = a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.refreshPosting(context.Background(), saved); err != nil {
		t.Fatal(err)
	}
	got, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id || got.URL != postingURL || got.SourceStatus != "retrieved" || got.Title != "My edited title" ||
		got.Description != "My saved application notes" || got.Company != "CrowdStrike, Inc." ||
		got.SelectedResumeID != resume.ID || got.Stage != StageReview || got.Priority != PriorityHigh {
		t.Fatalf("Workday refresh lost source or applicant-owned state: %+v", got)
	}
	if gotReviews := reviewCount(t, a); gotReviews != preRefreshReviewCount {
		t.Fatalf("refresh changed applicant review history: before=%d after=%d", preRefreshReviewCount, gotReviews)
	}
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "failed", "Workday HTTP 503"
	}
	if err := a.refreshPosting(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	afterTransientFailure, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if afterTransientFailure.Stage != StageReview || afterTransientFailure.SourceStatus != "failed" || afterTransientFailure.Title != got.Title || afterTransientFailure.Description != got.Description {
		t.Fatalf("transient Workday failure damaged recovered opportunity: %+v", afterTransientFailure)
	}

	const freshURL = "https://crowdstrike.wd5.myworkdayjobs.com/crowdstrikecareers/job/usa---remote/technical-support_r30773?fresh=1"
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return platforms.Metadata{Title: "Technical Support", Company: "CrowdStrike, Inc.", Description: "Employer description returned by Workday."}, "retrieved", "posting details returned"
	}
	freshID, duplicate, err := a.savePostingURL(context.Background(), freshURL, "")
	if err != nil || duplicate || freshID <= 0 {
		t.Fatalf("fresh Workday intake id=%d duplicate=%v err=%v", freshID, duplicate, err)
	}
	fresh, err := a.jobByID(freshID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Title != "Technical Support" || fresh.Company != "CrowdStrike, Inc." || fresh.Description != "Employer description returned by Workday." || fresh.SourceStatus != "retrieved" {
		t.Fatalf("fresh Workday intake did not populate employer metadata: %+v", fresh)
	}
}
