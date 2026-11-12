package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Completed, closed, or submission-recorded jobs must expose no additional
// Candidate Profile transfer scope and must refuse an answers-only POST
// before any managed browser launch.
func TestCompletedJobExposesNoProfileFactScope(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Role"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", LinkedIn: "https://www.linkedin.com/in/fixture", Approved: true}); err != nil {
		t.Fatal(err)
	}
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:why", Label: "Why this team?", Kind: "textarea"},
		{ID: "browser:linkedin", Label: "LinkedIn URL", Kind: "url"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	whyKey := questionKey(job.URL, "ashby", fields[0])
	if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,?,?,?,?,'Because the on-call work matters',1)`, job.URL, "ashby", whyKey, fields[0].Label); err != nil {
		t.Fatal(err)
	}
	active, err := a.buildContinuation(1)
	if err != nil {
		t.Fatal(err)
	}
	activeScope := active["answerScopeInternal"].([]preparedQuestion)
	hasProfile := false
	for _, q := range activeScope {
		if additionalProfileAttribute(q.Field) != "" && !q.IsEarly {
			hasProfile = true
			break
		}
	}
	if !hasProfile || len(activeScope) == 0 {
		t.Fatalf("precondition: active scope missing eligible profile fact: %+v", activeScope)
	}
	activeHash := reactAnswerScopeHash(activeScope)

	if _, err := a.db.Exec(`UPDATE jobs SET stage=? WHERE id=1`, StageApplied); err != nil {
		t.Fatal(err)
	}
	completed, err := a.buildContinuation(1)
	if err != nil {
		t.Fatal(err)
	}
	if pending := completed["pendingAnswers"].([]continuationAnswerJSON); len(pending) != 0 {
		t.Fatalf("completed scope not empty: %+v", pending)
	}
	if prep := completed["preparation"].(map[string]any); prep["includeAnswers"] != false {
		t.Fatalf("completed scope offers answers: %v", prep)
	}

	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		return nil, errors.New("unexpected launch for completed job")
	}
	body, _ := json.Marshal(map[string]any{"destination": testAshbyPosting, "include_answers": true, "review_hash": activeHash, "profile_revision": strconv.FormatInt(profile.Revision, 10)})
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("completed transfer accepted: %d %s", rec.Code, rec.Body.String())
	}
	if launches != 0 || a.browsers.state(1).Active {
		t.Fatalf("completed transfer launched a browser: launches=%d active=%v", launches, a.browsers.state(1).Active)
	}
}

// Additional Candidate Profile transfers require a nonempty matching profile
// revision. Omitted or stale revisions authorize nothing and launch nothing;
// ordinary saved-answer transfers keep their existing behavior.
func TestProfileFactTransferRequiresMatchingRevision(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Role"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", LinkedIn: "https://www.linkedin.com/in/fixture", Approved: true}); err != nil {
		t.Fatal(err)
	}
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:why", Label: "Why this team?", Kind: "textarea"},
		{ID: "browser:linkedin", Label: "LinkedIn URL", Kind: "url"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	whyKey := questionKey(job.URL, "ashby", fields[0])
	if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,?,?,?,?,'Because the on-call work matters',1)`, job.URL, "ashby", whyKey, fields[0].Label); err != nil {
		t.Fatal(err)
	}
	payload, err := a.buildContinuation(1)
	if err != nil {
		t.Fatal(err)
	}
	scope := payload["answerScopeInternal"].([]preparedQuestion)
	hasProfile := false
	for _, q := range scope {
		if additionalProfileAttribute(q.Field) != "" && !q.IsEarly {
			hasProfile = true
			break
		}
	}
	if !hasProfile {
		t.Fatalf("precondition: scope missing profile fact: %+v", scope)
	}
	hash := reactAnswerScopeHash(scope)

	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		return nil, errors.New("unexpected launch for unauthorized profile revision")
	}
	post := func(revision string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"destination": testAshbyPosting, "include_answers": true, "review_hash": hash, "profile_revision": revision})
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}
	if rec := post(""); rec.Code != http.StatusConflict || launches != 0 || a.browsers.state(1).Active {
		t.Fatalf("omitted revision authorized: %d %s launches=%d", rec.Code, rec.Body.String(), launches)
	}
	if rec := post("999999"); rec.Code != http.StatusConflict || launches != 0 || a.browsers.state(1).Active {
		t.Fatalf("stale revision authorized: %d %s launches=%d", rec.Code, rec.Body.String(), launches)
	}

	// Resolver-level: omitted revision cannot authorize profile facts, while
	// an ordinary saved answer keeps its existing approval.
	transfer := a.reactAnswerTransfer(job, scope, "")
	var ordinary, profileFact preparedQuestion
	for _, q := range scope {
		if additionalProfileAttribute(q.Field) != "" && !q.IsEarly {
			profileFact = q
		} else {
			ordinary = q
		}
	}
	if _, status := transfer.resolve[profileFact.Key](context.Background(), nil, profileFact); status != "manual" {
		t.Fatalf("omitted revision authorized profile fact: %q", status)
	}
	stale := a.reactAnswerTransfer(job, scope, "999999")
	if _, status := stale.resolve[profileFact.Key](context.Background(), nil, profileFact); status != "manual" {
		t.Fatalf("stale revision authorized profile fact: %q", status)
	}
	if _, status := transfer.resolve[ordinary.Key](context.Background(), nil, ordinary); status != "ready" {
		t.Fatalf("ordinary answer behavior changed for blank revision: %q", status)
	}
	_ = profile
}
