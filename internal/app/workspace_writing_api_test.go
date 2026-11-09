package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
 "path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
 "github.com/jenksed/yourmove-go/internal/storage"
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
		`{"question_key":"`+key+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Early synthetic motivation","early":true,"save_to_library":true,"discovery_source":"preview","discovery_fingerprint":"`+fingerprint+`","source_question_id":"application:motivation"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("early save status=%d body=%s", rec.Code, rec.Body.String())
	}
	entries,err:=a.answerLibraryEntries();if err!=nil||len(entries)!=1 {t.Fatalf("combined early save: %v %v",entries,err)}
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

func TestWritingQuestionsIncludeManualCompletionFields(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Render posting"}); err != nil {
		t.Fatal(err)
	}
	// Mirrors the observed Render preview: Website is a supported text field
	// while "How did you hear about Render?" arrives as an option-less Ashby
	// combobox that must stay manual-only but still be represented.
	previewFields := []schema.Field{
		{ID: "preview:form-namespace-1111-website", Label: "Website", Kind: "text"},
		{ID: "preview:form-namespace-2222-hear", Label: "How did you hear about Render?", Kind: "combobox"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: testAshbyPosting + "/application", Fields: previewFields}); err != nil {
		t.Fatal(err)
	}
	liveFields := []schema.Field{
		{ID: "browser:form-namespace-1111-website", Label: "Website", Kind: "text"},
		{ID: "browser:form-namespace-2222-hear", Label: "How did you hear about Render?", Kind: "combobox"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: liveFields}); err != nil {
		t.Fatal(err)
	}
	payload := writingQuestions(t, a, 1)
	early, _ := payload["early"].([]any)
	if len(early) != 2 {
		t.Fatalf("early inventory hid a discovered question: %v", payload["early"])
	}
	var sawEditable, sawManual bool
	for _, item := range early {
		q := item.(map[string]any)
		switch q["label"] {
		case "Website":
			if q["editable"] != true {
				t.Fatalf("Website should stay editable: %v", q)
			}
			sawEditable = true
		case "How did you hear about Render?":
			if q["editable"] != false || q["triageCategory"] != "review" || q["manualReason"] == "" {
				t.Fatalf("combobox source question misrepresented: %v", q)
			}
			sawManual = true
		}
	}
	if !sawEditable || !sawManual {
		t.Fatalf("early inventory incomplete: editable=%v manual=%v", sawEditable, sawManual)
	}
	live, _ := payload["live"].([]any)
	if len(live) != 2 {
		t.Fatalf("live inventory hid a discovered question: %v", payload["live"])
	}
	counts, _ := payload["manualReviewCount"].(float64)
	if counts != 1 {
		t.Fatalf("manual review count=%v, want 1", payload["manualReviewCount"])
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("questions read created %d submission reviews", got)
	}
}

func TestWritingQuestionsUnifiedProjection(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Render posting"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.test", Phone: "555-0100", City: "Portland", Region: "Oregon", Country: "USA", LinkedIn: "https://example.test/ada", Pronouns: "They/them", Approved: true}); err != nil {
		t.Fatal(err)
	}
	nsA := "aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa"
	nsB := "bbbbbbbb-2222-4222-8222-bbbbbbbbbbbb"
	quid := map[string]string{
		"phone": "c91a27dd-b714-41a6-9484-50ea8a524f94", "web": "d0f9c56d-5a87-4137-9a0a-f0241d0af505",
		"story": "e523cd20-b243-4230-9a13-50ac028dd5a0", "hear": "f73c44bf-c40a-45b2-973a-858bc7c851ed",
		"pronouns": "a93c8020-5d35-4732-98bc-3e6fd7535612", "race": "b1111111-1111-4111-8111-111111111111",
		"vet": "c2222222-2222-4222-8222-222222222222", "infoA": "d3333333-3333-4333-8333-333333333333",
		"infoB": "e4444444-4444-4444-8444-444444444444", "extra": "f5555555-5555-4555-8555-555555555555",
		"workauth": "b6666666-4666-4666-8666-666666666666",
	}
	previewID := func(suffix string) string { return "preview:" + nsA + suffix }
	liveID := func(suffix string) string { return "browser:" + nsB + suffix }
	storyLabel := "Tell us about a developer product you genuinely enjoy using and why."
	previewFields := []schema.Field{
		{ID: previewID("__systemfield_name"), Label: "Name", Kind: "text", Required: true},
		{ID: previewID("__systemfield_email"), Label: "Email", Kind: "email", Required: true},
		{ID: previewID("_"+quid["phone"]), Label: "Phone", Kind: "tel", Required: true},
		{ID: previewID("__systemfield_location"), Label: "Location", Kind: "combobox"},
		{ID: previewID("_fd231a7d-aac9-4546-88f4-7d796da5e281"), Label: "LinkedIn", Kind: "text"},
		{ID: previewID("_"+quid["web"]), Label: "Website", Kind: "text"},
		{ID: previewID("_"+quid["story"]), Label: storyLabel, Kind: "text"},
		{ID: previewID("_"+quid["hear"]), Label: "How did you hear about Render?", Kind: "combobox"},
		{ID: previewID("_"+quid["pronouns"]), Label: "Pronouns", Kind: "text"},
		{ID: previewID("_"+quid["race"]), Label: "Race", Kind: "radio", Sensitive: true, Options: []string{"Asian", "Decline to self-identify"}},
		{ID: previewID("_"+quid["workauth"]), Label: "Work authorization status", Kind: "text"},
		{ID: previewID("_"+quid["vet"]), Label: "Veteran Status", Kind: "radio", Sensitive: true, Options: []string{"I am not a protected veteran", "I decline to self-identify"}},
		{ID: previewID("__systemfield_resume"), Label: "Resume", Kind: "file", Required: true},
		{ID: previewID("_aab326cd-f9c6-4608-9262-4a98b773a08e"), Label: "Cover Letter", Kind: "file"},
		{ID: "preview:unidentified-controls", Label: "Controls without a stable field identity", Kind: "unsupported_ambiguous"},
		{ID: previewID("_"+quid["infoA"]), Label: "Other information", Kind: "text"},
		{ID: previewID("_"+quid["extra"]), Label: "Extra note", Kind: "text"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: testAshbyPosting + "/application", Fields: previewFields}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	drafts := map[string]string{previewID("_"+quid["web"]): "https://example.test/portfolio", previewID("_"+quid["story"]): "I enjoy using Render because deploys are boring in the best way."}
	for id, value := range drafts {
		q := earlyQuestionByID(t, a, job, id, "")
		if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", value), "", ""); rec.Code != 303 {
			t.Fatalf("save %s: %d %s", id, rec.Code, rec.Body.String())
		}
	}
	liveFields := []schema.Field{
		{ID: liveID("__systemfield_name"), Label: "Name", Kind: "text", Required: true},
		{ID: liveID("__systemfield_email"), Label: "Email", Kind: "email", Required: true},
		{ID: liveID("_"+quid["phone"]), Label: "Phone", Kind: "tel", Required: true},
		{ID: liveID("__systemfield_location"), Label: "Location", Kind: "combobox"},
		{ID: liveID("_fd231a7d-aac9-4546-88f4-7d796da5e281"), Label: "LinkedIn", Kind: "text"},
		{ID: liveID("_"+quid["web"]), Label: "Website", Kind: "text"},
		{ID: liveID("_"+quid["story"]), Label: storyLabel, Kind: "text"},
		{ID: liveID("_"+quid["hear"]), Label: "How did you hear about Render?", Kind: "combobox"},
		{ID: liveID("_"+quid["pronouns"]), Label: "Pronouns", Kind: "radio", Options: []string{"She/her", "They/them", "He/him", "Decline to state"}},
		{ID: liveID("_"+quid["race"]), Label: "Race", Kind: "radio", Sensitive: true, Options: []string{"Asian", "Decline to self-identify"}},
		{ID: liveID("_"+quid["workauth"]), Label: "Work authorization status", Kind: "radio", Options: []string{"Yes", "No"}},
		{ID: liveID("_"+quid["vet"]), Label: "Veteran Status", Kind: "radio", Sensitive: true, Options: []string{"I am not a protected veteran", "I decline to self-identify"}},
		{ID: liveID("__systemfield_resume"), Label: "Resume", Kind: "file", Required: true},
		{ID: liveID("_aab326cd-f9c6-4608-9262-4a98b773a08e"), Label: "Cover Letter", Kind: "file"},
		{ID: "browser:unidentified-controls", Label: "Controls without a stable field identity", Kind: "unsupported_ambiguous"},
		{ID: liveID("_"+quid["infoA"]), Label: "Other information", Kind: "text"},
		{ID: liveID("_"+quid["infoB"]), Label: "Other information", Kind: "text"},
		{ID: liveID("_"+quid["extra"]), Label: "Extra note", Kind: "textarea"},
	}
	liveRowID, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: liveFields})
	if err != nil {
		t.Fatal(err)
	}

	unifiedByLabel := func(payload map[string]any) map[string][]map[string]any {
		out := make(map[string][]map[string]any)
		questions, _ := payload["questions"].([]any)
		for _, item := range questions {
			q := item.(map[string]any)
			label, _ := q["label"].(string)
			out[label] = append(out[label], q)
		}
		return out
	}
	answerLabels := func(byLabel map[string][]map[string]any) []string {
		var labels []string
		for label, rows := range byLabel {
			for _, q := range rows {
				if q["triageCategory"] == "answer" {
					labels = append(labels, label)
				}
			}
		}
		return labels
	}

	// Phase 1: preview is current (browser closed). Saved drafts keep their
	// early editor identity; profile, employer, and manual rows are categorized.
	payload := writingQuestions(t, a, 1)
	byLabel := unifiedByLabel(payload)
	for label, want := range map[string]int{"Website": 1, storyLabel: 1, "How did you hear about Render?": 1, "Name": 1, "Email": 1, "Phone": 1, "Location": 1, "LinkedIn": 1, "Pronouns": 1, "Race": 1, "Work authorization status": 1, "Veteran Status": 1, "Other information": 2, "Extra note": 2} {
		if len(byLabel[label]) != want {
			t.Fatalf("phase 1 %q rows=%d, want %d", label, len(byLabel[label]), want)
		}
	}
	for _, absent := range []string{"Resume", "Cover Letter", "Controls without a stable field identity"} {
		if len(byLabel[absent]) != 0 {
			t.Fatalf("phase 1 %q should not be a question row: %v", absent, byLabel[absent])
		}
	}
	if got, _ := payload["unidentifiedCount"].(float64); got != 1 {
		t.Fatalf("phase 1 unidentifiedCount=%v, want 1", payload["unidentifiedCount"])
	}
	for _, row := range byLabel["Website"] {
		if row["isEarly"] != true || row["value"] != drafts[previewID("_"+quid["web"])] || row["triageCategory"] != "answer" {
			t.Fatalf("phase 1 Website lost its saved draft: %v", row)
		}
	}
	for _, row := range byLabel[storyLabel] {
		if row["isEarly"] != true || row["value"] != drafts[previewID("_"+quid["story"])] {
			t.Fatalf("phase 1 narrative lost its saved draft: %v", row)
		}
	}
	for _, row := range byLabel["How did you hear about Render?"] {
		if row["editable"] != false || row["triageCategory"] != "review" || row["manualReason"] == "" {
			t.Fatalf("phase 1 source question misrepresented: %v", row)
		}
	}
	for _, label := range []string{"Name", "Email", "Phone", "Location", "LinkedIn"} {
		if byLabel[label][0]["triageCategory"] != "profile" {
			t.Fatalf("phase 1 %q not profile-handled: %v", label, byLabel[label][0])
		}
	}
	for _, row := range byLabel["Pronouns"] {
		if row["isEarly"] != true || row["triageCategory"] != "profile" {
			t.Fatalf("phase 1 Pronouns should be the single early profile row: %v", row)
		}
	}
	for _, row := range byLabel["Work authorization status"] {
		if row["isEarly"] != true || row["triageCategory"] != "employer" || row["kind"] != "text" {
			t.Fatalf("phase 1 employer-side wording change should show the early row once: %v", row)
		}
	}
	for _, row := range byLabel["Extra note"] {
		if row["triageCategory"] != "answer" {
			t.Fatalf("phase 1 kind-drifted applicant question merged or mislabeled: %v", row)
		}
	}
	for _, label := range []string{"Race", "Veteran Status"} {
		if byLabel[label][0]["triageCategory"] != "employer" {
			t.Fatalf("phase 1 %q not employer guidance: %v", label, byLabel[label][0])
		}
	}
	for _, label := range answerLabels(byLabel) {
		switch label {
		case "Website", storyLabel, "Other information", "Extra note":
		default:
			t.Fatalf("phase 1 unexpected writing task %q", label)
		}
	}
	if previous, _ := payload["previous"].([]any); len(previous) != 0 {
		t.Fatalf("phase 1 unexpected preserved writing: %v", previous)
	}

	// Phase 2: the managed browser is open at the live observation. The same
	// unified rows appear; saved drafts keep their early identity. Active is
	// derived from a live session, so attach the test fake session.
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	a.browsers.states[1] = browserRunState{Destination: testAshbyPosting, LatestID: liveRowID}
	if a.browsers.sessions == nil {
		a.browsers.sessions = map[int64]readOnlySession{}
	}
	a.browsers.sessions[1] = &fakeBrowser{url: testAshbyPosting + "/application"}
	a.browsers.mu.Unlock()
	payload = writingQuestions(t, a, 1)
	byLabel = unifiedByLabel(payload)
	for label, want := range map[string]int{"Website": 1, storyLabel: 1, "How did you hear about Render?": 1, "Pronouns": 1, "Work authorization status": 1, "Other information": 2, "Extra note": 2} {
		if len(byLabel[label]) != want {
			t.Fatalf("phase 2 %q rows=%d, want %d", label, len(byLabel[label]), want)
		}
	}
	if got, _ := payload["unidentifiedCount"].(float64); got != 1 {
		t.Fatalf("phase 2 unidentifiedCount=%v, want 1", payload["unidentifiedCount"])
	}
	for _, row := range byLabel["Pronouns"] {
		if row["isEarly"] != false || row["triageCategory"] != "profile" {
			t.Fatalf("phase 2 Pronouns should be the live profile row once: %v", row)
		}
	}
	for _, row := range byLabel["Work authorization status"] {
		if row["isEarly"] != false || row["triageCategory"] != "employer" || row["kind"] != "radio" {
			t.Fatalf("phase 2 employer-side wording change should show the live row once: %v", row)
		}
	}
	for _, row := range byLabel["Website"] {
		if row["isEarly"] != true || row["value"] != drafts[previewID("_"+quid["web"])] {
			t.Fatalf("phase 2 Website draft identity lost: %v", row)
		}
	}
	for _, row := range byLabel["Other information"] {
		if row["triageCategory"] != "answer" {
			t.Fatalf("phase 2 same-label rows silently merged or mislabeled: %v", row)
		}
	}
	if got := reviewCount(t, a); got != 0 {
		t.Fatalf("questions read created %d submission reviews", got)
	}

	// Phase 3: a live answer saved for a merged pair wins the single row.
	liveKeyA := ""
	for _, field := range liveFields {
		if field.Label == "Other information" && strings.HasSuffix(field.ID, quid["infoA"]) {
			liveKeyA = questionKey(testAshbyPosting, "ashby", field)
		}
	}
	if liveKeyA == "" {
		t.Fatal("live infoA field missing")
	}
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+liveKeyA+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Live-side note"}`); rec.Code != http.StatusOK {
		t.Fatalf("live save status=%d body=%s", rec.Code, rec.Body.String())
	}
	byLabel = unifiedByLabel(writingQuestions(t, a, 1))
	if rows := byLabel["Other information"]; len(rows) != 2 {
		t.Fatalf("phase 3 Other information rows=%d, want 2", len(rows))
	} else {
		answered := 0
		for _, row := range rows {
			if row["value"] == "Live-side note" {
				answered++
				if row["isEarly"] != false {
					t.Fatalf("phase 3 live answer shown with early identity: %v", row)
				}
			}
		}
		if answered != 1 {
			t.Fatalf("phase 3 live answer missing: %v", rows)
		}
	}
}

func TestManualLetterCreationAndValidation(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Manual letter"}); err != nil {
		t.Fatal(err)
	}
	bad := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":"short"}`)
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("short letter: %d %s", bad.Code, bad.Body.String())
	}
	body := "Dear hiring team, this manually written letter belongs to this application."
	good := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":`+jsonString(body)+`}`)
	if good.Code != http.StatusOK {
		t.Fatalf("create: %d %s", good.Code, good.Body.String())
	}
	letter, err := a.latestCoverLetter(1)
	if err != nil || letter.Body != body || letter.Provider != "manual" || letter.MasterResumeID != 0 || letter.RoleResumeID != 0 {
		t.Fatalf("letter: %+v %v", letter, err)
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("draft count %d %v", count, err)
	}
}

func TestLetterSelectionPersistsAndStaysWithinJob(t *testing.T) {
 a, dir := testWorkspace(t)
 if err := a.saveJob(Job{URL:testAshbyPosting, Host:"jobs.ashbyhq.com",Title:"Selection"}); err != nil { t.Fatal(err) }
 body1 := "Dear hiring team, this first saved letter is the one I choose to use."
 body2 := "Dear hiring team, this second saved letter is not the selected one."
 for _, body := range []string{body1, body2} {
  r := postWritingJSON(t,a,http.MethodPost,"/api/jobs/1/letters/manual",`{"body":`+jsonString(body)+`}`)
  if r.Code!=http.StatusOK {t.Fatalf("create: %d %s",r.Code,r.Body.String())}
 }
 if r:=postWritingJSON(t,a,http.MethodPost,"/api/jobs/1/letters/select",`{"draftId":1}`);r.Code!=http.StatusOK {t.Fatalf("select: %d %s",r.Code,r.Body.String())}
 if r:=postWritingJSON(t,a,http.MethodPost,"/api/jobs/1/letters/select",`{"draftId":999}`);r.Code!=http.StatusConflict {t.Fatalf("invalid: %d",r.Code)}
 stale,err:=a.coverLetterByID(1,2);if err!=nil{t.Fatal(err)}
 launches:=0
 a.browsers.factory=func(context.Context,string)(readOnlySession,error){launches++;return nil,errors.New("browser must not start for a stale letter-only request")}
 request := `{"destination":`+jsonString(testAshbyPosting)+`,"include_letter":true,"draft_id":"2","body_hash":`+jsonString(stale.ContentHash())+`,"mode":"file"}`
 rejected:=postWritingJSON(t,a,http.MethodPost,"/api/jobs/1/continue",request)
 if rejected.Code!=http.StatusOK || !strings.Contains(rejected.Body.String(),"selected letter changed") { t.Fatalf("stale selection accepted: %d %s",rejected.Code,rejected.Body.String()) }
 if launches!=0 { t.Fatalf("stale letter-only request launched the browser %d times",launches) }
 if a.browsers.state(1).Active { t.Fatal("stale letter-only request left a browser session behind") }
 if err:=a.db.Close();err!=nil {t.Fatal(err)}
 db,err:=storage.OpenDB(filepath.Join(dir,"jobs.db"));if err!=nil{t.Fatal(err)};defer db.Close()
 reopened:=newTestApp(t,db,filepath.Join(dir,"resumes"))
 letter,err:=reopened.selectedCoverLetter(1);if err!=nil||letter.ID!=1||letter.Body!=body1{t.Fatalf("selection after reopen: %+v %v",letter,err)}
 snapshot,err:=reopened.buildContinuation(1); if err!=nil { t.Fatal(err) }; payload,_:=json.Marshal(snapshot); if !strings.Contains(string(payload), `"id":1`) { t.Fatalf("continuation: %s",payload) }
}
