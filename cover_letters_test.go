package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
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
		for _, phrase := range []string{"Managed production incidents", "Linux and MySQL troubleshooting", "Work on Linux incidents"} {
			if !strings.Contains(prompt, phrase) {
				t.Errorf("missing prompt source %q", phrase)
			}
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
	if w.Code != 303 || w.Header().Get("Location") != coverNoticeURL(1, "sources") || calls != 0 {
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

// A minimal selectable-text PDF ensures letter generation is not DOCX-only.
func TestExtractPDFTextForCoverLetter(t *testing.T) {
	var b bytes.Buffer
	offsets := []int{0}
	b.WriteString("%PDF-1.4\n")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len("BT /F1 12 Tf 72 700 Td (Linux support production troubleshooting and incident response) Tj ET"), "BT /F1 12 Tf 72 700 Td (Linux support production troubleshooting and incident response) Tj ET"),
	}
	for n, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", n+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	text, err := extractPDFText(b.Bytes())
	if err != nil || !strings.Contains(text, "Linux") || !strings.Contains(text, "incident") {
		t.Fatalf("PDF text %q error %v", text, err)
	}
}

func TestCoverProviderHTTPContracts(t *testing.T) {
	const letter = "I have worked on production troubleshooting, incident recovery and Linux systems for several years."
	examples := []struct {
		provider string
		response string
	}{
		{"ollama", `{"response":"` + letter + `"}`},
		{"openai", `{"status":"completed","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"` + letter + `"}]}]}`},
		{"anthropic", `{"content":[{"type":"text","text":"` + letter + `"}]}`},
		{"openrouter", `{"choices":[{"message":{"content":"` + letter + `"}}]}`},
		{"compatible", `{"choices":[{"message":{"content":"` + letter + `"}}]}`},
	}
	for _, example := range examples {
		t.Run(example.provider, func(t *testing.T) {
			const key = "private-unit-test-key"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("method %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing content type")
				}
				if example.provider == "anthropic" {
					if r.Header.Get("x-api-key") != key || r.Header.Get("anthropic-version") != "2023-06-01" {
						t.Error("Anthropic header contract")
					}
				} else if example.provider != "ollama" && r.Header.Get("Authorization") != "Bearer "+key {
					t.Error("missing bearer credential")
				}
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if !bytes.Contains(data, []byte("trusted career experience")) {
					t.Error("prompt omitted")
				}
				if !bytes.Contains(data, []byte("sample-model")) {
					t.Error("model omitted")
				}
				if example.provider == "openai" && !bytes.Contains(data, []byte(`"store":false`)) {
					t.Error("OpenAI request must disable storage")
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, example.response)
			}))
			defer server.Close()
			got, err := runCoverProvider(context.Background(), coverBackend{Provider: example.provider, Model: "sample-model", URL: server.URL, APIKey: key}, "trusted career experience", nil)
			if err != nil || got != letter {
				t.Fatalf("got %q error %v", got, err)
			}
		})
	}
}

func TestCoverProviderFailureCategories(t *testing.T) {
	tests := []struct {
		status int
		body   string
		want   string
	}{
		{401, `{"error":{"message":"not authorized"}}`, "auth"},
		{403, `{}`, "auth"},
		{402, `{}`, "credits"},
		{429, `{}`, "rate"},
		{404, `{}`, "model-missing"},
		{504, `{}`, "timeout"},
		{503, `{}`, "provider-unavailable"},
		{200, `{"choices":[]}`, "output"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprint(tc.status)+tc.want, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			_, err := runCoverProvider(context.Background(), coverBackend{Provider: "openrouter", Model: "test", URL: server.URL, APIKey: "secret"}, "prompt", nil)
			if got := coverErrorNotice(err); got != tc.want {
				t.Fatalf("got %q wanted %q", got, tc.want)
			}
		})
	}
}

func TestCoverProviderDoesNotFollowRedirect(t *testing.T) {
	forwarded := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	_, err := runCoverProvider(context.Background(), coverBackend{Provider: "openai", Model: "test", URL: server.URL, APIKey: "secret"}, "prompt", nil)
	if coverErrorNotice(err) != "provider-unavailable" || forwarded != 0 {
		t.Fatalf("redirect followed: err=%v requests=%d", err, forwarded)
	}
}

func TestCoverProviderConfiguration(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if _, err := configuredCoverBackend("openai", "gpt-4.1-mini"); coverErrorNotice(err) != "key" {
		t.Fatalf("missing key %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "test-123")
	cfg, err := configuredCoverBackend("openai", "gpt-4.1-mini")
	if err != nil || cfg.APIKey != "test-123" || cfg.URL != "https://api.openai.com/v1/responses" {
		t.Fatalf("openai config %+v %v", cfg, err)
	}
	t.Setenv("YOURMOVE_COMPAT_URL", "http://example.com/v1/chat/completions")
	if _, err := configuredCoverBackend("compatible", "test"); !errors.Is(err, errCoverEndpoint) {
		t.Fatalf("unsafe endpoint %v", err)
	}
	t.Setenv("YOURMOVE_COMPAT_URL", "http://127.0.0.1:9001/v1/chat/completions")
	if _, err := configuredCoverBackend("compatible", "test"); err != nil {
		t.Fatalf("loopback endpoint %v", err)
	}
	if validCompatibleEndpoint("https://example.com/v1/chat/completions?key=secret") {
		t.Fatal("query credentials accepted")
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
