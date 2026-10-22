package app

import (
	"context"
	"net/url"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

type fakeAnswerBrowser struct {
	*fakeBrowser
	transferred []browser.AnswerValue
}

func (f *fakeAnswerBrowser) FillAnswer(_ context.Context, v browser.AnswerValue) (browser.AnswerOutcome, error) {
	f.transferred = append(f.transferred, v)
	return browser.AnswerOutcome{Label: v.Field.Label, Status: "filled", Detail: "Observed"}, nil
}
func TestAnswerTransferApprovalAndContinuation(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{{ID: "browser:why", Label: "Why this team?", Kind: "textarea"}, {ID: "browser:site", Label: "Website", Kind: "text"}}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	mux := newMux(a)
	for _, f := range fields {
		values := url.Values{"question_key": {questionKey(job.URL, "ashby", f)}, "revision": {"0"}, "answer": {"Approved for " + f.ID}, "destination": {job.URL}}
		if rec := postApp(mux, "/jobs/1/answers/save", values, "", ""); rec.Code != 303 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	f := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := a.browsers.start(context.Background(), 1, job.URL); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	prep, _ := a.prepareQuestions(job, CandidateProfile{})
	values := url.Values{"review_hash": {prep.ReviewHash()}, "destination": {job.URL}}
	values.Set("review_hash", "invalid")
	if rec := postApp(mux, "/jobs/1/answers/transfer", values, "", ""); rec.Code != 409 {
		t.Fatal("unreviewed values transferred")
	}
	values.Set("review_hash", prep.ReviewHash())
	if rec := postApp(mux, "/jobs/1/answers/transfer", values, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	// A pending answer edit invalidates only that field. The other remains usable.
	edited := fields[0]
	if rec := postApp(mux, "/jobs/1/answers/save", url.Values{"question_key": {questionKey(job.URL, "ashby", edited)}, "revision": {"1"}, "answer": {"A revised answer requiring review"}, "destination": {job.URL}}, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code)
	}
	a.browsers.tryAnswers(context.Background(), 1)
	if len(f.transferred) != 1 || f.transferred[0].Field.ID != fields[1].ID {
		t.Fatal("stale value reached browser or local failure blocked another answer")
	}
	state := a.browsers.state(1)
	if !state.Active || state.AnswerStatus != "complete" || len(state.AnswerOutcomes) != 2 {
		t.Fatalf("lost partial result: %+v", state)
	}
	// A new authorization can continue with the changed answer in the same session.
	prep, _ = a.prepareQuestions(job, CandidateProfile{})
	values.Set("review_hash", prep.ReviewHash())
	if rec := postApp(mux, "/jobs/1/answers/transfer", values, "", ""); rec.Code != 409 {
		t.Fatal("accepted a scope containing completed answers")
	}
	prep.Questions = pendingAnswers(prep.ReadyAnswers(), a.browsers.state(1).AnswerAttempted, false)
	values.Set("review_hash", prep.ReviewHash())
	if rec := postApp(mux, "/jobs/1/answers/transfer", values, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	a.browsers.tryAnswers(context.Background(), 1)
	if len(f.transferred) != 2 {
		t.Fatalf("continuation replayed a completed answer: %d", len(f.transferred))
	}
	if rec := postApp(mux, "/jobs/1/answers/transfer", values, "", ""); rec.Code != 409 {
		t.Fatal("unchanged approval replayed")
	}
}
