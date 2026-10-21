package main

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttentionAllowsOnlyIndependentNewMaterial(t *testing.T) {
	q := preparedQuestion{Key: "new", Value: "saved answer", Editable: true}
	w := jobWorkspace{BrowserState: browserRunState{Active: true, Phase: "attention", ResumeStatus: "attention", LetterStatus: "attention"}, Questions: questionPreparation{Questions: []preparedQuestion{q}}}
	w.Letter.ID = 1
	got := summarizePreparation(w)
	if got.Action != "Continue application" || !got.IncludeAnswers || got.IncludeResume || got.IncludeLetter {
		t.Fatalf("independent answer blocked or uncertain document replay offered: %+v", got)
	}
	w.BrowserState.AnswerAttempted = []preparedQuestion{q}
	if got := summarizePreparation(w); got.Action != "" {
		t.Fatalf("attempted work offered again: %+v", got)
	}
	w.BrowserState.AnswerAttempted = nil
	w.BrowserState.Active = false
	w.BrowserState.Phase = "interrupted"
	if got := summarizePreparation(w); got.Action != "" || !got.Recovery {
		t.Fatalf("lost browser offered continuation: %+v", got)
	}
	w.BrowserState.Active = true
	w.BrowserState.Destination = "https://different.example.test"
	w.URL = testAshbyPosting
	if got := summarizePreparation(w); got.Action != "" || !got.Recovery {
		t.Fatalf("changed destination offered continuation: %+v", got)
	}
}

func TestInvalidResumeOffersRecoveryInsteadOfFill(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("resume.pdf", strings.NewReader("%PDF-1.4\nsynthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(a.resumeDir, resume.StoredName)); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := rec.Body.String()
	if rec.Code != 200 || strings.Contains(body, `action="/jobs/1/continue"`) || !strings.Contains(body, "file is missing or changed") || !strings.Contains(body, "Save resume") {
		t.Fatalf("missing file recovery: %d %s", rec.Code, body)
	}
}

func TestCompletionOffersNextActiveJobAndHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, job := range []Job{{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Current"}, {URL: "https://example.test/closed", Title: "Closed", Stage: StageClosed}, {URL: "https://example.test/next", Title: "Next active", Stage: StageReady, Priority: PriorityHigh}} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	rec := postApp(newMux(a), "/jobs/1/submit", url.Values{"confirm": {"yes"}, "submission_token": {reviewedSubmissionToken(t, a, 1)}}, "", "")
	if rec.Code != 303 || rec.Header().Get("Location") != "/jobs/1#history" {
		t.Fatalf("completion redirect: %d %s", rec.Code, rec.Header())
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	for _, part := range []string{"Application recorded as Applied.", `action="/jobs/1/next"`, "Next job → Next active", "Submission recorded"} {
		if !strings.Contains(body, part) {
			t.Fatalf("missing %q: %s", part, body)
		}
	}
}

func TestRecheckKeepsCurrentBrowserAndReturnsToPrepare(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	f := &fakeBrowser{url: testAshbyPosting + "/application"}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	t.Cleanup(a.browsers.closeAll)
	mux := newMux(a)
	if rec := postApp(mux, "/jobs/1/browser/start", url.Values{}, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code)
	}
	rec := postApp(mux, "/jobs/1/browser/observe", url.Values{"return_to": {"prepare"}}, "", "")
	if rec.Code != 303 || rec.Header().Get("Location") != "/jobs/1#prepare" {
		t.Fatalf("recheck stranded user: %d %v", rec.Code, rec.Header())
	}
	if !a.browsers.has(1) || a.browsers.sessions[1] != f || len(a.browsers.handoffs) != 0 || len(a.browsers.answers) != 0 || len(a.browsers.letters) != 0 {
		t.Fatal("recheck replaced browser or armed transfer")
	}
	denied := postApp(mux, "/jobs/1/browser/restart", url.Values{"return_to": {"prepare"}}, "", "")
	if denied.Code != 400 {
		t.Fatal("restart omitted browser-work-loss confirmation")
	}
}

func TestNextJobClosesOnlyConfirmedCompletedBrowser(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, job := range []Job{{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}, {URL: "https://example.test/next", Title: "Next", Stage: StageReady}} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	f := &lostTabBrowser{}
	a.browsers.sessions = map[int64]readOnlySession{1: f}
	mux := newDesktopMux(a)
	denied := postApp(mux, "/jobs/1/next", url.Values{}, "wails://localhost", "1")
	if denied.Code != 409 || f.closedCount() != 0 {
		t.Fatal("next closed an unsubmitted application")
	}
	if err := a.recordSubmission(1, "", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	if f.closedCount() != 0 {
		t.Fatal("recording closed employer browser")
	}
	next := postApp(mux, "/jobs/1/next", url.Values{}, "wails://localhost", "1")
	if next.Code != 200 || !strings.Contains(next.Body.String(), `window.location.replace("/jobs/2#prepare")`) || f.closedCount() != 1 || a.browsers.has(1) {
		t.Fatalf("next did not retire completed session: %d %s", next.Code, next.Body.String())
	}
	// Retrying an old Next form must not close the next job's browser.
	other := &lostTabBrowser{}
	a.browsers.sessions[2] = other
	again := postApp(mux, "/jobs/1/next", url.Values{}, "wails://localhost", "1")
	if again.Code != 200 || other.closedCount() != 0 {
		t.Fatal("old Next action disrupted another application")
	}
	t.Cleanup(a.browsers.closeAll)
}

func TestAnotherSessionHasDirectReturnInsteadOfFailingFill(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, job := range []Job{{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Application in progress"}, {URL: strings.Replace(testAshbyPosting, "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 1), Host: "jobs.ashbyhq.com", Title: "Another job"}} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	resume, err := a.importResume("resume.pdf", strings.NewReader("%PDF-1.4\nsynthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(2, resume.ID); err != nil {
		t.Fatal(err)
	}
	f := &lostTabBrowser{}
	a.browsers.sessions = map[int64]readOnlySession{1: f}
	t.Cleanup(a.browsers.closeAll)
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/2", nil))
	body := page.Body.String()
	if strings.Contains(body, `action="/jobs/2/continue"`) || !strings.Contains(body, `href="/jobs/1#prepare"`) || !strings.Contains(body, "Another application browser is open") || f.closedCount() != 0 {
		t.Fatalf("live owner not preserved and exposed: %s", body)
	}
}
