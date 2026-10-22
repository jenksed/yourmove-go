package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverLetterStudioGeneratesSavesAndPinsSources(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Support Engineer", Company: "Example", Description: "Work on Linux incidents and MySQL queries"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Managed production incidents")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Linux and MySQL troubleshooting")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	a.coverLetterGenerate = func(_ context.Context, prompt string) (string, error) {
		for _, phrase := range []string{"Managed production incidents", "Linux and MySQL troubleshooting", "Work on Linux incidents", "Target 120–160 words", "180 words as the editorial ceiling", "Choose the strongest documented professional example", "never move achievements or responsibilities between employers", "Internal operations are not automatically external customer support", "Investigating a problem does not establish", "Preserve distinctions between technologies and databases", "not independently fact-checked records"} {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("missing prompt source %q", phrase)
			}
		}
		if !strings.HasSuffix(prompt, coverLetterInstructions) {
			t.Error("source evidence must be followed by the canonical writing instructions")
		}
		return "I have worked on Linux troubleshooting and production support. I would bring that experience to this role.", nil
	}
	mux := newMux(a)
	bad := postApp(mux, "/jobs/1/cover-letter/generate", url.Values{}, "", "")
	if bad.Code != 303 || bad.Header().Get("Location") != coverNoticeURL(1, "invalid") {
		t.Fatalf("no consent: %d %q", bad.Code, bad.Header().Get("Location"))
	}
	rec := postApp(newDesktopMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "wails://localhost", "1")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `window.location.replace("/jobs/1?cover=generated#prepare")`) {
		t.Fatalf("desktop generation redirect: %d %s", rec.Code, rec.Body.String())
	}
	draft, err := a.latestCoverLetter(1)
	if err != nil || draft.RoleResumeID != role.ID || draft.MasterResumeID != master.ID || draft.RoleSHA256 != role.SHA256 || draft.MasterSHA256 != master.SHA256 || len(draft.JobDescriptionSHA256) != 64 {
		t.Fatalf("draft %+v %v", draft, err)
	}
	update := postApp(mux, "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {"Edited letter grounded in verified experience and saved as a draft."}}, "", "")
	if update.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", update.Code, update.Body.String())
	}
	draft, err = a.latestCoverLetter(1)
	if err != nil || !strings.HasPrefix(draft.Body, "Edited letter") || draft.EditedAt == "" {
		t.Fatalf("saved %+v %v", draft, err)
	}
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1?cover=saved", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Edited letter grounded") || !strings.Contains(page.Body.String(), "Your edits were saved") {
		t.Fatalf("UI draft status %d", page.Code)
	}
	cross := postApp(mux, "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {"Attempted cross-origin update must not change persisted draft."}}, "https://evil.example", "")
	if cross.Code != 403 {
		t.Fatalf("cross origin save: %d", cross.Code)
	}
	if safeDesktopRedirectTarget("//evil.example/jobs/1?cover=saved#prepare") {
		t.Fatal("allowed foreign redirect")
	}
	// A new generation creates a second draft rather than erasing the edited first.
	rec = postApp(mux, "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "", "")
	if rec.Code != 303 {
		t.Fatalf("regenerate %d", rec.Code)
	}
	summaries, err := a.listCoverLetters(1)
	if err != nil || len(summaries) != 2 || summaries[0].ID == summaries[1].ID {
		t.Fatalf("draft versions %+v %v", summaries, err)
	}
	previous, err := a.coverLetterByID(1, draft.ID)
	if err != nil || !strings.HasPrefix(previous.Body, "Edited letter") {
		t.Fatalf("historical draft lost %+v %v", previous, err)
	}
	page = httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1?draft=1", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Edited letter grounded") {
		t.Fatal("cannot reopen earlier draft")
	}
	if err := a.setMasterResume(master.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.deleteResume(master.ID); !errors.Is(err, ErrResumeInUse) {
		t.Fatalf("deleted cover letter source: %v", err)
	}
}

func TestCoverLetterSourcesRequiredAndNoDraftOnFailure(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Example"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		calls++
		return "Unexpected model response should never be used here.", nil
	}
	w := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "", "")
	if w.Code != 303 || w.Header().Get("Location") != coverNoticeURL(1, "source-description") || calls != 0 {
		t.Fatalf("missing source validation %d %+v", w.Code, calls)
	}
	if _, err := a.latestCoverLetter(1); err == nil {
		t.Fatal("created draft without sources")
	}
}

func TestCoverLetterStudioCanChooseDifferentSourceThanApplication(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/job", Host: "example.com", Title: "Support", Description: "Troubleshoot production systems"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "General career background for support roles")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Specific Linux and MySQL experience")))
	if err != nil {
		t.Fatal(err)
	}
	selected, err := a.importResume("apply.pdf", strings.NewReader("%PDF-1.4\nmock PDF without readable text"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, selected.ID); err != nil {
		t.Fatal(err)
	}
	a.coverLetterGenerate = func(_ context.Context, prompt string) (string, error) {
		if !strings.Contains(prompt, "Specific Linux and MySQL experience") {
			t.Errorf("override role evidence not present")
		}
		return "I have supported Linux services and production environments. My experience is relevant to this position.", nil
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), `name="letter_resume_id"`) || !strings.Contains(page.Body.String(), "apply.pdf") {
		t.Fatalf("selection UI missing: %d", page.Code)
	}
	sent := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}, "letter_resume_id": {fmt.Sprint(role.ID)}, "model": {"test-model"}}, "", "")
	if sent.Code != 303 || sent.Header().Get("Location") != coverNoticeURL(1, "generated") {
		t.Fatalf("generation failed %d", sent.Code)
	}
	draft, err := a.latestCoverLetter(1)
	if err != nil || draft.RoleResumeID != role.ID || draft.Model != "test-model" {
		t.Fatalf("wrong evidence source %+v %v", draft, err)
	}
	job, err := a.jobByID(1)
	if err != nil || job.SelectedResumeID != selected.ID {
		t.Fatalf("application selection modified %+v %v", job, err)
	}
}

func TestCoverLetterWritingPolicyHasSingleEditableSource(t *testing.T) {
	if strings.TrimSpace(coverLetterInstructions) == "" {
		t.Fatal("embedded cover letter instructions are empty")
	}
	for _, phrase := range []string{
		"120–160 words",
		"Dear Hiring Team,",
		"No markdown",
		"never move achievements or responsibilities between employers",
	} {
		if !strings.Contains(coverLetterInstructions, phrase) {
			t.Errorf("canonical instructions missing %q", phrase)
		}
	}
}

func TestCoverLetterExternalRequiresSeparateConsent(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/support", Host: "example.com", Title: "Support", Description: "Linux troubleshooting"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Production incidents")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Linux systems")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	a.coverLetterGenerate = func(ctx context.Context, prompt string) (string, error) {
		return "I have resolved production issues and maintained Linux systems in customer-facing roles.", nil
	}
	legacy := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{"provider": {"openai"}, "model": {"gpt-4.1-mini"}, "confirm_local": {"yes"}}, "", "")
	if legacy.Header().Get("Location") != coverNoticeURL(1, "invalid") {
		t.Fatal("cloud request accepted local consent")
	}
	if _, err := a.latestCoverLetter(1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("draft stored without external consent")
	}
	approved := postApp(newDesktopMux(a), "/jobs/1/cover-letter/generate", url.Values{"provider": {"openai"}, "model": {"gpt-4.1-mini"}, "confirm_external": {"yes"}}, "wails://localhost", "1")
	if approved.Code != http.StatusOK || !strings.Contains(approved.Body.String(), `window.location.replace("/jobs/1?cover=generated#prepare")`) {
		t.Fatalf("approval redirect %d %s", approved.Code, approved.Body.String())
	}
	draft, err := a.latestCoverLetter(1)
	if err != nil || draft.Provider != "openai" || draft.Model != "gpt-4.1-mini" {
		t.Fatalf("draft attribution %+v %v", draft, err)
	}
}

func TestCoverProviderMigrationPreservesOldDrafts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE cover_letters (
        id INTEGER PRIMARY KEY, job_id INTEGER NOT NULL, body TEXT NOT NULL,
        model TEXT NOT NULL, master_resume_id INTEGER NOT NULL, master_sha256 TEXT NOT NULL,
        role_resume_id INTEGER NOT NULL, role_sha256 TEXT NOT NULL,
        job_description_sha256 TEXT NOT NULL, created_at TEXT NOT NULL,
        edited_at TEXT NOT NULL DEFAULT '' )`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`INSERT INTO cover_letters (id,job_id,body,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES (1,1,'Previous draft text','legacy-model',2,'master-hash',3,'role-hash','job-hash','2026-10-19')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	draft, err := (&App{db: db}).latestCoverLetter(1)
	if err != nil || draft.Provider != "ollama" || draft.Model != "legacy-model" || draft.Body != "Previous draft text" {
		t.Fatalf("migration lost legacy draft: %+v %v", draft, err)
	}
}

func TestCoverProviderNoticesStayInWails(t *testing.T) {
	for _, code := range []string{"key", "auth", "rate", "credits", "timeout", "output", "provider-unavailable", "offline", "model-missing", "cli", "login", "cli-failed"} {
		if !safeDesktopRedirectTarget(coverNoticeURL(2, code)) {
			t.Errorf("Wails would reject safe provider notice %q", code)
		}
	}
	if safeDesktopRedirectTarget("//evil.example/jobs/2?cover=auth#prepare") || safeDesktopRedirectTarget("/jobs/2?cover=unknown#prepare") {
		t.Fatal("unsafe or unknown provider redirect allowed")
	}
}

func TestAIConnectionHubNoCredentialsAndWailsRedirects(t *testing.T) {
	a, _ := testWorkspace(t)
	t.Setenv("OPENAI_API_KEY", "never-show-secret-in-html")
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/ai", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "ChatGPT via Codex") ||
		!strings.Contains(page.Body.String(), "Claude via Claude Code") ||
		strings.Contains(page.Body.String(), "never-show-secret-in-html") {
		t.Fatalf("connection hub response %d, secret leak=%v", page.Code, strings.Contains(page.Body.String(), "never-show-secret-in-html"))
	}
	if !safeDesktopRedirectTarget("/ai") {
		t.Fatal("safe AI hub redirect rejected")
	}
}

func TestWritingDeskShowsSubscriptionAndSeparateAPIChoices(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.org/post", Host: "example.org", Title: "Support"}); err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	for _, expected := range []string{`value="codex"`, `value="claude-cli"`, `value="ollama" selected`, `value="openai"`, `data-ai-generate`, `data-ai-draft`} {
		if expected == `data-ai-draft` {
			continue /* no draft until generation */
		}
		if !strings.Contains(page.Body.String(), expected) {
			t.Errorf("writing desk missing %q", expected)
		}
	}
}

func TestValidateGeneratedLetter(t *testing.T) {
	valid := "Dear Hiring Team,\n\nI investigated production failures and documented reproducible findings for engineering. I would bring that troubleshooting experience to your support team.\n\nSincerely,\nAlex"
	for name, body := range map[string]string{
		"candidate voice": "I need to understand each customer’s issue before recommending a next step. Let me know if you would like to discuss my support experience.",
		"plain":           valid, "exact whitespace": " \n" + valid + "\n\n", "long usable": strings.Repeat("I investigated production failures and documented findings. ", 30),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateGeneratedLetter(body); err != nil {
				t.Fatal(err)
			}
		})
	}
	for name, body := range map[string]string{
		"empty": " \n", "invalid utf8": valid + string([]byte{255}), "nul": valid + "\x00", "control": valid + "\x1b", "oversize": strings.Repeat("a", maxLetterBytes+1),
		"punctuation": strings.Repeat("!", 80), "two letters": valid + "\n\n" + valid,
		"inline greetings":        "Dear Hiring Team, my technical support work involved production debugging.\n\nDear Hiring Team, my account work involved technical escalations.",
		"two bodies and closings": "My support experience includes production debugging.\nSincerely,\nAlex\n\nMy account experience includes escalation management.\nBest regards,\nAlex",
		"reasoning":               "<think>I should select the strongest evidence.</think>\n" + valid,
		"analysis":                "Analysis: choose troubleshooting evidence.\n" + valid,
		"correction":              "Wait, I should revise this cover letter before returning it.\n" + valid,
		"alternatives":            "Letter 1:\n" + valid,
		"fenced":                  "```text\n" + valid + "\n```",
		"json envelope":           `{"letter":"I investigated production failures and documented findings for engineering."}`,
		"resume summary": `Based on the detailed work history you provided, here is a summary of your profile.

### Professional Summary
Senior software professional with extensive experience.

### Core Competencies
* Linux administration
* Client management`,
		"bulleted profile": "Dear Hiring Team,\n\nKey qualifications:\n- Linux troubleshooting\n- Customer escalations\n\nThank you for considering me.",
		"bare preamble":    "Based on your resume, your qualifications include troubleshooting production systems and managing support relationships across client portfolios.",
		"too many words":   strings.Repeat("I investigated production failures and documented findings. ", 60),
	} {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(validateGeneratedLetter(body), errCoverOutput) {
				t.Fatal("accepted malformed response")
			}
		})
	}
}

func TestGeneratedLetterRejectionPreservesDrafts(t *testing.T) {
	a, browser := scopeFirstWorkspace(t)
	original, err := a.coverLetterByID(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	for _, body := range []string{"", "Analysis: I should invent an example before writing this cover letter.", "Based on your resume, here is a profile summary.\n\n### Professional Summary\nExtensive Linux and customer support experience.\n\n* Managed hosting\n* Customer support", "Dear Hiring Team,\nFirst complete letter with professional evidence.\nSincerely,\nAlex\n\nDear Hiring Team,\nAnother complete letter with different professional evidence.\nSincerely,\nAlex"} {
		a.coverLetterGenerate = func(context.Context, string) (string, error) { return body, nil }
		r := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "", "")
		if r.Header().Get("Location") != coverNoticeURL(1, "output") {
			t.Fatal(r.Code, r.Header())
		}
		drafts, _ := a.listCoverLetters(1)
		after, _ := a.coverLetterByID(1, 1)
		currentJob, _ := a.jobByID(1)
		if len(drafts) != 2 || after != original || currentJob.SelectedResumeID != job.SelectedResumeID || browser.calls != 0 {
			t.Fatal("failure changed existing work")
		}
	}
	exact := " \n" + strings.Repeat("I investigated production failures and documented findings. ", 25) + "\n\n"
	a.coverLetterGenerate = func(context.Context, string) (string, error) { return exact, nil }
	r := postApp(newMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "", "")
	draft, err := a.latestCoverLetter(1)
	if r.Header().Get("Location") != coverNoticeURL(1, "generated") || err != nil || draft.Body != exact {
		t.Fatal("usable output changed", r.Header(), err)
	}
	edited := "\n  " + exact + "  \n"
	if err := a.saveCoverLetterBody(1, draft.ID, edited); err != nil {
		t.Fatal(err)
	}
	saved, _ := a.coverLetterByID(1, draft.ID)
	if saved.Body != edited {
		t.Fatal("manual save altered whitespace")
	}
}
