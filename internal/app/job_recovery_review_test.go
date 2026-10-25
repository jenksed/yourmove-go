package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

// This qualifies the manual continuation through real handlers. It does not
// qualify native browser launch, rich clipboard input, or employer acceptance.
func TestRecoveryIndependentManualContinuation(t *testing.T) {
	a, dir := testWorkspace(t)
	mux := newMux(a)
	post := func(path string, form url.Values) *httptest.ResponseRecorder {
		t.Helper()
		return postApp(mux, path, form, "http://localhost", "")
	}
	const posting = "https://careers.example.test/jobs/public-role?private_query=omit-me"
	w := post("/jobs", url.Values{"job_url": {posting}})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/jobs/1#overview" {
		t.Fatalf("unsupported intake = %d %q: %s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	const description = "Responsibilities\n• Help applicants\n\nQualifications\nClear communication\n<script>private-source-sample</script>"
	w = post("/jobs/1/edit", url.Values{"title": {"Corrected role"}, "company": {"Synthetic employer"}, "description": {description}, "stage": {"review"}, "priority": {"normal"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("manual recovery = %d: %s", w.Code, w.Body.String())
	}
	resume, err := a.importResume("Synthetic.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Synthetic applicant experience"))))
	if err != nil {
		t.Fatal(err)
	}
	w = post("/jobs/1/resume", url.Values{"resume_id": {strconv.FormatInt(resume.ID, 10)}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("resume selection = %d: %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Open application") || !strings.Contains(w.Body.String(), "Record completed application") || strings.Contains(w.Body.String(), "<script>private-source-sample</script>") {
		t.Fatalf("manual Prepare continuation unavailable or unsafe: %d", w.Code)
	}
	opened := ""
	a.openPosting = func(destination string) error { opened = destination; return nil }
	w = post("/jobs/1/open", nil)
	if w.Code != http.StatusSeeOther || opened != posting {
		t.Fatalf("explicit external open = %d %q", w.Code, opened)
	}
	// Unknown providers retain no managed browser or transfer authority.
	w = post("/jobs/1/browser/start", nil)
	if w.Code != http.StatusUnprocessableEntity || a.browsers.state(1).Active {
		t.Fatalf("unknown posting acquired managed authority: %d", w.Code)
	}
	w = post("/jobs/1/refresh", nil)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("unsupported refresh = %d", w.Code)
	}
	w = post("/jobs/1/diagnostics", url.Values{"reviewed": {"yes"}})
	if w.Code != http.StatusOK {
		t.Fatalf("reviewed diagnostics = %d", w.Code)
	}
	for _, private := range []string{"omit-me", "public-role", "Corrected role", "Synthetic employer", "private-source-sample", resume.SHA256, resume.Name} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatalf("default diagnostics disclosed %q", private)
		}
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a = newTestApp(t, db, filepath.Join(dir, "resumes"))
	mux = newMux(a)
	job, err := a.jobByID(1)
	if err != nil || job.Description != description || job.Title != "Corrected role" || !job.DescriptionManual || job.SelectedResumeID != resume.ID {
		t.Fatalf("restart lost recovered content: %+v %v", job, err)
	}
	w = post("/jobs/1/submit", url.Values{"submission_token": {reviewedSubmissionToken(t, a, 1)}, "confirm": {"yes"}})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/jobs/1#history" {
		t.Fatalf("manual completion = %d %q", w.Code, w.Header().Get("Location"))
	}
	job, err = a.jobByID(1)
	if err != nil || job.SubmissionCount != 1 || job.Stage != StageApplied {
		t.Fatalf("manual completion not recorded: %+v %v", job, err)
	}
	w = post("/jobs/1/next", nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/jobs" {
		t.Fatalf("continue queue = %d %q", w.Code, w.Header().Get("Location"))
	}
}
