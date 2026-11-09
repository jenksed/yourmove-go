package app

import (
	"context"
	"github.com/jenksed/yourmove-go/internal/application/browser"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// A-B-C acceptance: Save once, automatically prepare identical and approved-
// alternative questions, keep related wording manual, protect explicit work,
// require selection on conflict, and survive restart.
func TestAnswerLibraryAutomaticPreparationABC(t *testing.T) {
	a, dir := testWorkspace(t)
	labelA := "Describe a production incident you resolved for a customer"
	answerA := "At 02:14 I reviewed Linux logs, isolated the failing service, restored it, and documented the follow-up for the customer."

	// Application A: eligible question, Save to Answer Library in one step.
	urlA := testAshbyPosting
	if err := a.saveJob(Job{URL: urlA, Host: "jobs.ashbyhq.com", Title: "Support A", Company: "Fixture A"}); err != nil {
		t.Fatal(err)
	}
	fieldA := schema.Field{ID: "browser:incident-story", Label: labelA, Kind: "textarea", Required: true}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlA + "/application", Fields: []schema.Field{fieldA}}); err != nil {
		t.Fatal(err)
	}
	keyA := questionKey(urlA, "ashby", fieldA)
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+keyA+`","revision":0,"destination":"`+urlA+`","answer":`+jsonString(answerA)+`,"save_to_library":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("A save to library: %d %s", rec.Code, rec.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("A library: %+v %v", entries, err)
	}
	entry := entries[0]

	// Application B: identical question on a different posting auto-populates.
	urlB := "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000001"
	if err := a.saveJob(Job{URL: urlB, Host: "jobs.ashbyhq.com", Title: "Support B", Company: "Fixture B"}); err != nil {
		t.Fatal(err)
	}
	fieldB := schema.Field{ID: "browser:incident-story", Label: labelA, Kind: "textarea", Required: true, Group: "rendered form"}
	a.browsers.pollEvery = 10 * time.Millisecond
	a.browsers.factory = func(_ context.Context, raw string) (readOnlySession, error) {
		return &captureNarrativeBrowser{fakeBrowser: fakeBrowser{url: raw + "/application"}, field: fieldB}, nil
	}
	defer a.browsers.close(2)
	request := httptest.NewRequest("POST", "http://localhost/jobs/2/browser/start", strings.NewReader(""))
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	newMux(a).ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for a.browsers.state(2).LatestID == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if a.browsers.state(2).LatestID == 0 {
		t.Fatal("start-only observer did not capture equivalent library question")
	}
	jobB, _ := a.jobByID(2)
	profile, _ := a.candidateProfile()
	prepB, err := a.prepareQuestions(jobB, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepB.Questions) != 1 {
		t.Fatalf("B questions: %+v", prepB.Questions)
	}
	qB := prepB.Questions[0]
	if qB.Value != answerA {
		t.Fatalf("B identical question not auto-populated: %q", qB.Value)
	}
	if qB.Revision != 0 || qB.LibraryAutoEntryID != entry.ID {
		t.Fatalf("B auto must stay unsaved with provenance: rev=%d auto=%d", qB.Revision, qB.LibraryAutoEntryID)
	}
	if qB.Source != libraryAutoSource {
		t.Fatalf("B auto source: %q", qB.Source)
	}
	// Suggestions still surface the exact match for review.
	payloadB := s2Questions(t, a, 2)
	foundAuto := false
	for _, item := range payloadB["live"].([]any) {
		q := item.(map[string]any)
		if v, _ := q["value"].(string); v == answerA {
			foundAuto = true
			if q["libraryAutoPrepared"] != true {
				t.Fatalf("B JSON missing auto flag: %v", q)
			}
		}
	}
	if !foundAuto {
		t.Fatal("B JSON did not carry auto value")
	}

	// Existing application-specific writing is never overwritten: save a
	// distinct answer on B, then re-prepare and confirm it wins over auto.
	keyB := questionKey(urlB, "ashby", fieldB)
	saveB := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/answers",
		`{"question_key":"`+keyB+`","revision":0,"destination":"`+urlB+`","answer":`+jsonString("Employer-specific adaptation for B.")+`}`)
	if saveB.Code != http.StatusOK {
		t.Fatalf("B explicit save: %d %s", saveB.Code, saveB.Body.String())
	}
	prepB2, _ := a.prepareQuestions(jobB, profile)
	if prepB2.Questions[0].Value != "Employer-specific adaptation for B." || prepB2.Questions[0].LibraryAutoEntryID != 0 {
		t.Fatalf("B explicit answer not protected: %+v", prepB2.Questions[0])
	}

	// Application C: approved alternative phrasing auto-populates.
	alt := "Tell us about a production incident you resolved for a customer"
	if err := a.updateAnswerLibraryWithAlternatives(entry.ID, entry.Revision, answerA, []string{alt}); err != nil {
		t.Fatal(err)
	}
	// Refresh entry revision for later checks.
	entries, _ = a.answerLibraryEntries()
	entry = entries[0]
	urlC := "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000002"
	if err := a.saveJob(Job{URL: urlC, Host: "jobs.ashbyhq.com", Title: "Support C", Company: "Fixture C"}); err != nil {
		t.Fatal(err)
	}
	fieldC := schema.Field{ID: "browser:incident-story", Label: alt, Kind: "textarea", Required: true}
	if _, err := a.saveInspection(3, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlC + "/application", Fields: []schema.Field{fieldC}}); err != nil {
		t.Fatal(err)
	}
	jobC, _ := a.jobByID(3)
	prepC, _ := a.prepareQuestions(jobC, profile)
	if len(prepC.Questions) != 1 || prepC.Questions[0].Value != answerA || prepC.Questions[0].LibraryAutoEntryID != entry.ID {
		t.Fatalf("C alternative not auto-populated: %+v", prepC.Questions)
	}
	// Automatic preparation never enters Fill scope until explicitly saved.
	if len(prepC.ReadyAnswers()) != 0 {
		t.Fatalf("auto must not be Fill-ready: %+v", prepC.ReadyAnswers())
	}
	// After an explicit save, the answer becomes Fill-ready and auto clears.
	keyC := questionKey(urlC, "ashby", fieldC)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/3/answers",
		`{"question_key":"`+keyC+`","revision":0,"destination":"`+urlC+`","answer":`+jsonString(answerA)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("C explicit save of auto text: %d %s", rec.Code, rec.Body.String())
	}
	prepC2, _ := a.prepareQuestions(jobC, profile)
	if len(prepC2.ReadyAnswers()) != 1 || prepC2.Questions[0].LibraryAutoEntryID != 0 {
		t.Fatalf("saved auto text should be Fill-ready without auto flag: %+v", prepC2.Questions[0])
	}

	// Related but unapproved wording stays a suggestion, not auto-population.
	urlD := "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000003"
	if err := a.saveJob(Job{URL: urlD, Host: "jobs.ashbyhq.com", Title: "Support D", Company: "Fixture D"}); err != nil {
		t.Fatal(err)
	}
	relatedLabel := "Describe a difficult customer interaction you handled"
	fieldD := schema.Field{ID: "browser:incident-story", Label: relatedLabel, Kind: "textarea", Required: true}
	if _, err := a.saveInspection(4, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlD + "/application", Fields: []schema.Field{fieldD}}); err != nil {
		t.Fatal(err)
	}
	jobD, _ := a.jobByID(4)
	prepD, _ := a.prepareQuestions(jobD, profile)
	if len(prepD.Questions) != 1 {
		t.Fatalf("D questions: %+v", prepD.Questions)
	}
	if prepD.Questions[0].Value != "" || prepD.Questions[0].LibraryAutoEntryID != 0 {
		t.Fatalf("D related wording must not auto-populate: %+v", prepD.Questions[0])
	}

	// Ambiguous equivalents require selection: add a second entry with the
	// same alternative, then auto must stop for that wording.
	urlE := "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000004"
	if err := a.saveJob(Job{URL: urlE, Host: "jobs.ashbyhq.com", Title: "Support E", Company: "Fixture E"}); err != nil {
		t.Fatal(err)
	}
	fieldE := schema.Field{ID: "browser:incident-story", Label: "Describe a checkout outage you resolved quietly", Kind: "textarea", Required: true}
	if _, err := a.saveInspection(5, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlE + "/application", Fields: []schema.Field{fieldE}}); err != nil {
		t.Fatal(err)
	}
	keyE := questionKey(urlE, "ashby", fieldE)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/5/answers",
		`{"question_key":"`+keyE+`","revision":0,"destination":"`+urlE+`","answer":`+jsonString("Second story with enough narrative wording to qualify.")+`,"save_to_library":true}`); rec.Code != http.StatusOK {
		t.Fatalf("E save: %d %s", rec.Code, rec.Body.String())
	}
	secondEntries, _ := a.answerLibraryEntries()
	var second AnswerLibraryEntry
	for _, e := range secondEntries {
		if e.SourceJobID == 5 {
			second = e
		}
	}
	if second.ID == 0 {
		t.Fatal("second entry missing")
	}
	// Both entries claim the same alternative phrasing.
	if err := a.updateAnswerLibraryWithAlternatives(second.ID, second.Revision, second.Answer, []string{alt}); err != nil {
		t.Fatal(err)
	}
	// Fresh posting with the ambiguous wording must not auto-populate; the
	// existing saved answer on job 3 stays protected and is not rechecked here.
	urlF := "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000005"
	if err := a.saveJob(Job{URL: urlF, Host: "jobs.ashbyhq.com", Title: "Support F", Company: "Fixture F"}); err != nil {
		t.Fatal(err)
	}
	fieldF := schema.Field{ID: "browser:incident-story", Label: alt, Kind: "textarea", Required: true}
	if _, err := a.saveInspection(6, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlF + "/application", Fields: []schema.Field{fieldF}}); err != nil {
		t.Fatal(err)
	}
	jobF, _ := a.jobByID(6)
	prepConflict, _ := a.prepareQuestions(jobF, profile)
	if prepConflict.Questions[0].LibraryAutoEntryID != 0 || prepConflict.Questions[0].Value != "" {
		t.Fatalf("conflicting equivalents must not auto-populate: %+v", prepConflict.Questions[0])
	}
	suggestions, err := a.answerLibrarySuggestionsForQuestions([]preparedQuestion{{Key: "k", Field: fieldC, Editable: true}})
	if err != nil || len(suggestions["k"]) != 2 {
		t.Fatalf("conflict suggestions: %+v %v", suggestions, err)
	}

	// Editing application-specific text never mutates the library.
	before, _ := a.answerLibraryBySourceID(a.db, entry.ID)
	if before.Answer != answerA {
		t.Fatalf("library changed by app saves: %q", before.Answer)
	}

	// Restart persistence: library answer and alternatives survive reopen,
	// and the surviving primary entry still auto-populates new work.
	if err := a.db.QueryRow(`SELECT answer FROM answer_library WHERE id=?`, entry.ID).Scan(&before.Answer); err != nil {
		t.Fatal(err)
	}
	alts, err := a.libraryAlternatives(entry.ID)
	if err != nil || len(alts) != 1 || alts[0] != alt {
		t.Fatalf("alternatives before restart: %v %v", alts, err)
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, reopenedDB, filepath.Join(dir, "resumes"))
	var reopenedAnswer string
	if err := restarted.db.QueryRow(`SELECT answer FROM answer_library WHERE id=?`, entry.ID).Scan(&reopenedAnswer); err != nil || reopenedAnswer != answerA {
		t.Fatalf("restart lost library answer: %q %v", reopenedAnswer, err)
	}
	reopenedAlts, err := restarted.libraryAlternatives(entry.ID)
	if err != nil || len(reopenedAlts) != 1 || reopenedAlts[0] != alt {
		t.Fatalf("restart lost alternatives: %v %v", reopenedAlts, err)
	}
	// New posting with the surviving primary wording still auto-populates.
	urlG := "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000006"
	if err := restarted.saveJob(Job{URL: urlG, Host: "jobs.ashbyhq.com", Title: "Support G", Company: "Fixture G"}); err != nil {
		t.Fatal(err)
	}
	fieldG := schema.Field{ID: "browser:incident-story", Label: labelA, Kind: "textarea", Required: true}
	jobG, _ := restarted.jobByID(7)
	if _, err := restarted.saveInspection(7, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlG + "/application", Fields: []schema.Field{fieldG}}); err != nil {
		t.Fatal(err)
	}
	profileG, _ := restarted.candidateProfile()
	prepG, _ := restarted.prepareQuestions(jobG, profileG)
	if len(prepG.Questions) != 1 || prepG.Questions[0].Value != answerA {
		t.Fatalf("restart auto missing: %+v", prepG.Questions)
	}
	_ = strconv.Itoa(0)
	_ = strings.TrimSpace("")
}

func TestAnswerLibraryEarlyAutomaticPreparation(t *testing.T) {
	a, _ := testWorkspace(t)
	label := "Describe a time you improved a system"
	answer := "I reviewed the alert history, isolated the slow query, and documented the incident response."
	// Seed the library via a live application (Application A).
	urlA := testAshbyPosting
	if err := a.saveJob(Job{URL: urlA, Host: "jobs.ashbyhq.com", Title: "Support A", Company: "Fixture A"}); err != nil {
		t.Fatal(err)
	}
	fieldA := schema.Field{ID: "browser:story", Label: label, Kind: "textarea", Required: true}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlA + "/application", Fields: []schema.Field{fieldA}}); err != nil {
		t.Fatal(err)
	}
	keyA := questionKey(urlA, "ashby", fieldA)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+keyA+`","revision":0,"destination":"`+urlA+`","answer":`+jsonString(answer)+`,"save_to_library":true}`); rec.Code != http.StatusOK {
		t.Fatalf("seed library: %d %s", rec.Code, rec.Body.String())
	}
	entries, _ := a.answerLibraryEntries()
	if len(entries) != 1 {
		t.Fatalf("seed entries: %+v", entries)
	}
	// Early discovery on a different posting with the identical wording
	// auto-populates without a durable early answer.
	urlB := earlyGreenhousePosting
	jobB := Job{URL: urlB, Host: "job-boards.greenhouse.io", Title: "Synthetic Engineer", Company: "Synthetic Co"}
	if err := a.saveJob(jobB); err != nil {
		t.Fatal(err)
	}
	jobB, _ = a.jobByID(2)
	result := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: urlB, Fields: []schema.Field{
		{ID: "application:story", Label: label, Kind: "textarea", Required: true},
	}}
	if _, err := a.saveInspection(2, result); err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareEarlyQuestions(jobB, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(prep.Questions) != 1 {
		t.Fatalf("early questions: %+v", prep.Questions)
	}
	q := prep.Questions[0]
	if q.Value != answer || q.LibraryAutoEntryID != entries[0].ID || q.Revision != 0 {
		t.Fatalf("early auto missing: %+v", q)
	}
	// Early auto is presentation-only: no durable row is created by reading.
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers WHERE job_id=?`, jobB.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("early auto wrote durable state: %d %v", count, err)
	}
}

type captureNarrativeBrowser struct {
	fakeBrowser
	field schema.Field
}

func (f *captureNarrativeBrowser) Observe(context.Context) (browser.Observation, error) {
	return browser.Observation{URL: f.url, Forms: 1, Controls: []browser.Control{{ID: strings.TrimPrefix(f.field.ID, "browser:"), Label: f.field.Label, Kind: f.field.Kind, Required: f.field.Required}}}, nil
}
