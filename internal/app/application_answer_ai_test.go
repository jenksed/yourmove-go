package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func narrativeFixture(t *testing.T) (*App, url.Values, int64) {
	t.Helper()
	a, _, job, _, key := newAnswerLibraryFixture(t)
	record := addCareerEvidenceForSearch(t, a, careerEvidenceContent{Title: "Synthetic MySQL investigation", Situation: "Synthetic queries slowed down.", Actions: "I reviewed EXPLAIN plans and recommended an index.", OutcomeState: "unknown", SourceNotes: "PRIVATE_SOURCE_NOTES"}, true)
	id := record.ID
	e, err := careerEvidenceByID(a.db, id)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`UPDATE career_evidence SET source_writing='PRIVATE_ORIGINAL_WRITING',source_question='PRIVATE_OLD_QUESTION',outcome='UNSUPPORTED_RESULT' WHERE id=?`, id)
	if err != nil {
		t.Fatal(err)
	}
	return a, url.Values{"destination": {job.URL}, "question_key": {key}, "revision": {"0"}, "provider": {"ollama"}, "model": {"synthetic-model"}, "career_evidence": {fmt.Sprintf("%d:%d:%s", id, e.Revision, e.CreatedAt)}}, id
}
func reviewNarrative(t *testing.T, a *App, v url.Values) string {
	t.Helper()
	r := postApp(newMux(a), "/jobs/1/answers/ai/review", v, "", "")
	if r.Code != 200 {
		t.Fatalf("review %d: %s", r.Code, r.Body.String())
	}
	var result struct{ Ticket string }
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Ticket
}
func generationForm(ticket string) url.Values {
	return url.Values{"ticket": {ticket}, "authorize_generation": {"yes"}, "authorize_external": {"yes"}}
}
func TestNarrativePolicyAndOutput(t *testing.T) {
	for _, tc := range []struct {
		field          schema.Field
		editable, want bool
	}{
		{schema.Field{ID: "browser:story", Label: answerLibraryStory, Kind: "textarea"}, true, true},
		{schema.Field{ID: "browser:story", Label: answerLibraryStory, Kind: "textarea"}, false, false},
		{schema.Field{ID: "browser:story", Label: "Describe your work authorization", Kind: "textarea"}, true, false},
		{schema.Field{ID: "browser:story", Label: answerLibraryStory, Kind: "select"}, true, false},
		{schema.Field{ID: "browser:story", Label: strings.Repeat("describe ", 600), Kind: "textarea"}, true, false},
	} {
		if narrativeAnswerEligible(preparedQuestion{Field: tc.field, Editable: tc.editable}) != tc.want {
			t.Fatal(tc)
		}
	}
	for _, good := range []string{"I investigated it.", "I reviewed EXPLAIN plans and recommended an index.", insufficientAnswer} {
		if err := validateGeneratedApplicationAnswer(good); err != nil {
			t.Fatal(good, err)
		}
	}
	for _, bad := range []string{"", " ", "\xff", "a\x00", strings.Repeat("x", 6001), strings.Repeat("a ", 601), insufficientAnswer + " prose", "<think>analysis</think>Answer", `{"answer":"text"}`, "```text```", "Assistant: proposed answer", "[Insert answer]"} {
		if validateGeneratedApplicationAnswer(bad) == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
func TestNarrativeReviewSelectionAndStaleness(t *testing.T) {
	a, v, id := narrativeFixture(t)
	var calls atomic.Int32
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) { calls.Add(1); return "I reviewed plans.", nil }
	ticket := reviewNarrative(t, a, v)
	if calls.Load() != 0 {
		t.Fatal("review called provider")
	}
	a.answerAI.mu.Lock()
	s := a.answerAI.tickets[ticket].snapshot
	a.answerAI.mu.Unlock()
	for _, private := range []string{"PRIVATE_SOURCE_NOTES", "PRIVATE_ORIGINAL_WRITING", "PRIVATE_OLD_QUESTION", "UNSUPPORTED_RESULT"} {
		if strings.Contains(s.Payload, private) {
			t.Fatal("projection leaked", private)
		}
	}
	if !strings.Contains(s.Payload, "unknown") || !strings.Contains(s.Instructions, "phrasing and structure") {
		t.Fatal("authority missing")
	}
	if !a.validateAnswerGenerationSnapshot(s) {
		t.Fatal("fresh snapshot stale")
	}
	for _, status := range []string{"draft", "reviewed"} {
		_, err := a.db.Exec(`UPDATE career_evidence SET status=?,revision=revision+1 WHERE id=?`, status, id)
		if err != nil {
			t.Fatal(err)
		}
		if a.validateAnswerGenerationSnapshot(s) {
			t.Fatal("changed source accepted")
		}
	}
	if r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", ""); r.Code != 409 {
		t.Fatal(r.Code, r.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatal("stale transmission")
	}
	v.Del("career_evidence")
	if _, err := a.buildAnswerGenerationSnapshot(1, v); err == nil {
		t.Fatal("no factual evidence")
	}
}
func TestNarrativeLifecycleDuplicateAndNoPersistence(t *testing.T) {
	a, v, _ := narrativeFixture(t)
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	a.applicationAnswerGenerate = func(ctx context.Context, prompt, instructions string) (string, error) {
		calls.Add(1)
		close(entered)
		select {
		case <-release:
			return "I reviewed EXPLAIN plans and recommended an index.", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	ticket := reviewNarrative(t, a, v)
	second := reviewNarrative(t, a, v)
	if r := postApp(newMux(a), "/jobs/1/answers/ai/generate", url.Values{"ticket": {ticket}}, "", ""); r.Code != 403 {
		t.Fatal(r.Code)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
		if r.Code != 200 {
			t.Errorf("generate: %d %s", r.Code, r.Body.String())
		}
	}()
	<-entered
	for i := 0; i < 8; i++ {
		r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
		if r.Code != 200 || !strings.Contains(r.Body.String(), "running") {
			t.Fatal(r.Code, r.Body.String())
		}
	}
	if r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(second), "", ""); r.Code != 409 {
		t.Fatal(r.Code)
	}
	close(release)
	wg.Wait()
	r := postApp(newDesktopMux(a), "/jobs/1/answers/ai/adopt-check", url.Values{"ticket": {ticket}, "question_key": v["question_key"], "revision": {"0"}}, "wails://localhost", "")
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"adoptable":true`) || !strings.HasPrefix(r.Header().Get("Content-Type"), "application/json") || r.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(r.Code, r.Body.String())
	}
	_ = postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
	if calls.Load() != 1 {
		t.Fatal("duplicate provider call", calls.Load())
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&count); err != nil || count != 0 {
		t.Fatal("generation persisted", count, err)
	}
	_, err := a.db.Exec(`UPDATE jobs SET stage=? WHERE id=1`, StageApplied)
	if err != nil {
		t.Fatal(err)
	}
	r = postApp(newMux(a), "/jobs/1/answers/ai/adopt-check", url.Values{"ticket": {ticket}, "question_key": v["question_key"], "revision": {"0"}}, "", "")
	if r.Code != 409 {
		t.Fatal(r.Code)
	}
}
func TestNarrativeCancelExpiryCapacityAndShutdown(t *testing.T) {
	a, v, _ := narrativeFixture(t)
	a.applicationAnswerGenerate = func(ctx context.Context, _, _ string) (string, error) { <-ctx.Done(); return "", ctx.Err() }
	ticket := reviewNarrative(t, a, v)
	r := postApp(newMux(a), "/jobs/1/answers/ai/cancel", url.Values{"ticket": {ticket}}, "", "")
	if r.Code != 200 {
		t.Fatal(r.Code)
	}
	r = postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
	if !strings.Contains(r.Body.String(), "canceled") {
		t.Fatal(r.Body.String())
	}
	a.answerAI.mu.Lock()
	a.answerAI.tickets[ticket].expires = time.Now().Add(-time.Second)
	a.answerAI.mu.Unlock()
	r = postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
	if r.Code != 410 {
		t.Fatal(r.Code)
	}
	for i := 0; i < 16; i++ {
		reviewNarrative(t, a, v)
	}
	r = postApp(newMux(a), "/jobs/1/answers/ai/review", v, "", "")
	if r.Code != 409 {
		t.Fatal(r.Code)
	}
	a.Close()
	if _, _, ok := a.answerAI.add(answerGenerationSnapshot{}); ok {
		t.Fatal("review after shutdown")
	}
}
func TestNarrativeStrictRequestAndStaleDuringGeneration(t *testing.T) {
	a, v, id := narrativeFixture(t)
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		_, err := a.db.Exec(`UPDATE career_evidence SET revision=revision+1 WHERE id=?`, id)
		return "I reviewed query plans.", err
	}
	for _, field := range []string{"source_text", "endpoint", "authorize_external"} {
		v.Set(field, "untrusted")
		r := postApp(newMux(a), "/jobs/1/answers/ai/review", v, "", "")
		if r.Code != 400 {
			t.Fatal(field, r.Code)
		}
		v.Del(field)
	}
	v.Add("model", "duplicate")
	if r := postApp(newMux(a), "/jobs/1/answers/ai/review", v, "", ""); r.Code != 400 {
		t.Fatal(r.Code)
	}
	v["model"] = v["model"][:1]
	ticket := reviewNarrative(t, a, v)
	r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"adoptable":false`) || !strings.Contains(r.Body.String(), "stale") {
		t.Fatal(r.Code, r.Body.String())
	}
}

func TestNarrativeSourcesIdentityBudgetsAndLibraryAuthority(t *testing.T) {
	a, v, id := narrativeFixture(t)
	// Oversized curated experience is rejected, never truncated.
	_, err := a.db.Exec(`UPDATE career_evidence SET actions=? WHERE id=?`, strings.Repeat("action ", 2000), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.buildAnswerGenerationSnapshot(1, v); err == nil {
		t.Fatal("oversized evidence accepted")
	}
	_, err = a.db.Exec(`UPDATE career_evidence SET actions='I reviewed query plans.' WHERE id=?`, id)
	if err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	key := v.Get("question_key")
	saveLibrarySourceAnswer(t, a, job, key, "I invented a result in an earlier writing example.", 0)
	if r := promoteLibrarySource(t, a, 1, key, 1); r.Code != 303 {
		t.Fatal(r.Code)
	}
	entries, err := a.answerLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	v.Set("revision", "1")
	v.Set("library_entry", fmt.Sprintf("%d:%d:%s", entry.ID, entry.Revision, entry.CreatedAt))
	s, err := a.buildAnswerGenerationSnapshot(1, v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.Payload, "phrasing and structure only") {
		t.Fatal("writing authority")
	}
	if err := a.updateAnswerLibrary(entry.ID, entry.Revision, "Edited synthetic writing."); err != nil {
		t.Fatal(err)
	}
	if a.validateAnswerGenerationSnapshot(s) {
		t.Fatal("edited library accepted")
	}
	v.Del("career_evidence")
	v.Del("library_entry")
	v.Set("include_job_context", "yes")
	if _, err := a.buildAnswerGenerationSnapshot(1, v); err == nil {
		t.Fatal("job context sufficient alone")
	}
	// Creation identities detect ID reuse even if revision is unchanged.
	v.Set("career_evidence", s.Selection.Get("career_evidence"))
	v.Del("include_job_context")
	_, err = a.db.Exec(`UPDATE career_evidence SET created_at='new-identity' WHERE id=?`, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.buildAnswerGenerationSnapshot(1, v); err == nil {
		t.Fatal("reused identity accepted")
	}
}
func TestNarrativeInsufficientFailureAndSnapshotBaseline(t *testing.T) {
	for _, output := range []string{insufficientAnswer, "<think>trace</think>Answer", "I reviewed plans."} {
		t.Run(output, func(t *testing.T) {
			a, v, _ := narrativeFixture(t)
			a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) { return output, nil }
			ticket := reviewNarrative(t, a, v)
			r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(ticket), "", "")
			if output == insufficientAnswer && (!strings.Contains(r.Body.String(), `"state":"insufficient"`) || strings.Contains(r.Body.String(), insufficientAnswer)) {
				t.Fatal(r.Body.String())
			}
			if strings.Contains(output, "<think>") && r.Code != 422 {
				t.Fatal(r.Code)
			}
			a.answerAI.mu.Lock()
			s := a.answerAI.tickets[ticket].snapshot
			a.answerAI.mu.Unlock()
			job, _ := a.jobByID(1)
			saveLibrarySourceAnswer(t, a, job, v.Get("question_key"), "Manually authored answer.", 0)
			if a.validateAnswerGenerationSnapshot(s) {
				t.Fatal("saved revision changed")
			}
		})
	}
}
func TestNarrativeShutdownCancelsRunningWithoutHoldingLocks(t *testing.T) {
	a, v, _ := narrativeFixture(t)
	entered := make(chan struct{})
	done := make(chan struct{})
	a.applicationAnswerGenerate = func(ctx context.Context, _, _ string) (string, error) {
		close(entered)
		<-ctx.Done()
		return "", ctx.Err()
	}
	token := reviewNarrative(t, a, v)
	go func() {
		defer close(done)
		postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(token), "", "")
	}()
	<-entered
	// A local review and DB read can proceed while the provider waits.
	reviewNarrative(t, a, v)
	if _, err := a.jobByID(1); err != nil {
		t.Fatal(err)
	}
	a.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not cancel")
	}
}
func TestNarrativeInstructionsAreIndependentAndBounded(t *testing.T) {
	a, _ := testWorkspace(t)
	c := testConfig(t, a.db, a.resumeDir)
	for _, bad := range []string{"", strings.Repeat("a", 8193), "\xff"} {
		c.ApplicationAnswerInstructions = bad
		if _, err := New(c); err == nil {
			t.Fatal("invalid narrative resource accepted")
		}
	}
	c = testConfig(t, a.db, a.resumeDir)
	c.ApplicationAnswerInstructions = "Independent narrative instructions."
	b, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.resources.applicationAnswerInstructions == b.resources.applicationAnswerInstructions || a.resources.coverLetterInstructions != b.resources.coverLetterInstructions {
		t.Fatal("coupled instructions")
	}
}
func TestNarrativeResumeIntegrityAndContext(t *testing.T) {
	a, v, _ := narrativeFixture(t)
	v.Del("career_evidence")
	resume, err := a.importResume("PRIVATE_filename.docx", bytes.NewReader(sampleDOCXForPreview(t, strings.Repeat("Synthetic resume evidence. ", 500))))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	resume, err = a.resumeByID(resume.ID)
	if err != nil {
		t.Fatal(err)
	}
	v.Set("application_resume", fmt.Sprintf("%d:%d:%s:%s", resume.ID, resume.Version, resume.SHA256, resume.ImportedAt))
	v.Set("include_job_context", "yes")
	_, err = a.db.Exec(`UPDATE jobs SET description=? WHERE id=1`, strings.Repeat("Untrusted employer requirements. ", 500))
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.buildAnswerGenerationSnapshot(1, v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.Payload, "PRIVATE_filename") || strings.Contains(s.Payload, a.resumeDir) || !strings.Contains(s.Payload, "Excerpt truncated") || !strings.Contains(s.Payload, "not applicant evidence") {
		t.Fatal("resume privacy or context authority")
	}
	for _, source := range s.Sources {
		limit := 4096
		if strings.Contains(source.Category, "Employer") {
			limit = 3072
		}
		if len(answerAIJSON(source)) > limit {
			t.Fatal("serialized limit", len(answerAIJSON(source)), limit)
		}
	}
	if !a.validateAnswerGenerationSnapshot(s) {
		t.Fatal("fresh resume rejected")
	}
	_, err = a.db.Exec(`UPDATE resumes SET archived_at='now' WHERE id=?`, resume.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a.validateAnswerGenerationSnapshot(s) {
		t.Fatal("archived accepted")
	}
	_, err = a.db.Exec(`UPDATE resumes SET archived_at='' WHERE id=?`, resume.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.resumeDir, resume.StoredName), []byte("changed bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if a.validateAnswerGenerationSnapshot(s) {
		t.Fatal("changed bytes accepted")
	}
}

func TestNarrativeJSONOriginErrorsAndSafeProviderErrors(t *testing.T) {
	a, v, _ := narrativeFixture(t)
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		return "", fmt.Errorf("PRIVATE_PROVIDER_KEY PRIVATE_SOURCE_PAYLOAD")
	}
	for _, handler := range []http.Handler{newMux(a), newDesktopMux(a)} {
		r := postApp(handler, "/jobs/1/answers/ai/review", v, "https://foreign.invalid", "")
		if r.Code != 403 || !strings.HasPrefix(r.Header().Get("Content-Type"), "application/json") || r.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(r.Code, r.Header())
		}
	}
	token := reviewNarrative(t, a, v)
	r := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(token), "", "")
	if r.Code != 502 || strings.Contains(r.Body.String(), "PRIVATE_") {
		t.Fatal(r.Code, r.Body.String())
	}
	r = postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(token), "", "")
	if r.Code != 502 {
		t.Fatal("failure replay", r.Code)
	}
}

func TestNarrativeProviderSetupAndRuntimeFailuresGiveNextSteps(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("YOURMOVE_COMPAT_URL", "")
	if got := answerAIProviderNotice("ollama", "", nil); got != "model" {
		t.Fatalf("missing model notice=%q", got)
	}
	if got := answerAIProviderNotice("openai", "gpt-4.1-mini", nil); got != "key" {
		t.Fatalf("missing API key notice=%q", got)
	}
	if got := answerAIProviderNotice("compatible", "", nil); got != "model" {
		t.Fatalf("missing compatible model notice=%q", got)
	}
	for _, tc := range []struct{ provider, model, notice, want string }{
		{"ollama", "qwen3.5:4b", "offline", "127.0.0.1:11434"},
		{"ollama", "missing-model", "model-missing", "ollama list"},
		{"codex", "default", "login", "codex login"},
		{"claude-cli", "sonnet", "cli", "Claude Code"},
	} {
		if message := answerAIProviderMessage(tc.provider, tc.model, tc.notice); !strings.Contains(message, tc.want) {
			t.Fatalf("%s %s message=%q, want %q", tc.provider, tc.notice, message, tc.want)
		}
	}
	a, values, _ := narrativeFixture(t)
	values.Set("model", "")
	r := postApp(newMux(a), "/jobs/1/answers/ai/review", values, "", "")
	if r.Code != 422 || !strings.Contains(r.Body.String(), "Enter a model supported by this provider") {
		t.Fatalf("missing-model review=%d %s", r.Code, r.Body.String())
	}
	values.Set("provider", "openai")
	values.Set("model", "gpt-4.1-mini")
	r = postApp(newMux(a), "/jobs/1/answers/ai/review", values, "", "")
	if r.Code != 422 || !strings.Contains(r.Body.String(), "OPENAI_API_KEY") {
		t.Fatalf("missing-key review=%d %s", r.Code, r.Body.String())
	}
}

func TestNarrativeExcerptsPreserveCompleteProvenance(t *testing.T) {
	a, v, _ := narrativeFixture(t)
	job, _ := a.jobByID(1)
	key := v.Get("question_key")
	saveLibrarySourceAnswer(t, a, job, key, strings.Repeat("Résumé experience. ", 500), 0)
	if r := promoteLibrarySource(t, a, 1, key, 1); r.Code != 303 {
		t.Fatal(r.Code)
	}
	entries, err := a.answerLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	e := entries[0]
	v.Set("revision", "1")
	v.Set("library_entry", fmt.Sprintf("%d:%d:%s", e.ID, e.Revision, e.CreatedAt))
	v.Set("include_job_context", "yes")
	if _, err := a.db.Exec(`UPDATE jobs SET description=? WHERE id=1`, strings.Repeat("Employer requirements. ", 500)); err != nil {
		t.Fatal(err)
	}
	s, err := a.buildAnswerGenerationSnapshot(1, v)
	if err != nil {
		t.Fatal(err)
	}
	var writing struct{ Question, Employer, Job, Host, Example string }
	var context struct{ Title, Employer, Description string }
	for _, source := range s.Sources {
		if strings.HasPrefix(source.Category, "Previous writing") {
			if err := json.Unmarshal([]byte(source.Content), &writing); err != nil {
				t.Fatal("broken writing serialization", err)
			}
			if len(answerAIJSON(source)) > 4096 {
				t.Fatal("writing budget")
			}
		}
		if strings.HasPrefix(source.Category, "Employer requirements") {
			if err := json.Unmarshal([]byte(source.Content), &context); err != nil {
				t.Fatal("broken context serialization", err)
			}
			if len(answerAIJSON(source)) > 3072 {
				t.Fatal("context budget")
			}
		}
	}
	if writing.Question != e.OriginalQuestion || writing.Employer != e.SourceEmployer || writing.Job != e.SourceJobTitle || writing.Host != e.SourceHost || !strings.HasSuffix(writing.Example, "[Excerpt truncated]") {
		t.Fatal("lost provenance", writing)
	}
	if context.Title != job.Title || context.Employer != job.Company || !strings.HasSuffix(context.Description, "[Excerpt truncated]") {
		t.Fatal("lost context", context)
	}
	// Do not silently throw away provenance to admit an otherwise impossible source.
	if _, err := a.db.Exec(`UPDATE answer_library SET original_question=? WHERE id=?`, strings.Repeat("Original question ", 500), e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.buildAnswerGenerationSnapshot(1, v); err == nil {
		t.Fatal("oversized provenance accepted")
	}
	response := postApp(newMux(a), "/jobs/1/answers/ai/review", v, "", "")
	if response.Code != 422 || !strings.Contains(response.Body.String(), `"code":"provenance_budget"`) || !strings.Contains(response.Body.String(), "Choose another previous-writing example") {
		t.Fatal(response.Code, response.Body.String())
	}
}

func TestNarrativeEscapedUTF8ExcerptsKeepActualEvidence(t *testing.T) {
	original := "Résumé investigation: " + strings.Repeat("<>& résumé ", 1000)
	source, err := boundedAnswerAISource(answerAISource{Category: "resume", Content: original}, 512)
	if err != nil || len(answerAIJSON(source)) > 512 || !validNarrativeText(source.Content) || !strings.HasPrefix(source.Content, "Résumé investigation:") || !strings.HasSuffix(source.Content, "[Excerpt truncated]") {
		t.Fatal("escaped excerpt", source, err)
	}
	if _, err := boundedAnswerAISource(answerAISource{Category: strings.Repeat("metadata", 100), Content: "actual facts"}, 50); err == nil {
		t.Fatal("metadata-only source admitted")
	}
}
