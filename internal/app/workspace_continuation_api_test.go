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

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func continuationScope(t *testing.T, a *App, jobID int64) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/continuation", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("continuation status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func seedContinuationJob(t *testing.T, a *App) {
	t.Helper()
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Role"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:why", Label: "Why this team?", Kind: "textarea"},
		{ID: "browser:site", Label: "Website", Kind: "text"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,?,?,?,?,'Approved for `+f.ID+`',1)`, testAshbyPosting, "ashby", questionKey(testAshbyPosting, "ashby", f), f.Label); err != nil {
			t.Fatal(err)
		}
	}
}

func TestContinuationScopeIsTokenFreeAndComplete(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	resume, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nbytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	f := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := a.browsers.start(context.Background(), 1, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("precondition: submission_reviews=%d", got)
	}
	payload := continuationScope(t, a, 1)
	prep := payload["preparation"].(map[string]any)
	if prep["includeAnswers"] != true {
		t.Fatalf("answers not offered: %v", prep)
	}
	pending := payload["pendingAnswers"].([]any)
	if len(pending) != 2 {
		t.Fatalf("pending answers: %v", pending)
	}
	if _, ok := payload["browser"]; !ok {
		t.Fatal("browser state missing")
	}
	lowered := ""
	if raw, merr := json.Marshal(payload); merr == nil {
		lowered = strings.ToLower(string(raw))
	}
	for _, leaked := range []string{"submission_token", "stored_name", "sha256\", \"valid", ".pdf\n"} {
		if strings.Contains(lowered, leaked) {
			t.Fatalf("continuation leaks %q", leaked)
		}
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("continuation reads created %d submission reviews", got)
	}
}

func TestContinueRequiresSelectionAndDestination(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}
	if rec := post(`{"destination":"` + testAshbyPosting + `"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty scope accepted: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(`{"destination":"https://example.com/other","include_answers":true,"review_hash":"x"}`); rec.Code != http.StatusConflict {
		t.Fatalf("wrong destination accepted: %d %s", rec.Code, rec.Body.String())
	}
	// No browser session: arming must fail without transferring anything.
	if rec := post(`{"destination":"` + testAshbyPosting + `","include_answers":true,"review_hash":"bogus"}`); rec.Code != http.StatusConflict {
		t.Fatalf("continue status=%d %s", rec.Code, rec.Body.String())

	}
}

func TestContinueAnswersTransferWithFakeBrowser(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	f := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}, controls: []browser.Control{{ID: "why", Label: "Why this team?", Kind: "textarea"}, {ID: "site", Label: "Website", Kind: "text"}}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := a.browsers.start(context.Background(), 1, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"destination":"` + testAshbyPosting + `","include_answers":true,"review_hash":"` + reactAnswerScopeHash(prep.ReadyAnswers()) + `"}`
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("continue status=%d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []continueItemResult `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || len(result.Items) != 1 || !result.Items[0].OK {
		t.Fatalf("answers not armed: %s", rec.Body.String())
	}
	a.browsers.tryAnswers(context.Background(), 1)
	state := a.browsers.state(1)
	if state.AnswerStatus != "complete" || len(state.AnswerOutcomes) != 2 {
		t.Fatalf("lost outcomes: %+v", state)
	}
	if len(f.transferred) != 2 {
		t.Fatalf("transferred=%d", len(f.transferred))
	}
	// Transfer does not mark the job Applied.
	if stored, _ := a.jobByID(1); stored.Stage == StageApplied || stored.SubmissionCount > 0 {
		t.Fatalf("transfer recorded a submission: %+v", stored)
	}
}

func TestSubmissionReviewSubmitAndHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	resume, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nbytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	mint := func() (int, map[string]any) {
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/submission-review", nil))
		var payload map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &payload)
		return rec.Code, payload
	}
	code, payload := mint()
	if code != http.StatusOK || payload["token"] == "" {
		t.Fatalf("mint: %d %v", code, payload)
	}
	token := payload["token"].(string)
	submit := func(token, note string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/submit", strings.NewReader(`{"confirm":"yes","notes":`+jsonString(note)+`,"submission_token":"`+token+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}
	// A malformed token takes the legacy uncertain-recording path, not a record.
	if rec := submit("bogus", "x"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("bad token: %d %s", rec.Code, rec.Body.String())
	}
	note := "Applied personally, confirmation SYN-1"
	if rec := submit(token, note); rec.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
	}
	// Replaying the exact submission is idempotent, not a second record.
	if rec := submit(token, note); rec.Code != http.StatusOK {
		t.Fatalf("replay: %d %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=1`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("submissions=%d err=%v", count, err)
	}
	job, _ := a.jobByID(1)
	if job.Stage != StageApplied || job.SubmissionCount != 1 {
		t.Fatalf("not applied: %+v", job)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/submissions", nil))
	var history struct {
		Count       int `json:"count"`
		Submissions []struct {
			ResumeName string `json:"resumeName"`
			Notes      string `json:"notes"`
		} `json:"submissions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil || history.Count != 1 || history.Submissions[0].Notes != note || history.Submissions[0].ResumeName != "role.pdf" {
		t.Fatalf("history: %s", rec.Body.String())
	}
	// Changing the selection stales a minted review.
	if _, err := a.importResume("other.pdf", strings.NewReader("%PDF-1.4\nother")); err != nil {
		t.Fatal(err)
	}
	code, payload = mint()
	if code != http.StatusOK {
		t.Fatalf("second mint: %d", code)
	}
	other, _ := a.resumeByID(2)
	if err := a.assignResume(1, other.ID); err != nil {
		t.Fatal(err)
	}
	if rec := submit(payload["token"].(string), "after change"); rec.Code != http.StatusConflict {
		t.Fatalf("stale review accepted: %d %s", rec.Code, rec.Body.String())
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=1`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stale submit wrote: %d", count)
	}
	// The stale submit consumed its review; ordinary reads still mint nothing.
	continuationScope(t, a, 1)
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("unexpected reviews=%d", got)
	}
}

func TestContinuationNextWithoutSubmission(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	next := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/next", nil))
		return rec
	}
	if rec := next(); rec.Code != http.StatusOK {
		t.Fatalf("next before applied: %d %s", rec.Code, rec.Body.String())
	}
	job, err := a.jobByID(1)
	if err != nil || job.Stage == StageApplied || job.SubmissionCount != 0 {
		t.Fatalf("navigation changed submission state: %+v %v", job, err)
	}
	f := &fakeBrowser{url: testAshbyPosting + "/application"}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := a.browsers.start(context.Background(), 1, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	if rec := next(); rec.Code != http.StatusConflict || f.timesClosed() != 0 {
		t.Fatalf("active browser not protected: %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/next?close_browser=yes", nil))
	if rec.Code != http.StatusOK || f.timesClosed() != 1 {
		t.Fatalf("confirmed close: %d, closes %d", rec.Code, f.timesClosed())
	}
	token := reviewedSubmissionToken(t, a, 1)
	if err := a.recordSubmission(1, "done", token); err != nil {
		t.Fatal(err)
	}
	if rec := next(); rec.Code != http.StatusOK {
		t.Fatalf("next: %d %s", rec.Code, rec.Body.String())
	}
}

func TestContinueAuthorizesEachMaterialIndependently(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Role", Description: "Resolve production incidents"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Production incident experience"))))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Technical support experience"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:why", Label: "Why this team?", Kind: "textarea"}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,?,?,?,?,'Because the on-call work matters',1)`, testAshbyPosting, "ashby", questionKey(testAshbyPosting, "ashby", field), field.Label); err != nil {
		t.Fatal(err)
	}
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		return "Dear Hiring Manager,\n\nI resolve production incidents with care and clear customer communication.\n\nSincerely,\nCasey Fixture", nil
	}
	f := &continuationBrowser{fakeLetterBrowser: &fakeLetterBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := a.browsers.start(context.Background(), 1, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	_, err = a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	savedRole, _ := a.resumeByID(role.ID)
	draft, err := func() (CoverLetter, error) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/letters", strings.NewReader(`{"provider":"ollama","model":"","letter_resume_id":0,"career_evidence":[],"confirm_local":"yes"}`))
		req.Header.Set("Content-Type", "application/json")
		newMux(a).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("letter generate: %d %s", rec.Code, rec.Body.String())
		}
		return a.latestCoverLetter(1)
	}()
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"destination":"` + testAshbyPosting + `","include_resume":true,"resume_id":"` + itoa(savedRole.ID) + `","resume_hash":"` + savedRole.SHA256 + `","transfer_contacts":true,"profile_revision":"` + itoa(profile.Revision) + `","include_letter":true,"draft_id":"` + itoa(draft.ID) + `","body_hash":"` + draft.ContentHash() + `","mode":"file","include_answers":true,"review_hash":"` + continuationScope(t, a, 1)["preparation"].(map[string]any)["answerReviewHash"].(string) + `"}`
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("continue status=%d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []continueItemResult `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || len(result.Items) != 3 {
		t.Fatalf("items: %s %v", rec.Body.String(), err)
	}
	for _, item := range result.Items {
		if !item.OK {
			t.Fatalf("item %s not armed: %+v", item.Item, item)
		}
	}
	// Authorizing transfer never records a submission.
	if stored, _ := a.jobByID(1); stored.Stage == StageApplied || stored.SubmissionCount > 0 {
		t.Fatalf("authorization recorded a submission: %+v", stored)
	}
}

func TestContinuationBrowserActions(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	f := &fakeBrowser{url: testAshbyPosting + "/application"}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	defer a.browsers.closeAll()
	call := func(action string) map[string]any {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/browser", strings.NewReader(`{"action":"`+action+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", action, rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := call("start"); out["ok"] != true {
		t.Fatalf("start: %v", out)
	}
	if out := call("observe"); out["ok"] != true {
		t.Fatalf("observe: %v", out)
	}
	scope := continuationScope(t, a, 1)
	if scope["questionsAvailable"] != true {
		t.Fatalf("observed questions not current: %v", scope["questionsAvailable"])
	}
	if out := call("stop"); out["ok"] != true {
		t.Fatalf("stop: %v", out)
	}
	if a.browsers.has(1) {
		t.Fatal("session alive after stop")
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/browser", strings.NewReader(`{"action":"bogus"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("bogus action: %d", rec.Code)
	}
	_ = browser.AnswerOutcome{}
}

func TestContinuationReportsResumeLaunchFailure(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	resume, err := a.importResume("resume.pdf", strings.NewReader("%PDF-1.4\nsynthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		return nil, errors.New("synthetic launch failure")
	}
	payload, err := json.Marshal(continueInput{Destination: testAshbyPosting, IncludeResume: true, ResumeID: strconv.FormatInt(resume.ID, 10), ResumeHash: resume.SHA256, TransferContacts: true, ProfileRevision: "0"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	var out struct {
		Items []continueItemResult `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Items) != 1 || out.Items[0].OK || !strings.Contains(out.Items[0].Detail, "synthetic launch failure") {
		t.Fatalf("launch failure hidden: %d %s %v", rec.Code, rec.Body.String(), err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test"}); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || launches != 1 {
		t.Fatalf("stale profile allowed a new launch: %d launches=%d", rec.Code, launches)
	}
}

func TestContinuationContactsAfterResumeSelection(t *testing.T) {
	a, _ := testWorkspace(t)
	seedContinuationJob(t, a)
	resume, err := a.importResume("resume.pdf", strings.NewReader("%PDF-1.4\nsynthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test"}); err != nil {
		t.Fatal(err)
	}
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	fake := &contactFakeBrowser{}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fake, nil }
	if err := a.browsers.start(context.Background(), 1, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	if err := a.browsers.armResume(1, t.TempDir(), "/tmp/fixture.pdf", "fixture.pdf", nil, resumeHandoffInfo{resumeID: resume.ID, resumeHash: resume.SHA256}); err != nil {
		t.Fatal(err)
	}
	a.browsers.tryResume(context.Background(), 1)
	payload, err := json.Marshal(continueInput{Destination: testAshbyPosting, ResumeID: strconv.FormatInt(resume.ID, 10), ResumeHash: resume.SHA256, TransferContacts: true, ProfileRevision: strconv.FormatInt(profile.Revision, 10)})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	var out struct {
		Items []continueItemResult `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Items) != 1 || !out.Items[0].OK {
		t.Fatalf("contact-only action failed: %s %v", rec.Body.String(), err)
	}
	a.browsers.tryContacts(context.Background(), 1)
	if fake.calls != 1 || fake.contactCalls != 1 || fake.approved.Email != profile.Email {
		t.Fatalf("unexpected replay or contact values: uploads=%d contacts=%d", fake.calls, fake.contactCalls)
	}
}

func TestContinuationIncludesSavedEarlyAnswerCandidates(t *testing.T) {
	a, _ := testWorkspace(t)
	job, discovery := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, discovery); err != nil {
		t.Fatal(err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "")
	const value = "A saved answer\nwith exact formatting."
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", value), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if _, err := a.latestInspectionBySource(job.ID, "browser"); err == nil {
		t.Fatal("unexpected browser inspection")
	}
	payload, err := a.buildContinuation(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidates, ok := payload["earlyAnswerCandidates"].([]continuationEarlyAnswerCandidate)
	if !ok || len(candidates) != 1 {
		t.Fatalf("unexpected candidates: %#v", payload["earlyAnswerCandidates"])
	}
	got := candidates[0]
	if got.ID <= 0 || got.Destination != job.URL || got.DiscoverySource != q.DiscoverySource || got.DiscoveryFingerprint != q.DiscoveryFingerprint || got.SourceQuestionID != q.SourceQuestionID || got.Revision != 1 || got.Value != value || got.Field.ID != q.Field.ID || got.Field.Label != q.Field.Label {
		t.Fatalf("candidate mismatch: %+v", got)
	}
	if a.browsers.state(job.ID).Active || a.browsers.answers[job.ID] != nil {
		t.Fatal("read-only continuation opened a browser or armed transfer")
	}
	preparation := payload["preparation"].(map[string]any)
	pending := payload["pendingAnswers"].([]continuationAnswerJSON)
	if preparation["includeAnswers"] != true || len(pending) != 1 || pending[0].Value != value {
		t.Fatalf("saved early answer is not selectable for the original Fill: includeAnswers=%v pending=%+v", preparation["includeAnswers"], pending)
	}
}

func TestContinuationRejectsChangedEarlyAnswerBeforeBrowserLaunch(t *testing.T) {
	a, _ := testWorkspace(t)
	job, discovery := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, discovery); err != nil {
		t.Fatal(err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "")
	mux := newMux(a)
	if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", "Original"), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code)
	}
	payload, err := a.buildContinuation(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	hash := payload["preparation"].(map[string]any)["answerReviewHash"].(string)
	if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "1", "Edited"), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code)
	}
	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		return nil, errors.New("unexpected launch")
	}
	body, _ := json.Marshal(map[string]any{"destination": job.URL, "include_resume": true, "include_answers": true, "review_hash": hash})
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/continue", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || launches != 0 {
		t.Fatalf("stale scope status=%d launches=%d body=%s", rec.Code, launches, rec.Body.String())
	}
}
