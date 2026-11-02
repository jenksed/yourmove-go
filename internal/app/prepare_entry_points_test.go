package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestPrepareWritingEntryPointsUseCurrentQuestionSourceAndExplainLetterSources(t *testing.T) {
	a, _ := testWorkspace(t)
	job := Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io", Title: "Synthetic Engineer"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	get := func() string {
		t.Helper()
		w := httptest.NewRecorder()
		newMux(a).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("job page status %d: %s", w.Code, w.Body.String())
		}
		return w.Body.String()
	}

	page := get()
	for _, want := range []string{
		`id="answers" data-workspace-panel="answers"`,
		`id="application-answers-workspace" name="job-writing-activity" data-writing-activity data-writing-activity-key="answers" open`,
		`id="letter-editor" name="job-writing-activity" data-writing-activity data-writing-activity-key="letter"`,
		`<nav class="prepare-entry-actions" aria-label="Writing tasks">`,
		`href="#early-application-questions" data-prepare-question-entry`,
		`<strong>Prepare answers</strong>`,
		`href="#letter-editor" data-open-details="letter-editor"><strong>Write cover letter</strong>`,
		"Optional · write or paste a letter, with AI assistance available separately",
		"Designate an active master resume in the document library.",
		"Choose an active role-specific resume for this job.",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("initial Prepare page omitted %q", want)
		}
	}

	if _, err := a.saveInspection(1, schema.Result{
		Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL,
		Fields: []schema.Field{{ID: "application:why", Label: "Why this role?", Kind: "textarea", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}
	page = get()
	for _, want := range []string{
		`href="#early-application-questions" data-prepare-question-entry`,
		`<strong>Answer questions</strong><small>1 need your input`,
		`id="early-application-questions" tabindex="-1"`,
		`action="/jobs/1/answers/early-save"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("discovered-question Prepare page omitted %q", want)
		}
	}
}

func TestWritingAnswerContextFollowsReturnSourceAndCurrentSnapshot(t *testing.T) {
	live := applicationQuestionSnapshot{Available: true, Anchor: "#application-questions"}
	early := applicationQuestionSnapshot{Available: true, Anchor: "#early-application-questions"}
	cases := []struct {
		name, url, want string
		snapshot        applicationQuestionSnapshot
	}{
		{name: "current live inventory", snapshot: live, want: "live"},
		{name: "early inventory fallback", snapshot: early, want: "early"},
		{name: "live save return", url: "http://localhost/jobs/1?question=exact-key#prepare", snapshot: early, want: "live"},
		{name: "early save return", url: "http://localhost/jobs/1?early_question=exact-key#prepare", snapshot: live, want: "early"},
		{name: "no inventory", want: "early"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var request *http.Request
			if tc.url != "" {
				request = httptest.NewRequest(http.MethodGet, tc.url, nil)
			}
			if got := writingAnswerContext(request, tc.snapshot); got != tc.want {
				t.Fatalf("writingAnswerContext() = %q, want %q", got, tc.want)
			}
		})
	}
}
