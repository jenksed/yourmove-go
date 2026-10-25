package app

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/jobdocument"
	"github.com/jenksed/yourmove-go/internal/platforms"
)

func TestJobReadingViewUsesEmployerDocumentAndDirectFacts(t *testing.T) {
	canonical := "Platform role\nBuild reliable services."
	source := sealedTestPostingSource(t, canonical)
	job := Job{
		URL: testAshbyPostingURL, Description: canonical, SalaryText: source.SalarySummary,
		SalaryOrigin: "employer", SourceStatus: "retrieved", SourceCheckedAt: testPostingSuccessAt,
	}
	state := jobPostingSourceState{
		LatestSuccessAt: testPostingSuccessAt, Availability: platforms.PostingSourceAvailable,
		Source: &source, DescriptionConfirmed: true, SalaryConfirmed: true,
	}
	view := buildJobReadingView(job, state, true)
	if !view.HasPrimary || view.PrimaryLabel != "Employer-provided description" || view.PrimaryDocument.Blocks[0].Kind != "heading" {
		t.Fatalf("employer document was not selected for the primary reading: %+v", view)
	}
	if view.Location != "Remote" || view.Workplace != "Remote" || view.EmploymentType != "Full time" {
		t.Fatalf("direct employer facts were not presented: %+v", view)
	}
	if view.SalaryNote != "Confirmed by the latest successful retrieval." || !view.HasCanonical || view.CanonicalText != job.DisplayDescription() {
		t.Fatalf("salary or canonical projection was changed: %+v", view)
	}
}

func TestJobReadingViewKeepsManualDescriptionAndSalaryPrimary(t *testing.T) {
	canonical := "Employer description"
	source := sealedTestPostingSource(t, canonical)
	job := Job{
		URL: testAshbyPostingURL, Description: "Applicant's edited description", DescriptionManual: true,
		SalaryText: "Applicant's salary correction", SalaryManual: true,
		SourceStatus: "failed", SourceCheckedAt: "2026-10-24T13:30:00Z",
	}
	state := jobPostingSourceState{
		LatestSuccessAt: testPostingSuccessAt, Availability: platforms.PostingSourceAvailable,
		Source: &source, DescriptionConfirmed: true, SalaryConfirmed: true,
	}
	view := buildJobReadingView(job, state, true)
	if !view.HasPrimary || view.PrimaryLabel != "Your saved description" || !view.HasEmployer || view.EmployerDocument.Blocks[0].Kind != "heading" {
		t.Fatalf("manual description did not remain primary with employer source secondary: %+v", view)
	}
	if view.SalaryNote != "Manually edited by you." || !view.HasEmployerSalary || view.EmployerSalary != source.SalarySummary {
		t.Fatalf("manual salary provenance was not retained: %+v", view)
	}
	if view.FreshnessLabel != "Earlier successful employer source retained" || view.FreshnessDetail == "" {
		t.Fatalf("failed check was presented as current: %+v", view)
	}
}

func TestJobReadingViewKeepsFactsWhenDescriptionIsAbsentAndIgnoresInvalidSource(t *testing.T) {
	identity, err := platforms.AshbyPostingIdentity(testAshbyPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity: identity, Location: "Berlin", WorkplaceType: "OnSite", EmploymentType: "Contract",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	job := Job{URL: testAshbyPostingURL, SourceStatus: "retrieved", SourceCheckedAt: testPostingSuccessAt}
	state := jobPostingSourceState{LatestSuccessAt: testPostingSuccessAt, Availability: platforms.PostingSourceAvailable, Source: &facts}
	view := buildJobReadingView(job, state, true)
	if view.HasPrimary || view.Location != "Berlin" || view.Workplace != "On-site" || view.EmploymentType != "Contract" {
		t.Fatalf("facts-only observation lost facts or invented a description: %+v", view)
	}
	state.Invalid = true
	view = buildJobReadingView(job, state, true)
	if view.Location != "" || view.Workplace != "" || view.EmploymentType != "" || view.FreshnessLabel != "Saved employer source could not be validated" {
		t.Fatalf("invalid source facts were displayed: %+v", view)
	}
}

func TestJobReadingViewLabelsUnknownProviderValuesWithoutInference(t *testing.T) {
	if got := workplaceArrangement("Flexible"); got != "Unrecognized provider value: Flexible" {
		t.Fatalf("unknown workplace type = %q", got)
	}
	if got := employmentTypeLabel("Seasonal-ish"); got != "Unrecognized provider value: Seasonal-ish" {
		t.Fatalf("unknown employment type = %q", got)
	}
}

func TestJobReadingViewPrioritizesJumpsAndPreservesOriginalSectionOrder(t *testing.T) {
	const canonical = "Senior Platform Engineer\n\nA canonical plain description for existing copy behavior."
	const rich = `<p>Posting preamble stays available.</p>` +
		`<h1>Senior Platform Engineer</h1><p>Opening content stays before the sections.</p>` +
		`<h2>Compensation</h2><p>$145,000–$185,000 per year.</p>` +
		`<h2>What you'll do</h2><ul><li>Improve reliability.</li></ul>` +
		`<h2>Equal employment opportunity</h2><p>Read the full policy before applying.</p>` +
		`<h2>Qualifications</h2><p>Production experience.</p>` +
		`<h2>How to apply</h2><p>Include a short work sample.</p>` +
		`<p>Closing accommodation instructions remain last.</p>`
	identity, err := platforms.AshbyPostingIdentity(testAshbyPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	source, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity: identity, DescriptionHTML: rich, DescriptionPlain: canonical,
	}, canonical)
	if err != nil {
		t.Fatal(err)
	}
	job := Job{URL: testAshbyPostingURL, Description: canonical}
	view := buildJobReadingView(job, jobPostingSourceState{
		Availability: platforms.PostingSourceAvailable, Source: &source,
	}, true)
	if view.CanonicalText != canonical {
		t.Fatalf("canonical description changed: %q", view.CanonicalText)
	}
	wantLabels := []string{"Responsibilities", "Qualifications", "Compensation", "Hiring process"}
	gotLabels := make([]string, 0, len(view.SectionJumps))
	for _, jump := range view.SectionJumps {
		gotLabels = append(gotLabels, jump.Label)
	}
	if !reflect.DeepEqual(gotLabels, wantLabels) {
		t.Fatalf("priority navigation labels = %v, want %v", gotLabels, wantLabels)
	}
	var flattened []jobdocument.Block
	for _, section := range view.Sections {
		flattened = append(flattened, section.Blocks...)
	}
	if !reflect.DeepEqual(flattened, view.PrimaryDocument.Blocks) {
		t.Fatal("focused section model changed, omitted, or duplicated original document blocks")
	}
	if len(view.Sections) < 6 || view.Sections[0].Kind != jobdocument.SectionUnknown {
		t.Fatalf("preamble and unknown sections were not retained: %+v", view.Sections)
	}
	ids := map[string]bool{}
	for _, section := range view.Sections {
		if section.AnchorID == "" {
			continue
		}
		if ids[section.AnchorID] {
			t.Fatalf("duplicate section anchor %q", section.AnchorID)
		}
		ids[section.AnchorID] = true
	}
	if len(ids) != 4 {
		t.Fatalf("anchor count = %d, want one per recognized heading: %v", len(ids), ids)
	}
}

func TestJobOverviewRendersPriorityAnchorsAndCompleteOriginalOrderWithoutJavaScript(t *testing.T) {
	const canonical = "Senior Platform Engineer\n\nCanonical copy remains plain and unchanged."
	const rich = `<p>Posting preamble.</p><h1>Senior Platform Engineer</h1><p>Opening content.</p>` +
		`<h2>Responsibilities</h2><p>Improve reliability.</p>` +
		`<h2>Benefits</h2><p>Health and retirement benefits.</p>` +
		`<h2>Application instructions</h2><p>Send a short work sample.</p>` +
		`<h2>Equal employment opportunity</h2><p>Review the complete policy.</p>` +
		`<p>Accommodation instructions follow the policy.</p>`
	identity, err := platforms.AshbyPostingIdentity(testAshbyPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	source, err := platforms.SealPostingSource(platforms.PostingSource{
		Identity: identity, DescriptionHTML: rich, DescriptionPlain: canonical,
	}, canonical)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := testWorkspace(t)
	job := Job{
		URL: testAshbyPostingURL, Host: "jobs.ashbyhq.com", Title: "Senior Platform Engineer",
		Company: "Fixture", Description: canonical, Stage: StageReview, Priority: PriorityNormal,
	}
	if _, added, err := a.storeJobIfNewWithPostingSource(job, platforms.AvailablePostingSource(source), testPostingSuccessAt, true, true); err != nil || !added {
		t.Fatalf("store synthetic reading fixture: added=%v err=%v", added, err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Overview status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<nav class="job-section-navigation" aria-label="Priority sections in the description">`,
		`href="#job-section-responsibilities-1"`,
		`href="#job-section-hiring-1"`,
		`id="job-section-responsibilities-1" tabindex="-1"`,
		`id="job-section-hiring-1" tabindex="-1"`,
		`id="canonical-description-copy" hidden readonly>Senior Platform Engineer`,
		"Review the complete policy.",
		"Accommodation instructions follow the policy.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Overview missing %q", want)
		}
	}
	for _, content := range []string{"Posting preamble.", "Opening content.", "Health and retirement benefits.", "Send a short work sample.", "Review the complete policy.", "Accommodation instructions follow the policy."} {
		if count := strings.Count(body, content); count != 1 {
			t.Errorf("source content %q appears %d times, want once", content, count)
		}
	}
	order := []string{"Posting preamble.", "Opening content.", "Improve reliability.", "Health and retirement benefits.", "Send a short work sample.", "Review the complete policy.", "Accommodation instructions follow the policy."}
	last := -1
	for _, content := range order {
		position := strings.Index(body, content)
		if position <= last {
			t.Errorf("content order for %q at %d is not after previous block position %d", content, position, last)
		}
		last = position
	}
}
