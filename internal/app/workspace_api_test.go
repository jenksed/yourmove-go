package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

func reviewCount(t *testing.T, a *App) int {
	t.Helper()
	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submission_reviews`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWorkspaceReadIsTokenFree(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/1", Host: "example.com", Title: "Role"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nbytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	a.browsers.mu.Lock()
	a.browsers.states = map[int64]browserRunState{}
	a.browsers.states[1] = browserRunState{
		Phase: "attention", Message: "Chromium runtime failed at /private/tmp/synthetic-browser-profile",
	}
	a.browsers.mu.Unlock()
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("precondition: submission_reviews=%d", got)
	}
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/workspace", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("workspace status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		for _, leaked := range []string{"submission_token", `"token"`, "sha256", "stored_name", "resumeDir", ".pdf\n", "Chromium runtime", "synthetic-browser-profile", "browserDetail"} {
			if strings.Contains(body, leaked) {
				t.Fatalf("workspace leaks %q: %s", leaked, body)
			}
		}
		var snapshot struct {
			Job              workspaceJobJSON `json:"job"`
			SelectedResumeID int64            `json:"selectedResumeId"`
			Readiness        map[string]any   `json:"readiness"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Job.ID != 1 || snapshot.SelectedResumeID != resume.ID || snapshot.Job.SelectedResumeID != resume.ID {
			t.Fatalf("wrong job/selection: %+v", snapshot)
		}
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("workspace reads created %d submission reviews", got)
	}
	// The token mechanism still works through the explicit review path, so the
	// eventual submission workflow is preserved.
	token := reviewedSubmissionToken(t, a, 1)
	if got := reviewCount(t, a); got != 1 {
		t.Fatalf("explicit review did not record: %d", got)
	}
	if err := a.recordSubmission(1, "applied personally", token); err != nil {
		t.Fatalf("submission with reviewed token failed: %v", err)
	}
}

func TestWorkspaceDetailsValidationAndIdentity(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/2", Host: "example.com", Title: "Original"}); err != nil {
		t.Fatal(err)
	}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/details", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}
	if rec := post(`{"title":"x","stage":"bogus","priority":"normal"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad stage accepted: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(`{"title":"x","stage":"applied","priority":"normal"}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "confirmed submission") {
		t.Fatalf("applied without submission not refused: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(`{"title":"` + strings.Repeat("t", 201) + `","stage":"review","priority":"normal"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("overlong title accepted: %d %s", rec.Code, rec.Body.String())
	}
	stored, err := a.jobByID(1)
	if err != nil || stored.Title != "Original" {
		t.Fatalf("rejected edits changed job: %+v err=%v", stored, err)
	}
	if rec := post(`{"title":"Edited","company":"Acme","description":"Notes","salary_text":"$100k","stage":"ready","priority":"high"}`); rec.Code != http.StatusOK {
		t.Fatalf("valid edit refused: %d %s", rec.Code, rec.Body.String())
	}
	updated, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Edited" || updated.Company != "Acme" || updated.Stage != "ready" || updated.Priority != "high" {
		t.Fatalf("edits not persisted: %+v", updated)
	}
	if updated.URL != "https://example.com/role/2" || updated.Host != "example.com" {
		t.Fatalf("source identity changed: %+v", updated)
	}
	persisted, err := a.jobByID(1)
	if err != nil || persisted.TitleManual != true || persisted.CompanyManual != true {
		t.Fatalf("manual precedence not recorded: %+v err=%v", persisted, err)
	}
}

func TestWorkspaceRefreshDisclaimerAndManualPrecedence(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/3", Host: "example.com", Title: "Draft title"}); err != nil {
		t.Fatal(err)
	}
	// Make the title manual by changing it, so a metadata refresh must preserve it.
	if err := a.updateJob(Job{ID: 1, Title: "Manual title", Stage: "review", Priority: "normal"}); err != nil {
		t.Fatal(err)
	}
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		return platforms.Metadata{Title: "Upstream title", Company: "Upstream Co", Description: "Upstream desc", SalaryText: "$1", SalaryOrigin: "employer"}, "retrieved", "ok"
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/refresh", nil)
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Job        workspaceJobJSON `json:"job"`
		Disclaimer string           `json:"disclaimer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Job.Title != "Manual title" {
		t.Fatalf("manual title overwritten: %+v", result.Job)
	}
	if result.Job.Company != "Upstream Co" {
		t.Fatalf("non-manual company not refreshed: %+v", result.Job)
	}
	if !strings.Contains(result.Disclaimer, "not verify whether the employer is still accepting applications") {
		t.Fatalf("disclaimer missing: %q", result.Disclaimer)
	}
	if result.Job.SourceStatus != "retrieved" {
		t.Fatalf("source status not stored: %+v", result.Job)
	}
}

func TestServeJobWorkspaceRouting(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/4", Host: "example.com", Title: "Routed"}); err != nil {
		t.Fatal(err)
	}
	a.resources.frontendIndex = []byte("<html>workspace</html>")
	react := httptest.NewRecorder()
	newMux(a).ServeHTTP(react, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
	if react.Code != http.StatusOK || react.Body.String() != "<html>workspace</html>" {
		t.Fatalf("React workspace not served: %d %s", react.Code, react.Body.String())
	}
	legacy := httptest.NewRecorder()
	newMux(a).ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1?legacy=1", nil))
	if legacy.Code != http.StatusOK || strings.Contains(legacy.Body.String(), "<html>workspace</html>") || !strings.Contains(legacy.Body.String(), "Routed") {
		t.Fatalf("legacy page not preserved: %d %.200s", legacy.Code, legacy.Body.String())
	}
	missing := httptest.NewRecorder()
	newMux(a).ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/999/workspace", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown job not 404: %d %s", missing.Code, missing.Body.String())
	}
	var _ = sql.ErrNoRows
}

func TestWorkspaceNextActionPrecedence(t *testing.T) {
	baseJob := Job{ID: 1, URL: "https://example.com/role", Stage: StageReview, SelectedResumeID: 7}
	validReadiness := applicationReadiness{QuestionsAvailable: true, NeedsAnswerCount: 0, PreparedCount: 2}
	quietBrowser := browserRunState{}
	quietPrep := HandoffPresentation{}
	cases := []struct {
		name      string
		job       Job
		state     browserRunState
		valid     bool
		readiness applicationReadiness
		prep      HandoffPresentation
		activity  string
		label     string
	}{
		{
			name:  "recorded history outranks closed stage",
			job:   func() Job { j := baseJob; j.Stage = StageClosed; j.SubmissionCount = 2; return j }(),
			state: quietBrowser, valid: true, readiness: validReadiness, prep: quietPrep,
			activity: "applied", label: "View submission history",
		},
		{
			name:  "applied without history",
			job:   func() Job { j := baseJob; j.Stage = StageApplied; return j }(),
			state: quietBrowser, valid: true, readiness: validReadiness, prep: quietPrep,
			activity: "applied", label: "Review application status",
		},
		{
			name:  "closed without history",
			job:   func() Job { j := baseJob; j.Stage = StageClosed; return j }(),
			state: quietBrowser, valid: true, readiness: validReadiness, prep: quietPrep,
			activity: "job", label: "Review opportunity",
		},
		{
			name:  "active browser before preparation",
			job:   baseJob,
			state: browserRunState{Active: true}, valid: false,
			readiness: applicationReadiness{QuestionsAvailable: true, NeedsAnswerCount: 3},
			prep:      quietPrep,
			activity:  "apply", label: "Review application status",
		},
		{
			name:  "recovery before preparation",
			job:   baseJob,
			state: quietBrowser, valid: false,
			readiness: applicationReadiness{QuestionsAvailable: true, NeedsAnswerCount: 3},
			prep:      HandoffPresentation{Recovery: true},
			activity:  "apply", label: "Review application status",
		},
		{
			name:  "opening browser before preparation",
			job:   baseJob,
			state: browserRunState{Phase: "opening"}, valid: false,
			readiness: applicationReadiness{},
			prep:      quietPrep,
			activity:  "apply", label: "Review application status",
		},
		{
			name:  "interrupted browser before preparation",
			job:   baseJob,
			state: browserRunState{Phase: "interrupted"}, valid: true,
			readiness: validReadiness,
			prep:      quietPrep,
			activity:  "apply", label: "Review application status",
		},
		{
			name:  "missing resume",
			job:   func() Job { j := baseJob; j.SelectedResumeID = 0; return j }(),
			state: quietBrowser, valid: false, readiness: validReadiness, prep: quietPrep,
			activity: "documents", label: "Choose a résumé",
		},
		{
			name:  "invalid selected resume",
			job:   baseJob,
			state: quietBrowser, valid: false, readiness: validReadiness, prep: quietPrep,
			activity: "documents", label: "Review selected résumé",
		},
		{
			name:  "unanswered questions",
			job:   baseJob,
			state: quietBrowser, valid: true,
			readiness: applicationReadiness{QuestionsAvailable: true, NeedsAnswerCount: 2, PreparedCount: 1},
			prep:      quietPrep,
			activity:  "questions", label: "Review available questions",
		},
		{
			name:  "unknown inventory falls through to options",
			job:   baseJob,
			state: quietBrowser, valid: true,
			readiness: applicationReadiness{QuestionsAvailable: false},
			prep:      quietPrep,
			activity:  "apply", label: "Review application options",
		},
		{
			name:  "partial without unanswered falls through",
			job:   baseJob,
			state: quietBrowser, valid: true,
			readiness: applicationReadiness{QuestionsAvailable: true, NeedsAnswerCount: 0, PreparedCount: 1, QuestionPartial: true},
			prep:      quietPrep,
			activity:  "apply", label: "Review application options",
		},
		{
			name:  "complete zero falls through",
			job:   baseJob,
			state: quietBrowser, valid: true,
			readiness: applicationReadiness{QuestionsAvailable: true, NeedsAnswerCount: 0, PreparedCount: 0},
			prep:      quietPrep,
			activity:  "apply", label: "Review application options",
		},
	}
	for _, tc := range cases {
		if got := chooseWorkspaceNextAction(tc.job, tc.state, tc.valid, tc.readiness, tc.prep); got.Activity != tc.activity || got.Label != tc.label {
			t.Fatalf("%s: got %+v want %s/%s", tc.name, got, tc.activity, tc.label)
		}
	}
}

func TestWorkspaceSnapshotCarriesNextAction(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/5", Host: "example.com", Title: "Next"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/workspace", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("workspace status=%d body=%s", rec.Code, rec.Body.String())
	}
	var snapshot struct {
		NextAction workspaceNextActionJSON `json:"nextAction"`
		Job        workspaceJobJSON        `json:"job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.NextAction.Activity != "documents" || snapshot.NextAction.Label == "" {
		t.Fatalf("missing next action for resume-less job: %+v", snapshot.NextAction)
	}
}

func TestWorkspaceFamilyCountSuggestionAndManualOverride(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveFamily(RoleFamily{Name: "Support", TitleTerms: "support engineer"}); err != nil {
		t.Fatal(err)
	}
	families, err := a.allFamilies()
	if err != nil || len(families) != 1 {
		t.Fatalf("families=%+v err=%v", families, err)
	}
	first, err := a.importResume("Support_A.pdf", strings.NewReader("%PDF-1.4\nA"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.importResume("Support_B.pdf", strings.NewReader("%PDF-1.4\nB"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := a.importResume("General.pdf", strings.NewReader("%PDF-1.4\nC"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResumeFamily(first.ID, families[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResumeFamily(second.ID, families[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/support/1", Host: "example.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	get := func() struct {
		FamilyCount      int                   `json:"familyCount"`
		SelectedResumeID int64                 `json:"selectedResumeId"`
		Suggestion       workspaceMatchJSON    `json:"suggestion"`
		Options          []workspaceResumeJSON `json:"resumeOptions"`
	} {
		t.Helper()
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/workspace", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("workspace status=%d body=%s", rec.Code, rec.Body.String())
		}
		var snapshot struct {
			FamilyCount      int                   `json:"familyCount"`
			SelectedResumeID int64                 `json:"selectedResumeId"`
			Suggestion       workspaceMatchJSON    `json:"suggestion"`
			Options          []workspaceResumeJSON `json:"resumeOptions"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	snapshot := get()
	if snapshot.FamilyCount != 1 {
		t.Fatalf("familyCount=%d", snapshot.FamilyCount)
	}
	if snapshot.Suggestion.Status != "matched" || snapshot.Suggestion.ResumeID != second.ID || snapshot.Suggestion.FamilyName != "Support" {
		t.Fatalf("suggestion=%+v want matched primary %d", snapshot.Suggestion, second.ID)
	}
	seen := map[int64]workspaceResumeJSON{}
	for _, option := range snapshot.Options {
		seen[option.ID] = option
	}
	if seen[first.ID].RoleFamilyName != "Support" || seen[first.ID].IsPrimary {
		t.Fatalf("first option wrong: %+v", seen[first.ID])
	}
	if !seen[second.ID].IsPrimary {
		t.Fatalf("primary flag missing: %+v", seen[second.ID])
	}
	if seen[plain.ID].RoleFamilyName != "" {
		t.Fatalf("unassigned resume shows a family: %+v", seen[plain.ID])
	}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/api/jobs/1/resume", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}
	// Manual selection overrides the suggestion through the same endpoint.
	if rec := post(`{"resume_id":` + itoa(first.ID) + `}`); rec.Code != http.StatusOK {
		t.Fatalf("override refused: %d %s", rec.Code, rec.Body.String())
	}
	if snapshot := get(); snapshot.SelectedResumeID != first.ID {
		t.Fatalf("override not saved: %+v", snapshot.SelectedResumeID)
	}
	// A failed assignment preserves the previous selection.
	if rec := post(`{"resume_id":9999}`); rec.Code == http.StatusOK {
		t.Fatalf("unknown resume accepted: %d", rec.Code)
	}
	if snapshot := get(); snapshot.SelectedResumeID != first.ID {
		t.Fatalf("failed assignment changed selection: %+v", snapshot.SelectedResumeID)
	}
}
