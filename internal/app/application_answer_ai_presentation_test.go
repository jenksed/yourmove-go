package app

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNarrativeWorkbenchPresentationIsOptionalAndUnselected(t *testing.T) {
	a, _, _ := narrativeFixture(t)
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	for _, s := range []string{"Draft answer with AI", "data-answer-ai-controls hidden", "Inline AI drafting requires JavaScript", "Use as unsaved answer", "data-answer-ai-payload", "name=\"career_evidence\""} {
		if !strings.Contains(body, s) {
			t.Fatal("missing", s)
		}
	}
	if strings.Contains(body, `name="career_evidence" checked`) {
		t.Fatal("auto-selected evidence")
	}
	start, end := strings.Index(body, `class="application-question-form"`), strings.Index(body, `data-answer-ai-controls`)
	if start < 0 || !strings.Contains(body[start:end], "</form>") {
		t.Fatal("drafting nested in answer save form")
	}
	if _, err := a.db.Exec(`UPDATE jobs SET stage=? WHERE id=1`, StageApplied); err != nil {
		t.Fatal(err)
	}
	page = httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if strings.Contains(page.Body.String(), "data-answer-ai-controls") {
		t.Fatal("completed application offers drafting")
	}
}

func TestNarrativeResumeAlternativeDoesNotRequireCareerEvidence(t *testing.T) {
	a, _, evidenceID := narrativeFixture(t)
	if _, err := a.db.Exec(`DELETE FROM career_evidence WHERE id=?`, evidenceID); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("synthetic-answer-evidence.docx", bytes.NewReader(sampleDOCXForPreview(t, "Reviewed synthetic support experience")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	if page.Code != 200 || !strings.Contains(body, `name="application_resume"`) || !strings.Contains(body, "A resume can be used without creating Career Evidence first") {
		t.Fatalf("resume evidence alternative is unavailable: %d %s", page.Code, body)
	}
	if strings.Contains(body, `name="application_resume" checked`) {
		t.Fatal("resume evidence was selected automatically")
	}
}

func TestPrepareExplainsMissingObservedQuestionsAndDraftEvidence(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic opportunity"}); err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if !strings.Contains(page.Body.String(), "No application questions captured yet. Open the application to inspect them.") || !strings.Contains(page.Body.String(), "AI drafting needs a supported question observed in the managed employer browser.") {
		t.Fatal("missing observed-question guidance", page.Code, page.Body.String())
	}

	b, _, evidenceID := narrativeFixture(t)
	if _, err := b.db.Exec(`DELETE FROM career_evidence WHERE id=?`, evidenceID); err != nil {
		t.Fatal(err)
	}
	page = httptest.NewRecorder()
	newMux(b).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if !strings.Contains(page.Body.String(), "No usable resume is available.") || !strings.Contains(page.Body.String(), "Import or repair a resume") || !strings.Contains(page.Body.String(), "review Career Evidence") {
		t.Fatal("missing factual-source recovery guidance", page.Code, page.Body.String())
	}
}

func TestApplicationAnswerProviderOptionsExplainDefaultsAndReadiness(t *testing.T) {
	options := applicationAnswerAIProviders(map[string]bool{"openai": false, "anthropic": true, "compatible": false, "codex": true})
	byID := map[string]answerAIProviderOption{}
	for _, option := range options {
		byID[option.ID] = option
	}
	if byID["ollama"].Model == "" || !strings.Contains(byID["ollama"].Help, "Keep the service running") {
		t.Fatalf("Ollama default/readiness is unclear: %+v", byID["ollama"])
	}
	if !strings.Contains(byID["openai"].Help, "OPENAI_API_KEY") || !strings.Contains(byID["anthropic"].Help, "available in this launch environment") {
		t.Fatalf("API key state is unclear: openai=%+v anthropic=%+v", byID["openai"], byID["anthropic"])
	}
	if !strings.Contains(byID["compatible"].Help, "YOURMOVE_COMPAT_URL") || !strings.Contains(byID["codex"].Help, "sign-in is checked") {
		t.Fatalf("endpoint or CLI readiness is unclear: compatible=%+v codex=%+v", byID["compatible"], byID["codex"])
	}
}
