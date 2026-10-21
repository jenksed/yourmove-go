package main

import (
	"bytes"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestDeleteJobsAtomicPreservesResumeLibraryAndDuplicatesCanBeReimported(t *testing.T) {
	a, _ := testWorkspace(t)
	resume, err := a.importResume("cv.pdf", bytes.NewReader([]byte("%PDF-1.4\ntest")))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := a.saveJob(Job{URL: "https://example.com/job" + string(rune('a'+i)), Host: "example.com"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(3, "real application", reviewedSubmissionToken(t, a, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteJobs([]int64{1, 3}, false); !errors.Is(err, ErrJobHasHistory) {
		t.Fatalf("expected atomic rejection, got %v", err)
	}
	if _, err := a.jobByID(1); err != nil {
		t.Fatalf("deletion was not atomic: %v", err)
	}
	if n, err := a.deleteJobs([]int64{1, 2}, false); err != nil || n != 2 {
		t.Fatalf("delete test jobs: %d %v", n, err)
	}
	if _, err := a.jobByID(1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("still present: %v", err)
	}
	if _, err := a.resumeByID(resume.ID); err != nil {
		t.Fatalf("resume removed by job deletion: %v", err)
	}
	if _, added, err := a.storeJobIfNew(Job{URL: "https://example.com/joba", Host: "example.com"}); err != nil || !added {
		t.Fatalf("cannot reuse deleted URL: %v %v", added, err)
	}
	if _, err := a.deleteJobs([]int64{3}, false); !errors.Is(err, ErrJobHasHistory) {
		t.Fatalf("history guard: %v", err)
	}
	if _, err := a.deleteJobs([]int64{3}, true); err != nil {
		t.Fatal(err)
	}
	history, err := a.jobSubmissions(3)
	if err != nil || len(history) != 0 {
		t.Fatalf("history remains: %v %v", history, err)
	}
}

func TestDeleteFormsExplicitConfirmationAndWailsNavigation(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/test", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/other", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	bad := postApp(newMux(a), "/jobs/1/delete", url.Values{"confirm_phrase": {"DELETE"}}, "", "")
	if bad.Code != 400 {
		t.Fatalf("weak confirmation: %d", bad.Code)
	}
	good := postApp(newDesktopMux(a), "/jobs/1/delete", url.Values{"confirm": {"yes"}, "confirm_phrase": {"DELETE"}}, "wails://localhost", "1")
	if good.Code != http.StatusOK || !strings.Contains(good.Body.String(), `window.location.replace("/jobs")`) {
		t.Fatalf("Wails navigation %d %s", good.Code, good.Body.String())
	}
	if err := a.recordSubmission(2, "real", reviewedSubmissionToken(t, a, 2)); err != nil {
		t.Fatal(err)
	}
	weak := postApp(newMux(a), "/jobs/2/delete", url.Values{"confirm": {"yes"}, "confirm_phrase": {"DELETE"}}, "", "")
	if weak.Code != 409 {
		t.Fatalf("deleted history without consent: %d", weak.Code)
	}
	strong := postApp(newMux(a), "/jobs/2/delete", url.Values{"confirm": {"yes"}, "confirm_history": {"yes"}, "confirm_phrase": {"DELETE APPLICATION HISTORY"}}, "", "")
	if strong.Code != 303 {
		t.Fatalf("historical deletion %d", strong.Code)
	}
}

func TestBulkDeleteRejectsHistoryAndCrossSiteRequests(t *testing.T) {
	a, _ := testWorkspace(t)
	for i := 0; i < 2; i++ {
		if err := a.saveJob(Job{URL: "https://example.com/job" + string(rune('a'+i)), Host: "example.com"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.recordSubmission(2, "actual", reviewedSubmissionToken(t, a, 2)); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"job_ids": {"1", "2"}, "confirm": {"yes"}, "confirm_phrase": {"DELETE"}}
	bad := postApp(newMux(a), "/jobs/delete", form, "", "")
	if bad.Code != 409 {
		t.Fatalf("should refuse historical rows: %d", bad.Code)
	}
	if _, err := a.jobByID(1); err != nil {
		t.Fatalf("atomic rollback: %v", err)
	}
	form["job_ids"] = []string{"1"}
	forged := postApp(newMux(a), "/jobs/delete", form, "https://untrusted.test", "")
	if forged.Code != 403 {
		t.Fatalf("origin guard: %d", forged.Code)
	}
	good := postApp(newMux(a), "/jobs/delete", form, "", "")
	if good.Code != 303 {
		t.Fatalf("bulk delete: %d", good.Code)
	}
	if _, err := a.jobByID(2); err != nil {
		t.Fatalf("historical application changed: %v", err)
	}
}
