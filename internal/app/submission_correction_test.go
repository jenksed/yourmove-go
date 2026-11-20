package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// An accidentally recorded submission must be correctable without losing
// protected applicant material: the latest local record goes, older
// history stays, and editing resumes only when no confirmed history
// remains. Answers, letters, résumé associations, job details, and
// unrelated records are never part of the correction.
func TestSubmissionCorrectionRestoresEditing(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/correct-me", Host: "example.com", Title: "Support Engineer", Company: "Example", Description: "Help customers."}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/other", Host: "example.com", Title: "Other role"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("correction-resume.pdf", strings.NewReader("%PDF-1.4\ncorrection resume"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	created := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", `{"question":"Why do you want to join this team?"}`)
	if created.Code != http.StatusOK {
		t.Fatalf("create manual question: %d %s", created.Code, created.Body.String())
	}
	var createdPayload struct {
		Question map[string]any `json:"question"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdPayload); err != nil {
		t.Fatal(err)
	}
	key, _ := createdPayload.Question["key"].(string)
	if key == "" {
		t.Fatalf("manual question without key: %s", created.Body.String())
	}
	saved := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+key, `{"answer":"I enjoy resolving difficult technical problems for customers.","revision":1}`)
	if saved.Code != http.StatusOK {
		t.Fatalf("save manual answer: %d %s", saved.Code, saved.Body.String())
	}
	letterBody := "Dear Hiring Team, I am interested in this support role and would welcome the opportunity to contribute with careful follow-up."
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":`+jsonString(letterBody)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("save manual letter: %d %s", rec.Code, rec.Body.String())
	}
	if err := a.recordSubmission(1, "recorded by accident", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(2, "unrelated history", reviewedSubmissionToken(t, a, 2)); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil || job.Stage != StageApplied || job.SubmissionCount != 1 {
		t.Fatalf("recorded job=%+v err=%v", job, err)
	}
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+key, `{"answer":"Blocked edit.","revision":2}`); rec.Code != http.StatusConflict {
		t.Fatalf("editing not locked while recorded: %d %s", rec.Code, rec.Body.String())
	}

	corrected := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/unsubmit", `{"confirm":"yes"}`)
	if corrected.Code != http.StatusOK {
		t.Fatalf("correct submission: %d %s", corrected.Code, corrected.Body.String())
	}
	var correction struct {
		Corrected bool  `json:"corrected"`
		Remaining int64 `json:"remaining"`
	}
	if err := json.Unmarshal(corrected.Body.Bytes(), &correction); err != nil || !correction.Corrected || correction.Remaining != 0 {
		t.Fatalf("correction payload: %s err=%v", corrected.Body.String(), err)
	}

	job, err = a.jobByID(1)
	if err != nil || job.Stage != StageReview || job.SubmissionCount != 0 {
		t.Fatalf("corrected job=%+v err=%v", job, err)
	}
	if job.SelectedResumeID != resume.ID || job.Title != "Support Engineer" || job.Company != "Example" || job.Description != "Help customers." {
		t.Fatalf("correction disturbed job details or resume association: %+v", job)
	}
	if history, err := a.jobSubmissions(1); err != nil || len(history) != 0 {
		t.Fatalf("corrected history=%+v err=%v", history, err)
	}
	// Saved writing survives the correction with its identity.
	questions := postManualJSON(t, a, http.MethodGet, "/api/jobs/1/questions", `{}`)
	if questions.Code != http.StatusOK || !strings.Contains(questions.Body.String(), "I enjoy resolving difficult technical problems for customers.") {
		t.Fatalf("answer lost: %d %s", questions.Code, questions.Body.String())
	}
	letters := postWritingJSON(t, a, http.MethodGet, "/api/jobs/1/letters", "")
	if letters.Code != http.StatusOK || !strings.Contains(letters.Body.String(), "careful follow-up") {
		t.Fatalf("letter lost: %d %s", letters.Code, letters.Body.String())
	}
	// Editing resumes on the corrected opportunity.
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+key, `{"answer":"I enjoy resolving difficult technical problems for customers, updated.","revision":2}`); rec.Code != http.StatusOK {
		t.Fatalf("editing still locked after correction: %d %s", rec.Code, rec.Body.String())
	}
	// The unrelated opportunity keeps its confirmed history.
	if history, err := a.jobSubmissions(2); err != nil || len(history) != 1 || history[0].Notes != "unrelated history" {
		t.Fatalf("unrelated history=%+v err=%v", history, err)
	}
	if other, err := a.jobByID(2); err != nil || other.Stage != StageApplied || other.SubmissionCount != 1 {
		t.Fatalf("unrelated job=%+v err=%v", other, err)
	}
}

func TestSubmissionCorrectionKeepsOlderHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/retry", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "first attempt", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	secondToken, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmissionReview(1, secondToken, 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "accidental duplicate", secondToken); err != nil {
		t.Fatal(err)
	}
	corrected := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/unsubmit", `{"confirm":"yes"}`)
	if corrected.Code != http.StatusOK {
		t.Fatalf("correct submission: %d %s", corrected.Code, corrected.Body.String())
	}
	var correction struct {
		Remaining int64 `json:"remaining"`
	}
	if err := json.Unmarshal(corrected.Body.Bytes(), &correction); err != nil || correction.Remaining != 1 {
		t.Fatalf("correction payload: %s err=%v", corrected.Body.String(), err)
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 || history[0].Notes != "first attempt" {
		t.Fatalf("older history=%+v err=%v", history, err)
	}
	job, err := a.jobByID(1)
	if err != nil || job.Stage != StageApplied || job.SubmissionCount != 1 {
		t.Fatalf("job with history=%+v err=%v", job, err)
	}
}

func TestSubmissionCorrectionRefusals(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/nothing", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/unsubmit", `{"confirm":"yes"}`); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "nothing-to-correct") {
		t.Fatalf("correct without history: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/unsubmit", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("correct without confirmation: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/999/unsubmit", `{"confirm":"yes"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("correct missing job: %d %s", rec.Code, rec.Body.String())
	}
	if job, err := a.jobByID(1); err != nil || job.Stage != StageReview || job.SubmissionCount != 0 {
		t.Fatalf("refused correction changed state: %+v err=%v", job, err)
	}
}
