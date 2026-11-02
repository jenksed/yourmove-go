package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func writingQuestions(t *testing.T, a *App, jobID int64) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/questions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("questions status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func postWritingJSON(t *testing.T, a *App, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	return rec
}

func seedWritingJob(t *testing.T, a *App) (liveKey string, earlyIDs map[string]string) {
	t.Helper()
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	liveField := schema.Field{ID: "browser:story", Label: "Describe a time you resolved a production incident", Kind: "textarea", Required: true}
	liveID, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{liveField}})
	if err != nil {
		t.Fatal(err)
	}
	// Live questions are current only while the managed session is active at
	// that exact observation, mirroring legacy Prepare readiness.
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	a.browsers.states[1] = browserRunState{Active: true, Destination: testAshbyPosting, LatestID: liveID}
	a.browsers.mu.Unlock()
	// Ashby postings use the read-only preview discovery path, not public_api.
	earlyField := schema.Field{ID: "application:motivation", Label: "Why do you want this role?", Kind: "textarea", Required: true}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "preview", Completeness: "public_schema", ApplicationURL: testAshbyPosting, Fields: []schema.Field{earlyField}}); err != nil {
		t.Fatal(err)
	}
	return questionKey(testAshbyPosting, "ashby", liveField), map[string]string{
		"source": "preview", "question": "application:motivation",
	}
}

func TestWritingQuestionsDistinguishLiveAndEarly(t *testing.T) {
	a, _ := testWorkspace(t)
	liveKey, _ := seedWritingJob(t, a)
	payload := writingQuestions(t, a, 1)
	if payload["available"] != true {
		t.Fatalf("questions unavailable: %v", payload)
	}
	live, _ := payload["live"].([]any)
	early, _ := payload["early"].([]any)
	if len(live) == 0 || len(early) == 0 {
		t.Fatalf("live=%d early=%d, want both", len(live), len(early))
	}
	foundLive := false
	for _, item := range live {
		q := item.(map[string]any)
		if q["key"] == liveKey {
			foundLive = true
			if q["isEarly"] == true || q["label"] == "" || q["localStatus"] == "" {
				t.Fatalf("live question mislabeled: %v", q)
			}
		}
	}
	if !foundLive {
		t.Fatalf("live question %s missing: %v", liveKey, live)
	}
	for _, item := range early {
		q := item.(map[string]any)
		if q["isEarly"] != true || q["discoverySource"] == "" || q["sourceQuestionId"] == "" {
			t.Fatalf("early question missing identity: %v", q)
		}
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("questions read created %d submission reviews", got)
	}
}

func TestWritingAnswerSaveReopenAndConflict(t *testing.T) {
	a, _ := testWorkspace(t)
	liveKey, _ := seedWritingJob(t, a)
	save := func(revision int64, answer string) *httptest.ResponseRecorder {
		return postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
			`{"question_key":"`+liveKey+`","revision":`+itoa(revision)+`,"destination":"`+testAshbyPosting+`","answer":`+jsonString(answer)+`}`)
	}
	if rec := save(0, "I paged the on-call, found the slow query, and added an index."); rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload := writingQuestions(t, a, 1)
	var reopened string
	for _, item := range payload["live"].([]any) {
		q := item.(map[string]any)
		if q["key"] == liveKey {
			reopened, _ = q["value"].(string)
		}
	}
	if !strings.Contains(reopened, "slow query") {
		t.Fatalf("saved answer not reopened: %q", reopened)
	}
	// Stale revision must not overwrite.
	if rec := save(0, "Stale overwrite"); rec.Code != http.StatusConflict {
		t.Fatalf("stale save status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload = writingQuestions(t, a, 1)
	for _, item := range payload["live"].([]any) {
		q := item.(map[string]any)
		if q["key"] == liveKey && q["value"] == "Stale overwrite" {
			t.Fatal("stale revision overwrote the saved answer")
		}
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("writes created %d submission reviews", got)
	}
}

func TestWritingAnswerSelectValidation(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support"}); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:schedule", Label: "Preferred schedule", Kind: "select", Options: []string{"Remote", "Hybrid"}}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	key := questionKey(testAshbyPosting, "ashby", field)
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+key+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Onsite"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid option accepted: %d %s", rec.Code, rec.Body.String())
	}
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+key+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Remote"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid option refused: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWritingAnswerInactiveRefused(t *testing.T) {
	a, _ := testWorkspace(t)
	liveKey, _ := seedWritingJob(t, a)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+liveKey+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Saved before apply"}`); rec.Code != http.StatusOK {
		t.Fatalf("setup save: %d %s", rec.Code, rec.Body.String())
	}
	token := reviewedSubmissionToken(t, a, 1)
	if err := a.recordSubmission(1, "applied personally", token); err != nil {
		t.Fatal(err)
	}
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+liveKey+`","revision":1,"destination":"`+testAshbyPosting+`","answer":"After apply"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("post-submission save status=%d body=%s", rec.Code, rec.Body.String())
	}
	var stored string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1`).Scan(&stored); err != nil || stored != "Saved before apply" {
		t.Fatalf("saved writing changed: %q %v", stored, err)
	}
}

func TestWritingEarlyAnswerSaveReopen(t *testing.T) {
	a, _ := testWorkspace(t)
	_, ids := seedWritingJob(t, a)
	payload := writingQuestions(t, a, 1)
	var key, fingerprint string
	for _, item := range payload["early"].([]any) {
		q := item.(map[string]any)
		if q["sourceQuestionId"] == ids["question"] {
			key, _ = q["key"].(string)
			fingerprint, _ = q["discoveryFingerprint"].(string)
		}
	}
	if key == "" {
		t.Fatalf("early question missing: %v", payload["early"])
	}
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+key+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Early synthetic motivation","early":true,"discovery_source":"preview","discovery_fingerprint":"`+fingerprint+`","source_question_id":"application:motivation"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("early save status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload = writingQuestions(t, a, 1)
	for _, item := range payload["early"].([]any) {
		q := item.(map[string]any)
		if q["key"] == key && q["value"] != "Early synthetic motivation" {
			t.Fatalf("early answer not reopened: %v", q)
		}
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("early writes created %d submission reviews", got)
	}
}

func TestWritingAssistance(t *testing.T) {
	a, _ := testWorkspace(t)
	liveKey, _ := seedWritingJob(t, a)
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/assistance",
		`{"question_key":"`+liveKey+`","revision":0,"destination":"`+testAshbyPosting+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("assistance status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Providers       []answerAIProviderOption `json:"providers"`
		DefaultProvider string                 `json:"defaultProvider"`
		Sources         *answerAIChoices       `json:"sources"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Providers) == 0 || payload.DefaultProvider == "" || payload.Sources == nil {
		t.Fatalf("assistance incomplete: %+v", payload)
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("assistance created %d submission reviews", got)
	}
}

const writingTestLetter = "Dear Hiring Manager,\n\nI am excited to apply for the support role. I investigated slow MySQL queries, reviewed execution plans, and recommended an index that reduced load.\n\nSincerely,\nSynthetic Applicant"

func seedLetterJob(t *testing.T, a *App) {
	t.Helper()
	if err := a.saveJob(Job{URL: "https://example.com/support", Host: "example.com", Title: "Support Engineer", Company: "Fixture", Description: "Production incident experience required."}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Career leadership"))))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Linux and SQL support"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	a.coverLetterGenerate = func(context.Context, string) (string, error) { return writingTestLetter, nil }
}

func TestWritingLettersGenerateSaveReopen(t *testing.T) {
	a, _ := testWorkspace(t)
	seedLetterJob(t, a)
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters",
		`{"provider":"ollama","model":"","confirm_local":"yes"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var generated struct {
		Generated bool  `json:"generated"`
		DraftID   int64 `json:"draftId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &generated); err != nil || !generated.Generated || generated.DraftID == 0 {
		t.Fatalf("generate response: %s %v", rec.Body.String(), err)
	}
	list := postWritingJSON(t, a, http.MethodGet, "/api/jobs/1/letters", "")
	_ = list
	getRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/letters", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("letters status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var letters struct {
		Drafts  []map[string]any `json:"drafts"`
		EvidenceChoices []json.RawMessage `json:"evidenceChoices"`
		Current *struct {
			Body string `json:"body"`
		} `json:"current"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &letters); err != nil {
		t.Fatal(err)
	}
	if len(letters.Drafts) != 1 || letters.Current == nil || !strings.Contains(letters.Current.Body, "slow MySQL queries") {
		t.Fatalf("draft not listed: %s", getRec.Body.String())
	}
	if letters.EvidenceChoices == nil {
		t.Fatalf("empty evidence choices must be an array: %s", getRec.Body.String())
	}
	edited := "Dear Hiring Manager,\n\nI am excited to apply for the support role. Edited by the applicant with exact wording preserved.\n\nSincerely,\nSynthetic Applicant"
	save := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/1", `{"body":`+jsonString(edited)+`}`)
	if save.Code != http.StatusOK {
		t.Fatalf("letter save status=%d body=%s", save.Code, save.Body.String())
	}
	single := httptest.NewRecorder()
	newMux(a).ServeHTTP(single, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/letters/1", nil))
	var draft struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(single.Body.Bytes(), &draft); err != nil || draft.Body != edited {
		t.Fatalf("edited draft not reopened: %s %v", single.Body.String(), err)
	}
	// A second generation creates a new draft instead of overwriting the edit.
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters",
		`{"provider":"ollama","model":"","confirm_local":"yes"}`); rec.Code != http.StatusOK {
		t.Fatalf("second generate: %d %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters WHERE job_id=1`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("draft count=%d err=%v", count, err)
	}
	var first string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=1`).Scan(&first); err != nil || first != edited {
		t.Fatalf("first draft overwritten: %q %v", first, err)
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("letter reads created %d submission reviews", got)
	}
}

func TestWritingLettersWithReviewedEvidenceAndStaleness(t *testing.T) {
	a, _ := testWorkspace(t)
	seedLetterJob(t, a)
	evidence := activationEvidence(t, a, false)
	identity := itoa(evidence.ID) + ":" + itoa(evidence.Revision) + ":" + evidence.CreatedAt
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters",
		`{"provider":"ollama","model":"","confirm_local":"yes","career_evidence":[`+jsonString(identity)+`]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("evidence generate status=%d body=%s", rec.Code, rec.Body.String())
	}
	single := httptest.NewRecorder()
	newMux(a).ServeHTTP(single, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/letters/1", nil))
	var draft struct {
		EvidenceRefs  []letterEvidenceRef `json:"evidenceRefs"`
		EvidenceStale bool                `json:"evidenceStale"`
	}
	if err := json.Unmarshal(single.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if len(draft.EvidenceRefs) != 1 || draft.EvidenceStale {
		t.Fatalf("evidence refs not pinned: %s", single.Body.String())
	}
	// Revising the evidence record must surface staleness without rewriting the draft.
	content := careerEvidenceContent{Title: "Synthetic MySQL investigation", Situation: "Synthetic queries slowed down.", Actions: "I reviewed EXPLAIN plans, added tracing, and recommended an index.", OutcomeState: "unknown", Technologies: "MySQL"}
	if err := a.updateCareerEvidence(evidence.ID, evidence.Revision, content, true); err != nil {
		t.Fatal(err)
	}
	stale := httptest.NewRecorder()
	newMux(a).ServeHTTP(stale, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/letters/1", nil))
	var after struct {
		Body          string `json:"body"`
		EvidenceStale bool   `json:"evidenceStale"`
	}
	if err := json.Unmarshal(stale.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if !after.EvidenceStale || !strings.Contains(after.Body, "slow MySQL queries") {
		t.Fatalf("staleness not surfaced or draft changed: %s", stale.Body.String())
	}
}

func TestWritingLetterGenerationFailurePreservesDrafts(t *testing.T) {
	a, _ := testWorkspace(t)
	seedLetterJob(t, a)
	a.coverLetterGenerate = func(context.Context, string) (string, error) { return "", context.DeadlineExceeded }
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters",
		`{"provider":"ollama","model":"","confirm_local":"yes"}`)
	if rec.Code == http.StatusOK {
		t.Fatalf("failed generation reported success: %s", rec.Body.String())
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters WHERE job_id=1`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed generation wrote drafts: %d %v", count, err)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
