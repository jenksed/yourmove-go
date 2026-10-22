package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

const earlyGreenhousePosting = "https://job-boards.greenhouse.io/synthetic/jobs/12345"

func earlyGreenhouseFixture(t *testing.T, a *App) (Job, schema.Result) {
	t.Helper()
	job := Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io", Title: "Synthetic Engineer", Company: "Synthetic Co"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	result := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL, Note: "Public question definitions only. Conditional fields and later screens are not verified.", Fields: []schema.Field{
		{ID: "application:story", Label: "Describe a time you improved a system", Kind: "textarea", Required: true, Group: "application"},
		{ID: "application:work_type", Label: "Preferred schedule", Kind: "multi_value_single_select", Options: []string{"Remote", "Hybrid"}, Group: "application"},
		{ID: "compliance:authorization", Label: "Work authorization", Kind: "input_text", Sensitive: true, Group: "compliance"},
	}, Steps: []schema.Step{{Key: "public-application", Label: "Public application requirements", State: "anticipated", Action: "inspect"}}}
	return job, result
}

func earlyQuestionByID(t *testing.T, a *App, job Job, questionID string, query string) earlyApplicationQuestion {
	t.Helper()
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1"+query, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, question := range prep.Questions {
		if question.SourceQuestionID == questionID {
			return question
		}
	}
	t.Fatalf("early question %q not found", questionID)
	return earlyApplicationQuestion{}
}

func earlySaveValues(job Job, q earlyApplicationQuestion, revision string, answer string) url.Values {
	return url.Values{
		"question_key":          {q.Key},
		"revision":              {revision},
		"destination":           {job.URL},
		"discovery_source":      {q.DiscoverySource},
		"discovery_fingerprint": {q.DiscoveryFingerprint},
		"source_question_id":    {q.SourceQuestionID},
		"answer":                {answer},
	}
}

func TestGreenhouseEarlyDiscoveryDraftPersistenceAndLiveIsolation(t *testing.T) {
	a, dir := testWorkspace(t)
	job, result := earlyGreenhouseFixture(t, a)
	var calls atomic.Int32
	a.inspectApplication = func(_ context.Context, raw string) (schema.Result, error) {
		calls.Add(1)
		if raw != job.URL {
			t.Fatalf("inspected non-saved destination: %s", raw)
		}
		return result, nil
	}
	mux := newMux(a)
	post := postApp(mux, "/jobs/1/inspect", url.Values{"return_to": {"prepare"}}, "", "")
	if post.Code != 303 || post.Header().Get("Location") != "/jobs/1#prepare" || calls.Load() != 1 {
		t.Fatalf("discovery: %d %q calls=%d", post.Code, post.Header().Get("Location"), calls.Load())
	}
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if prep := earlyQuestionSummary(t, a, job); prep.SupportedQuestionCount != 2 || prep.ManualQuestionCount != 1 {
		t.Fatalf("logical preparation counts: supported=%d manual=%d", prep.SupportedQuestionCount, prep.ManualQuestionCount)
	}
	triage := earlyQuestionSummary(t, a, job)
	if len(triage.ActionableQuestions()) != 2 || len(triage.EmployerSiteQuestions()) != 1 || len(triage.ProfileQuestions()) != 0 || len(triage.ManualReviewQuestions()) != 0 {
		t.Fatalf("Greenhouse public-discovery triage: action=%d employer=%d profile=%d review=%d", len(triage.ActionableQuestions()), len(triage.EmployerSiteQuestions()), len(triage.ProfileQuestions()), len(triage.ManualReviewQuestions()))
	}
	for _, expected := range []string{"3 distinct questions discovered", "2 supported for early preparation", "1 manual-only or ambiguous", "public_api", "Describe a time you improved a system", "Preferred schedule", "Remote", "Required", "not verified"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("Prepare omitted %q: %s", expected, page.Body.String())
		}
	}
	q := earlyQuestionByID(t, a, job, "application:story", "?early_question=")
	answer := "I reviewed the telemetry.\n\nThen I recommended a measured change — with supporting evidence."
	post = postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", answer), "", "")
	if post.Code != 303 || !strings.Contains(post.Header().Get("Location"), "early_question=") {
		t.Fatalf("early save: %d %s", post.Code, post.Body.String())
	}
	var saved string
	var revision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM early_application_answers WHERE job_id=1 AND source_question_id=?`, q.SourceQuestionID).Scan(&saved, &revision); err != nil || saved != answer || revision != 1 {
		t.Fatalf("early answer was not preserved exactly: %q revision=%d err=%v", saved, revision, err)
	}
	var liveCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&liveCount); err != nil || liveCount != 0 {
		t.Fatalf("early answer entered the live-answer table: count=%d err=%v", liveCount, err)
	}
	profile, _ := a.candidateProfile()
	live, err := a.prepareQuestions(job, profile)
	if err != nil || live.InspectionID != 0 || len(live.Questions) != 0 {
		t.Fatalf("public questions became browser transfer scope: %+v err=%v", live, err)
	}
	if conflict := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", "stale overwrite"), "", ""); conflict.Code != 409 {
		t.Fatalf("stale early editor accepted: %d", conflict.Code)
	}
	answer = "Résumé notes with Unicode — saved from the desktop route."
	desktop := postApp(newDesktopMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, q, "1", answer), "wails://localhost", "1")
	if desktop.Code != 200 || !strings.Contains(desktop.Body.String(), `window.location.replace("/jobs/1?early_question=`) {
		t.Fatalf("desktop early save redirect contract: %d %s", desktop.Code, desktop.Body.String())
	}
	if _, repeatErr := a.saveInspection(job.ID, result); repeatErr != nil {
		t.Fatal(repeatErr)
	}
	if countErr := a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers WHERE job_id=1`).Scan(&liveCount); countErr != nil || liveCount != 1 {
		t.Fatalf("identical discovery duplicated an early answer: %d %v", liveCount, countErr)
	}
	// A new public-schema fingerprint retains the old wording and answer for review.
	result.Fields[0].Label = "Describe a time you changed a production database"
	if _, err := a.saveInspection(job.ID, result); err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil || len(prep.Previous) != 1 || prep.Previous[0].Answer != answer || prep.Previous[0].QuestionLabel != "Describe a time you improved a system" || !strings.Contains(prep.Previous[0].CurrentQuestionState, "changed") {
		t.Fatalf("changed schema did not preserve the original draft: %+v err=%v", prep.Previous, err)
	}
	// Restarting the local app retains the separate early draft.
	a.Close()
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, db, filepath.Join(dir, "resumes"))
	var restartedAnswer string
	if err := restarted.db.QueryRow(`SELECT answer FROM early_application_answers WHERE job_id=1 AND source_question_id=?`, q.SourceQuestionID).Scan(&restartedAnswer); err != nil || restartedAnswer != answer {
		t.Fatalf("restart lost early answer: %q %v", restartedAnswer, err)
	}
}

func earlyQuestionSummary(t *testing.T, a *App, job Job) earlyQuestionPreparation {
	t.Helper()
	preparation, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	return preparation
}

func TestEarlyDraftRemainsVisibleWhenDestinationChangesBeforeNewDiscovery(t *testing.T) {
	a, _ := testWorkspace(t)
	job, result := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, result); err != nil {
		t.Fatal(err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "")
	answer := "Keep this draft with its original employer destination."
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", answer), "", ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("save early answer: %d %s", rec.Code, rec.Body.String())
	}
	changedURL := "https://job-boards.greenhouse.io/synthetic/jobs/98765"
	if _, err := a.db.Exec(`UPDATE jobs SET url=? WHERE id=?`, changedURL, job.ID); err != nil {
		t.Fatal(err)
	}
	changedJob, err := a.jobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := a.prepareEarlyQuestions(changedJob, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil || prepared.Item != nil || len(prepared.Previous) != 1 {
		t.Fatalf("destination change hid the earlier draft: %+v err=%v", prepared, err)
	}
	previous := prepared.Previous[0]
	if previous.Answer != answer || previous.Destination != job.URL || previous.Reconciliation.Status != "Manual only" || !strings.Contains(previous.CurrentQuestionState, "different employer destination") {
		t.Fatalf("changed-destination draft was lost or offered for reuse: %+v", previous)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if !strings.Contains(page.Body.String(), "Earlier prepared answers preserved") || !strings.Contains(page.Body.String(), answer) || !strings.Contains(page.Body.String(), "cannot be reconciled with the current destination") {
		t.Fatal("Prepare did not explain and preserve the earlier destination draft")
	}
}

func TestAshbyEarlyDraftSurvivesNormalizedIdentityChange(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic role"}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	result := schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: job.URL + "/application", Fields: []schema.Field{
		{ID: "preview:entry-old__story", Label: "Describe a technical support issue you resolved", Kind: "textarea"},
	}}
	if _, err := a.saveInspection(job.ID, result); err != nil {
		t.Fatal(err)
	}
	question := earlyQuestionByID(t, a, job, "preview:entry-old__story", "")
	answer := "I documented the symptoms and verified the resolution."
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, question, "0", answer), "", ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("save early answer: %d %s", rec.Code, rec.Body.String())
	}

	result.Fields[0].ID = "preview:entry-new__story"
	if _, err := a.saveInspection(job.ID, result); err != nil {
		t.Fatal(err)
	}
	preparation := earlyQuestionSummary(t, a, job)
	if len(preparation.Questions) != 1 || preparation.Questions[0].SourceQuestionID != "preview:entry-new__story" ||
		len(preparation.Previous) != 1 || preparation.Previous[0].SourceQuestionID != "preview:entry-old__story" ||
		preparation.Previous[0].Answer != answer {
		t.Fatalf("identity change lost or reused the original draft: current=%+v previous=%+v", preparation.Questions, preparation.Previous)
	}
	if preparation.Questions[0].Revision != 0 {
		t.Fatalf("old answer was silently attached to the new field identity: %+v", preparation.Questions[0])
	}
}

func TestEarlyQuestionTypeSafetyAndAIUsesExistingReviewTicket(t *testing.T) {
	a, _ := testWorkspace(t)
	job, result := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, result); err != nil {
		t.Fatal(err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "?early_question=")
	if !q.Editable || q.AIChoices == nil || !narrativeAnswerEligible(preparedQuestion{Field: q.Field, Editable: q.Editable}) {
		t.Fatalf("eligible early narrative question missing existing AI choices: %+v", q)
	}
	choice := addCareerEvidenceForSearch(t, a, careerEvidenceContent{Title: "Database performance improvement", Situation: "A reporting system had slow queries.", Actions: "I reviewed the plans and recommended a targeted index.", OutcomeState: "unknown"}, true)
	var calls atomic.Int32
	a.applicationAnswerGenerate = func(_ context.Context, prompt, _ string) (string, error) {
		calls.Add(1)
		if !strings.Contains(prompt, "published application question") || !strings.Contains(prompt, q.Field.Label) {
			t.Fatalf("early question was not distinguished in model context: %s", prompt)
		}
		return "I reviewed the query plans and recommended a targeted index.", nil
	}
	selection := url.Values{
		"question_key":          {q.Key},
		"revision":              {"0"},
		"destination":           {job.URL},
		"early":                 {"yes"},
		"discovery_source":      {q.DiscoverySource},
		"discovery_fingerprint": {q.DiscoveryFingerprint},
		"source_question_id":    {q.SourceQuestionID},
		"provider":              {"ollama"},
		"model":                 {"synthetic-model"},
		"career_evidence":       {fmt.Sprintf("%d:%d:%s", choice.ID, choice.Revision, choice.CreatedAt)},
	}
	review := postApp(newMux(a), "/jobs/1/answers/ai/review", selection, "", "")
	if review.Code != 200 || calls.Load() != 0 || !strings.Contains(review.Body.String(), `"preparation_kind":"early"`) {
		t.Fatalf("early AI source review: %d %s calls=%d", review.Code, review.Body.String(), calls.Load())
	}
	var response struct{ Ticket string }
	if err := json.Unmarshal(review.Body.Bytes(), &response); err != nil || response.Ticket == "" {
		t.Fatalf("generation ticket missing: %+v %v", response, err)
	}
	generated := postApp(newMux(a), "/jobs/1/answers/ai/generate", generationForm(response.Ticket), "", "")
	if generated.Code != 200 || calls.Load() != 1 || !strings.Contains(generated.Body.String(), "targeted index") {
		t.Fatalf("early AI preview: %d %s calls=%d", generated.Code, generated.Body.String(), calls.Load())
	}
	var earlyCount, applicationCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&earlyCount); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&applicationCount); err != nil {
		t.Fatal(err)
	}
	if earlyCount != 0 || applicationCount != 0 {
		t.Fatalf("AI proposal saved without applicant action: early=%d live=%d", earlyCount, applicationCount)
	}
	// A refreshed question definition invalidates the outstanding preview.
	result.Fields[0].Label = "Describe a time you improved reliability"
	if _, err := a.saveInspection(job.ID, result); err != nil {
		t.Fatal(err)
	}
	if check := postApp(newMux(a), "/jobs/1/answers/ai/adopt-check", url.Values{"ticket": {response.Ticket}, "question_key": {q.Key}, "revision": {"0"}}, "", ""); check.Code != 409 {
		t.Fatalf("stale early AI proposal passed adoption check: %d %s", check.Code, check.Body.String())
	}
	if calls.Load() != 1 {
		t.Fatal("stale AI adoption triggered another provider call")
	}
	selectQuestion := earlyQuestionByID(t, a, job, "compliance:authorization", "")
	if selectQuestion.Editable || selectQuestion.ManualReason == "" {
		t.Fatal("sensitive public question was exposed as an early editor")
	}
	for _, field := range []schema.Field{
		{ID: "preview:race__group", Label: "Race and ethnicity", Kind: "checkbox_group", Sensitive: true, Options: []string{"Option A", "Option B"}},
		{ID: "preview:tools__group", Label: "Tools used", Kind: "checkbox_group", Options: []string{"Tool A", "Tool B"}},
		{ID: "preview:interest__group", Label: "Areas of interest", Kind: "custom_checkbox_group", Options: []string{"Customer success", "Technical support"}},
		{ID: "preview:years__group", Label: "Support experience", Kind: "custom_radio_group", Options: []string{"One year", "Two years"}},
		{ID: "preview:location", Label: "Current location", Kind: "custom_combobox"},
	} {
		if editable, _ := earlyQuestionWritable(field, true); editable {
			t.Fatalf("newly grouped/custom question became eligible for early preparation: %+v", field)
		}
	}
}

func BenchmarkEarlyQuestionPreparation(b *testing.B) {
	a, job := benchmarkEarlyWorkspace(b, 100)
	r := httptest.NewRequest("GET", "http://localhost/jobs/1", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.prepareEarlyQuestions(job, r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEarlyAnswerSave(b *testing.B) {
	a, job := benchmarkEarlyWorkspace(b, 100)
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		b.Fatal(err)
	}
	q := prep.Questions[0]
	mux := newMux(a)
	answer := strings.Repeat("Synthetic multiline response with reviewed details. ", 20)
	revision := int64(0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, fmt.Sprint(revision), answer), "", "")
		if response.Code != 303 {
			b.Fatalf("save failed: %d %s", response.Code, response.Body.String())
		}
		revision++
	}
}

func BenchmarkPrepareRenderWithDiscoveredQuestions(b *testing.B) {
	a, _ := benchmarkEarlyWorkspace(b, 100)
	handler := newMux(a)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
		if response.Code != 200 {
			b.Fatalf("Prepare render failed: %d", response.Code)
		}
	}
}

func BenchmarkEarlyAnswerReconciliation100(b *testing.B) {
	a, job := benchmarkEarlyWorkspace(b, 100)
	public, err := a.latestInspectionBySource(job.ID, "public_api")
	if err != nil {
		b.Fatal(err)
	}
	liveFields := make([]schema.Field, len(public.Fields))
	for i, field := range public.Fields {
		name := strings.TrimPrefix(field.ID, "application:")
		liveFields[i] = schema.Field{ID: "browser:" + name, Label: field.Label, Kind: "textarea", Required: field.Required}
	}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fields: liveFields}); err != nil {
		b.Fatal(err)
	}
	now := answerLibraryTimestamp()
	tx, err := a.db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	for _, field := range public.Fields {
		if _, err := tx.Exec(`INSERT INTO early_application_answers(job_id,destination,discovery_source,source_question_id,question_label,field_snapshot_json,discovery_fingerprint,answer,revision,reconciliation_status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,1,'unreconciled',?,?)`, job.ID, job.URL, "public_api", field.ID, field.Label, earlyFieldSnapshot(field.Field), public.Fingerprint, "Synthetic prepared answer", now, now); err != nil {
			tx.Rollback()
			b.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://localhost/jobs/1", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.prepareEarlyQuestions(job, r); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkEarlyWorkspace(b *testing.B, questionCount int) (*App, Job) {
	dir := b.TempDir()
	db, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		b.Fatal(err)
	}
	resumeDir := filepath.Join(dir, "resumes")
	if err := os.Mkdir(resumeDir, 0700); err != nil {
		b.Fatal(err)
	}
	files := os.DirFS("../..")
	coverLetter, err := fs.ReadFile(files, "prompts/cover_letter.md")
	if err != nil {
		b.Fatal(err)
	}
	answerInstructions, err := fs.ReadFile(files, "prompts/application_answer.md")
	if err != nil {
		b.Fatal(err)
	}
	a, err := New(Config{DB: db, ResumeDir: resumeDir, TemplateFiles: files, StaticFiles: files, CoverLetterInstructions: string(coverLetter), ApplicationAnswerInstructions: string(answerInstructions)})
	if err != nil {
		b.Fatal(err)
	}
	job := Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io"}
	if err := a.saveJob(job); err != nil {
		b.Fatal(err)
	}
	job, _ = a.jobByID(1)
	fields := make([]schema.Field, questionCount)
	for i := range fields {
		fields[i] = schema.Field{ID: fmt.Sprintf("application:response_%03d", i), Label: fmt.Sprintf("Application response field %03d", i), Kind: "textarea", Group: "application"}
	}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL, Fields: fields}); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { a.Close(); db.Close() })
	return a, job
}
