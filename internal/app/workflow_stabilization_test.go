package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestEarlySavedAnswersStayExcludedUntilReconciledAndAuthorized(t *testing.T) {
	a, _ := testWorkspace(t)
	job, public := earlyGreenhouseFixture(t, a)
	second := public.Fields[0]
	// Use independently identified narrative questions with exact public/live
	// identity rules. Approval of one never grants authority to the other.
	for _, f := range public.Fields {
		if f.ID == "application:story" {
			second = f
		}
	}
	second.ID = "application:other-story"
	second.Label = "Describe a technical investigation"
	public.Fields = append(public.Fields, second)
	if _, err := a.saveInspection(job.ID, public); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	for _, id := range []string{"application:story", "application:other-story"} {
		q := earlyQuestionByID(t, a, job, id, "")
		if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", "Synthetic saved answer for "+id), "", ""); rec.Code != 303 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	live := schema.Result{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fields: []schema.Field{{ID: "browser:story", Label: "Describe a time you improved a system", Kind: "textarea", Required: true, Group: "rendered form"}, {ID: "browser:other-story", Label: second.Label, Kind: second.Kind, Required: second.Required, Group: "rendered form"}}}
	if _, err := a.saveInspection(job.ID, live); err != nil {
		t.Fatal(err)
	}
	early, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(early.AwaitingReconciliation()) != 2 {
		t.Fatalf("missing preserved drafts: %+v", early)
	}
	prepared, _ := a.prepareQuestions(job, CandidateProfile{})
	if len(prepared.ReadyAnswers()) != 0 {
		t.Fatal("early drafts silently became eligible")
	}
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	for _, copy := range []string{"2 saved answers awaiting reconciliation", "excluded from transfer", "Use prepared answer for this question", "Complete candidate profile"} {
		if !strings.Contains(page.Body.String(), copy) {
			t.Fatalf("missing actionable scope %q", copy)
		}
	}
	for _, q := range early.Questions {
		if q.Revision == 0 {
			continue
		}
		if !q.Reconciliation.CanAccept {
			t.Fatalf("fixture not reconciliable: %+v", q)
		}
		values := url.Values{"early_answer_id": {fmt.Sprint(q.EarlyAnswerID)}, "early_revision": {fmt.Sprint(q.Revision)}, "destination": {job.URL}, "live_inspection_fingerprint": {q.Reconciliation.LiveInspectionFingerprint}, "live_question_key": {q.Reconciliation.LiveQuestionKey}, "live_answer_revision": {fmt.Sprint(q.Reconciliation.LiveAnswerRevision)}}
		if rec := postApp(mux, "/jobs/1/answers/early-accept", values, "", ""); rec.Code != 303 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	prepared, _ = a.prepareQuestions(job, CandidateProfile{})
	if len(prepared.ReadyAnswers()) != 2 || len(a.browsers.answers) != 0 {
		t.Fatal("acceptance lost answers or authorized filling")
	}
	fake := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: job.URL}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fake, nil }
	if err = a.browsers.start(context.Background(), 1, job.URL, ""); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	if len(fake.transferred) != 0 {
		t.Fatal("opening browser transmitted answers")
	}
	if rec := postApp(mux, "/jobs/1/answers/transfer", url.Values{"destination": {job.URL}, "review_hash": {prepared.ReviewHash()}}, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	a.browsers.tryAnswers(context.Background(), 1)
	if len(fake.transferred) != 2 {
		t.Fatal("separate explicit transfer did not include both accepted answers")
	}
}
