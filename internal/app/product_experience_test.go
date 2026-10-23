package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductExperienceShellActivitiesAndSavedBrowserWriting(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	saveLibrarySourceAnswer(t, a, job, key, "Synthetic saved incident response, preserved independently of the disconnected browser.", 0)
	mux := newMux(a)
	get := func(path string) string {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		return w.Body.String()
	}
	for _, path := range []string{"/jobs", "/jobs/1", "/resumes", "/families", "/candidate", "/answer-library", "/career-evidence", "/ai", "/applications", "/utilities", "/requirements", "/enrichment"} {
		page := get(path)
		for _, want := range []string{`id="workspace-navigation"`, `aria-controls="workspace-navigation"`, `data-navigation-close`, `Skip to content`, `>Submissions</a>`} {
			if !strings.Contains(page, want) {
				t.Errorf("%s missing shared shell contract %s", path, want)
			}
		}
	}
	page := get("/jobs/1")
	for _, name := range []string{"overview", "prepare", "answers", "cover-letter", "history"} {
		if strings.Count(page, `data-workspace-panel="`+name+`"`) != 1 {
			t.Errorf("one authoritative %s panel required", name)
		}
	}
	if strings.Count(page, `action="/jobs/1/answers/save"`) != 1 {
		t.Error("saved question must have exactly one authoritative save form")
	}
	for _, want := range []string{"Saved browser-question workbench", `id="transfer-review"`, `data-contact-exclusion`, `Complete candidate profile`, `name="submission_token"`, `name="confirm" value="yes" required`, `data-writing-activity-key="letter"`} {
		if !strings.Contains(page, want) {
			t.Errorf("missing workspace contract %s", want)
		}
	}
	if !strings.Contains(page, "Questions not discovered yet") {
		t.Error("saved disconnected browser must not become current question readiness")
	}
	profile := get("/candidate")
	if strings.Contains(profile, "VERIFIED INFORMATION") || !strings.Contains(profile, "saving without it removes approval") {
		t.Error("profile approval must be applicant-owned and fresh")
	}
	ai := get("/ai")
	for _, want := range []string{`id="local-ai-heading"`, `id="external-ai-heading"`, "Manual writing and preparation remain available without AI"} {
		if !strings.Contains(ai, want) {
			t.Errorf("missing optional AI contract %s", want)
		}
	}
	if err := a.recordSubmission(1, "Synthetic confirmation", reviewedSubmissionToken(t, a, 1)); err != nil {
		t.Fatal(err)
	}
	saved, err := a.jobByID(1)
	if err != nil || saved.LastSubmittedAt == "" {
		t.Fatalf("submission date missing: %+v %v", saved, err)
	}
	if page = get("/applications"); !strings.Contains(page, saved.LastSubmittedAt) || !strings.Contains(page, "Submission recorded locally") {
		t.Error("submission list must show local record date")
	}
}

func TestDocumentDisplaySize(t *testing.T) {
	for _, c := range []struct {
		bytes int64
		want  string
	}{{687, "687 bytes"}, {2048, "2.0 KiB"}, {1572864, "1.5 MiB"}} {
		r := Resume{Size: c.bytes}
		if r.DisplaySize() != c.want {
			t.Errorf("%d: %s", c.bytes, r.DisplaySize())
		}
	}
}
