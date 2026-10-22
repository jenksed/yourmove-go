package app

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubmissionConfirmationUsesReviewedResume(t *testing.T) {
	newReviewedJob := func(t *testing.T) (*App, http.Handler, Resume, Resume) {
		t.Helper()
		a, _ := testWorkspace(t)
		if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Sample"}); err != nil {
			t.Fatal(err)
		}
		resumeA, err := a.importResume("resume-a.pdf", strings.NewReader("%PDF-1.4\nresume A"))
		if err != nil {
			t.Fatal(err)
		}
		resumeB, err := a.importResume("resume-b.pdf", strings.NewReader("%PDF-1.4\nresume B"))
		if err != nil {
			t.Fatal(err)
		}
		if err := a.assignResume(1, resumeA.ID); err != nil {
			t.Fatal(err)
		}
		return a, newMux(a), resumeA, resumeB
	}
	review := func(t *testing.T, mux http.Handler) string {
		t.Helper()
		page := httptest.NewRecorder()
		mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
		if page.Code != http.StatusOK {
			t.Fatalf("review page=%d %s", page.Code, page.Body.String())
		}
		return submissionTokenFromPage(t, page.Body.String())
	}
	confirm := func(mux http.Handler, token string) *httptest.ResponseRecorder {
		return postApp(mux, "/jobs/1/submit", url.Values{
			"confirm": {"yes"}, "submission_token": {token},
		}, "", "")
	}

	t.Run("same resume after navigation", func(t *testing.T) {
		a, mux, resumeA, _ := newReviewedJob(t)
		token := review(t, mux)
		for _, path := range []string{"/applications", "/jobs/1"} {
			page := httptest.NewRecorder()
			mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
			if page.Code != http.StatusOK {
				t.Fatalf("navigation to %s: %d", path, page.Code)
			}
		}
		if response := confirm(mux, token); response.Code != http.StatusSeeOther {
			t.Fatalf("unchanged review confirmation=%d %s", response.Code, response.Body.String())
		}
		history, err := a.jobSubmissions(1)
		if err != nil || len(history) != 1 || history[0].ResumeID != resumeA.ID || history[0].ResumeSHA256 != resumeA.SHA256 {
			t.Fatalf("confirmed resume history=%+v err=%v", history, err)
		}
	})

	t.Run("selected resume switched", func(t *testing.T) {
		a, mux, _, resumeB := newReviewedJob(t)
		token := review(t, mux)
		if err := a.assignResume(1, resumeB.ID); err != nil {
			t.Fatal(err)
		}
		response := confirm(mux, token)
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "Review the application and resume again") {
			t.Fatalf("stale selection confirmation=%d %s", response.Code, response.Body.String())
		}
		if history, err := a.jobSubmissions(1); err != nil || len(history) != 0 {
			t.Fatalf("stale selection was recorded: %+v err=%v", history, err)
		}
	})

	for _, mutate := range []struct {
		name string
		fn   func(*testing.T, *App, Resume)
	}{
		{name: "resume modified", fn: func(t *testing.T, a *App, resume Resume) {
			if err := os.WriteFile(filepath.Join(a.resumeDir, resume.StoredName), []byte("%PDF-1.4\nchanged resume A"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "resume file deleted", fn: func(t *testing.T, a *App, resume Resume) {
			if err := os.Remove(filepath.Join(a.resumeDir, resume.StoredName)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			a, mux, resumeA, _ := newReviewedJob(t)
			token := review(t, mux)
			mutate.fn(t, a, resumeA)
			response := confirm(mux, token)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "Review the application and resume again") {
				t.Fatalf("stale file confirmation=%d %s", response.Code, response.Body.String())
			}
			if history, err := a.jobSubmissions(1); err != nil || len(history) != 0 {
				t.Fatalf("stale file was recorded: %+v err=%v", history, err)
			}
		})
	}
}

func TestSubmissionTokenIdempotenceAndMissingJob(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/job", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	token := reviewedSubmissionToken(t, a, 1)
	if err := a.recordSubmission(1, "first", token); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "duplicate", token); err != nil {
		t.Fatal(err)
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 || history[0].Notes != "first" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	other, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(999, "", other); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing job: %v", err)
	}
	if err := a.recordSubmission(1, "", "bad-token"); err == nil {
		t.Fatal("accepted malformed token")
	}
}
