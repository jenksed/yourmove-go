package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

type fakeAnswerBrowser struct {
	*fakeBrowser
	transferred []browser.AnswerValue
	controls    []browser.Control
}

type uncertainAnswerBrowser struct {
	*fakeBrowser
	calls int
}

func (f *fakeAnswerBrowser) Observe(context.Context) (browser.Observation, error) {
	return browser.Observation{URL: f.url, Forms: 1, Controls: f.controls}, nil
}

func (f *fakeAnswerBrowser) FillAnswer(_ context.Context, v browser.AnswerValue) (browser.AnswerOutcome, error) {
	f.transferred = append(f.transferred, v)
	return browser.AnswerOutcome{Label: v.Field.Label, Status: "filled", Detail: "Observed"}, nil
}

func (f *uncertainAnswerBrowser) FillAnswer(_ context.Context, v browser.AnswerValue) (browser.AnswerOutcome, error) {
	f.calls++
	return browser.AnswerOutcome{Label: v.Field.Label, Status: "uncertain", Detail: "Review the selected location in Chromium."}, nil
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
	f := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}, controls: []browser.Control{{ID: "why", Label: "Why this team?", Kind: "textarea"}, {ID: "site", Label: "Website", Kind: "text"}}}
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

func TestProfileLocationCannotEnterAnswerTransfer(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	profile := CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", City: "Detroit", Region: "Michigan", Country: "United States", Approved: true}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:location", Label: "Current location", Kind: "combobox"}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := a.prepareQuestions(job, profile)
	if err != nil || len(prepared.ReadyAnswers()) != 0 {
		t.Fatalf("location not prepared: %+v %v", prepared.Questions, err)
	}
	filler := &uncertainAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}
	a.browsers.sessions = map[int64]readOnlySession{1: filler}
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	a.browsers.states = map[int64]browserRunState{1: {Destination: job.URL}}
	values := url.Values{"review_hash": {prepared.ReviewHash()}, "destination": {job.URL}}
	mux := newMux(a)
	if rec := postApp(mux, "/jobs/1/answers/transfer", values, "", ""); rec.Code != 409 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	a.browsers.tryAnswers(context.Background(), 1)
	if filler.calls != 0 {
		t.Fatal("profile dispatched as answer")
	}
}

func TestSavedAnswerWaitsForLaterControl(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:why", Label: "Why this team?", Kind: "textarea", Group: "rendered form"}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	mux := newMux(a)
	key := questionKey(job.URL, "ashby", field)
	if rec := postApp(mux, "/jobs/1/answers/save", url.Values{"question_key": {key}, "revision": {"0"}, "answer": {"Saved value"}, "destination": {job.URL}}, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	f := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: job.URL + "/application"}}
	a.browsers.sessions = map[int64]readOnlySession{1: f}
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	a.browsers.states = map[int64]browserRunState{1: {Destination: job.URL}}
	defer a.browsers.closeAll()
	prep, _ := a.prepareQuestions(job, CandidateProfile{})
	if rec := postApp(mux, "/jobs/1/answers/transfer", url.Values{"review_hash": {prep.ReviewHash()}, "destination": {job.URL}}, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	a.browsers.tryAnswers(context.Background(), 1)
	if len(f.transferred) != 0 || len(a.browsers.state(1).AnswerAttempted) != 0 || a.browsers.state(1).AnswerStatus != "waiting" {
		t.Fatal("absent answer did not remain pending")
	}
	// Persist a different page to prove validity does not depend on latest inspection.
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application/step2", Fields: []schema.Field{{ID: "browser:other", Label: "Other", Kind: "text"}}}); err != nil {
		t.Fatal(err)
	}
	f.controls = []browser.Control{{ID: "why", Label: "Why this team?", Kind: "textarea"}}
	a.browsers.tryAnswers(context.Background(), 1)
	a.browsers.tryAnswers(context.Background(), 1)
	if len(f.transferred) != 1 || f.transferred[0].Value != "Saved value" || len(a.browsers.state(1).AnswerAttempted) != 1 {
		t.Fatalf("unexpected transfers: %+v", f.transferred)
	}
}

type uncertainEarlyBrowser struct {
	*fakeAnswerBrowser
	calls int
}

func (f *uncertainEarlyBrowser) FillAnswer(_ context.Context, v browser.AnswerValue) (browser.AnswerOutcome, error) {
	f.calls++
	return browser.AnswerOutcome{Label: v.Field.Label, Status: "uncertain"}, nil
}

// The original authorization stays pending until its exact question appears.
// A saved live decision (including a clear) or an edited early draft revokes it.
func TestReactEarlyAnswerDelayedPrecedence(t *testing.T) {
	for _, scenario := range []string{"fill once", "live clear", "early edit", "uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			a, _ := testWorkspace(t)
			job, discovery := earlyGreenhouseFixture(t, a)
			if _, err := a.saveInspection(job.ID, discovery); err != nil {
				t.Fatal(err)
			}
			early := earlyQuestionByID(t, a, job, "application:story", "")
			mux := newMux(a)
			if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, early, "0", "Original answer"), "", ""); rec.Code != 303 {
				t.Fatal(rec.Code, rec.Body.String())
			}
			payload, err := a.buildContinuation(job.ID)
			if err != nil {
				t.Fatal(err)
			}
			scope := payload["answerScopeInternal"].([]preparedQuestion)
			if len(scope) != 1 {
				t.Fatalf("scope: %+v", scope)
			}
			fake := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: job.URL}}
			var session readOnlySession = fake
			var uncertain *uncertainEarlyBrowser
			if scenario == "uncertain" {
				uncertain = &uncertainEarlyBrowser{fakeAnswerBrowser: fake}
				session = uncertain
			}
			a.browsers.sessions = map[int64]readOnlySession{job.ID: session}
			a.browsers.cancels = map[int64]context.CancelFunc{job.ID: func() {}}
			a.browsers.states = map[int64]browserRunState{job.ID: {Destination: job.URL}}
			defer a.browsers.closeAll()
			if err := a.browsers.armAnswers(job.ID, job.URL, a.reactAnswerTransfer(job, scope, "")); err != nil {
				t.Fatal(err)
			}
			a.browsers.tryAnswers(context.Background(), job.ID)
			if a.browsers.state(job.ID).AnswerStatus != "waiting" {
				t.Fatal("missing field did not wait")
			}
			field := schema.Field{ID: "browser:story", Label: early.Field.Label, Kind: "textarea", Required: early.Field.Required, Group: "rendered form"}
			if scenario == "live clear" {
				if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(?,?,?,?,?,'',1)`, job.ID, job.URL, "greenhouse", questionKey(job.URL, "greenhouse", field), field.Label); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "early edit" {
				if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, early, "1", "Changed answer"), "", ""); rec.Code != 303 {
					t.Fatal(rec.Code, rec.Body.String())
				}
			}
			fake.controls = []browser.Control{{ID: "story", Label: field.Label, Kind: field.Kind, Required: field.Required}}
			a.browsers.tryAnswers(context.Background(), job.ID)
			a.browsers.tryAnswers(context.Background(), job.ID)
			if scenario == "fill once" && (len(fake.transferred) != 1 || fake.transferred[0].Value != "Original answer") {
				t.Fatalf("transfers: %+v state=%+v", fake.transferred, a.browsers.state(job.ID))
			}
			if (scenario == "live clear" || scenario == "early edit") && len(fake.transferred) != 0 {
				t.Fatalf("unexpected transfer: %+v", fake.transferred)
			}
			if scenario == "uncertain" {
				if uncertain.calls != 1 {
					t.Fatalf("uncertain write calls=%d", uncertain.calls)
				}
				if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, early, "1", "Changed after uncertain write"), "", ""); rec.Code != 303 {
					t.Fatal(rec.Code, rec.Body.String())
				}
				payload, err := a.buildContinuation(job.ID)
				if err != nil {
					t.Fatal(err)
				}
				if pending := payload["pendingAnswers"].([]continuationAnswerJSON); len(pending) != 0 {
					t.Fatalf("uncertain dispatch became eligible: %+v", pending)
				}
				a.browsers.tryAnswers(context.Background(), job.ID)
				if uncertain.calls != 1 {
					t.Fatalf("uncertain write replayed %d times", uncertain.calls)
				}
			}
		})
	}
}

func TestAnswerAuthorizationAddsWhileOtherAnswerWaits(t *testing.T) {
	h := &sessionHost{sessions: map[int64]readOnlySession{1: &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}}, cancels: map[int64]context.CancelFunc{1: func() {}}, states: map[int64]browserRunState{1: {Destination: testAshbyPosting}}}
	defer h.closeAll()
	current := func(preparedQuestion) bool { return true }
	first := preparedQuestion{Key: "first", Value: "one", Editable: true}
	second := preparedQuestion{Key: "second", Value: "two", Editable: true}
	if err := h.armAnswers(1, testAshbyPosting, &applicationAnswerTransfer{hash: "one", status: "waiting", approved: []preparedQuestion{first}, current: current}); err != nil {
		t.Fatal(err)
	}
	if err := h.armAnswers(1, testAshbyPosting, &applicationAnswerTransfer{hash: "two", status: "waiting", approved: []preparedQuestion{second}, current: current}); err != nil {
		t.Fatal(err)
	}
	if len(h.answers[1].approved) != 2 {
		t.Fatalf("pending authorizations lost: %+v", h.answers[1].approved)
	}
}

func TestChangedEarlyAnswerCanBeReauthorizedAfterRejectedWithoutDispatch(t *testing.T) {
	a, _ := testWorkspace(t)
	job, discovery := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, discovery); err != nil {
		t.Fatal(err)
	}
	early := earlyQuestionByID(t, a, job, "application:story", "")
	mux := newMux(a)
	save := func(rev, value string) {
		t.Helper()
		if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, early, rev, value), "", ""); rec.Code != 303 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	save("0", "Answer A")
	fake := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: job.URL}}
	a.browsers.sessions = map[int64]readOnlySession{job.ID: fake}
	a.browsers.cancels = map[int64]context.CancelFunc{job.ID: func() {}}
	a.browsers.states = map[int64]browserRunState{job.ID: {Destination: job.URL}}
	defer a.browsers.closeAll()
	authorize := func(payload map[string]any) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"destination": job.URL, "include_answers": true, "review_hash": payload["preparation"].(map[string]any)["answerReviewHash"]})
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var result struct {
			Items []continueItemResult `json:"items"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || len(result.Items) != 1 || !result.Items[0].OK {
			t.Fatalf("authorization: %d %s", rec.Code, rec.Body.String())
		}
	}
	payload, err := a.buildContinuation(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	authorize(payload)
	a.browsers.tryAnswers(context.Background(), job.ID)
	if a.browsers.state(job.ID).AnswerStatus != "waiting" {
		t.Fatal("A did not wait")
	}
	save("1", "Answer B")
	payload, err = a.buildContinuation(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending := payload["pendingAnswers"].([]continuationAnswerJSON); len(pending) != 1 || pending[0].Value != "Answer B" {
		t.Fatalf("B missing while A waits: %+v", pending)
	}
	a.browsers.tryAnswers(context.Background(), job.ID)
	if len(fake.transferred) != 0 || len(a.browsers.state(job.ID).AnswerAttempted) != 0 {
		t.Fatal("A was treated as dispatched")
	}
	payload, err = a.buildContinuation(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	pending := payload["pendingAnswers"].([]continuationAnswerJSON)
	if len(pending) != 1 || pending[0].Value != "Answer B" {
		t.Fatalf("B missing from Fill scope: %+v", pending)
	}
	authorize(payload)
	fake.controls = []browser.Control{{ID: "story", Label: early.Field.Label, Kind: "textarea", Required: early.Field.Required}}
	a.browsers.tryAnswers(context.Background(), job.ID)
	a.browsers.tryAnswers(context.Background(), job.ID)
	if len(fake.transferred) != 1 || fake.transferred[0].Value != "Answer B" {
		t.Fatalf("writes: %+v", fake.transferred)
	}
}

func TestQueuedChangedEarlyAnswerDoesNotReplayPriorDispatch(t *testing.T) {
	f := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}
	first := preparedQuestion{Key: "early-key", IsEarly: true, Value: "A", Revision: 1}
	second := preparedQuestion{Key: "early-key", IsEarly: true, Value: "B", Revision: 2}
	h := &sessionHost{
		sessions: map[int64]readOnlySession{1: f},
		cancels:  map[int64]context.CancelFunc{1: func() {}},
		states:   map[int64]browserRunState{1: {Destination: testAshbyPosting}},
		answers: map[int64]*applicationAnswerTransfer{1: {
			status: "waiting", approved: []preparedQuestion{second}, attempted: []preparedQuestion{first},
			current: func(preparedQuestion) bool { return true },
		}},
	}
	defer h.closeAll()
	h.tryAnswers(context.Background(), 1)
	if len(f.transferred) != 0 || len(h.state(1).AnswerAttempted) != 1 {
		t.Fatalf("queued answer replayed a prior dispatch: writes=%+v state=%+v", f.transferred, h.state(1))
	}
}
