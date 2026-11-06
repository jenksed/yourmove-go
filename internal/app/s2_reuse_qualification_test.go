package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func s2Questions(t *testing.T, a *App, jobID int64) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/"+strconv.FormatInt(jobID, 10)+"/questions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("questions %d %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestS2EarlyHelpEligibleExposed(t *testing.T) {
	a, _ := testWorkspace(t)
	_, _ = seedWritingJob(t, a)
	payload := s2Questions(t, a, 1)
	early, _ := payload["early"].([]any)
	if len(early) == 0 {
		t.Fatal("no early questions")
	}
	found := false
	for _, item := range early {
		q := item.(map[string]any)
		if q["sourceQuestionId"] == "application:motivation" {
			found = true
			if q["helpEligible"] != true {
				t.Fatalf("early narrative helpEligible not exposed: %v", q)
			}
			if q["libraryEligible"] != false {
				t.Fatalf("unsaved early should not be library eligible: %v", q)
			}
		}
	}
	if !found {
		t.Fatal("early motivation question missing")
	}
}

func TestS2AnswerBrowseAndSearch(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	answer := "At 02:14, I reviewed the Linux logs and database diagnostics. I isolated the failing service, restored it, and documented the follow-up."
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)
	if got := promoteLibrarySource(t, a, job.ID, key, 1); got.Code != http.StatusSeeOther {
		t.Fatalf("promote: %d %s", got.Code, got.Body.String())
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/answer-library?limit=20", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("browse: %d %s", rec.Code, rec.Body.String())
	}
	var browse struct {
		Entries      []map[string]any `json:"entries"`
		LibraryEmpty bool             `json:"libraryEmpty"`
		HasMore      bool             `json:"hasMore"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &browse); err != nil {
		t.Fatal(err)
	}
	if browse.LibraryEmpty || len(browse.Entries) != 1 {
		t.Fatalf("browse entries: %+v", browse)
	}
	// Search delegates to ranker and labels bounded results.
	search := httptest.NewRecorder()
	newMux(a).ServeHTTP(search, httptest.NewRequest(http.MethodGet, "http://localhost/api/answer-library?query=Linux+logs", nil))
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `"bounded":true`) {
		t.Fatalf("search bounded: %d %s", search.Code, search.Body.String())
	}
	empty := httptest.NewRecorder()
	newMux(a).ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "http://localhost/api/answer-library?query=zzzznope", nil))
	if empty.Code != http.StatusOK || strings.Contains(empty.Body.String(), `"libraryEmpty":true`) {
		t.Fatalf("no-match must not claim empty library: %s", empty.Body.String())
	}
}

func TestS2PromotionJSONNegotiation(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	saveLibrarySourceAnswer(t, a, job, key, "A qualifying Linux incident narrative with enough wording to pass eligibility checks.", 0)
	form := url.Values{"question_key": {key}, "revision": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/answers/library", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"saved":true`) {
		t.Fatalf("promotion JSON: %d %s", rec.Code, rec.Body.String())
	}
	again := httptest.NewRecorder()
	newMux(a).ServeHTTP(again, func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/answers/library", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Accept", "application/json")
		return r
	}())
	var second struct {
		Existing bool `json:"existing"`
	}
	if err := json.Unmarshal(again.Body.Bytes(), &second); err != nil || !second.Existing {
		t.Fatalf("duplicate promotion should report existing: %s %v", again.Body.String(), err)
	}
}

func TestS2EarlyPromotionCreatesEarlyKind(t *testing.T) {
	a, _ := testWorkspace(t)
	_, ids := seedWritingJob(t, a)
	payload := s2Questions(t, a, 1)
	var key, fingerprint string
	for _, item := range payload["early"].([]any) {
		q := item.(map[string]any)
		if q["sourceQuestionId"] == ids["question"] {
			key, _ = q["key"].(string)
			fingerprint, _ = q["discoveryFingerprint"].(string)
		}
	}
	if key == "" {
		t.Fatal("early key missing")
	}
	save := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+key+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Early synthetic motivation with enough narrative wording to qualify for curation.","early":true,"discovery_source":"preview","discovery_fingerprint":"`+fingerprint+`","source_question_id":"application:motivation"}`)
	if save.Code != http.StatusOK {
		t.Fatalf("early save: %d %s", save.Code, save.Body.String())
	}
	form := url.Values{"question_key": {key}, "revision": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/answers/early-library", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("early promotion: %d %s", rec.Code, rec.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 || entries[0].SourceKind != "early" {
		t.Fatalf("early entry kind: %+v %v", entries, err)
	}
	// Live and early identities cannot collide: same revision on same job with
	// live provider must be a distinct row.
	if entries[0].SourceProvider != "early:preview" || entries[0].SourceInspectionID != 0 {
		t.Fatalf("early provider namespace: %+v", entries[0])
	}
}

func TestS2AnswerOriginPersistPreserveAndConflict(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	answer := "At 02:14, I reviewed the Linux logs and database diagnostics. I isolated the failing service, restored it, and documented the follow-up for the customer."
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)
	if got := promoteLibrarySource(t, a, job.ID, key, 1); got.Code != http.StatusSeeOther {
		t.Fatalf("promote: %d %s", got.Code, got.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatal("entry missing")
	}
	entry := entries[0]
	// Destination job shares the same employer scope but a distinct posting for
	// exact-reuse isolation; the related narrative is reused via the library.
	dest := Job{URL: "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000001", Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Other Fixture"}
	if err := a.saveJob(dest); err != nil {
		t.Fatal(err)
	}
	dest, _ = a.jobByID(2)
	field := schema.Field{ID: "browser:challenge", Label: "Tell us about a challenging Linux incident you investigated", Kind: "textarea"}
	if _, err := a.saveInspection(dest.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: dest.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	destKey := questionKey(dest.URL, "ashby", field)
	// Save with provenance: current question eligible, source revision matches.
	save := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/answers",
		`{"question_key":"`+destKey+`","revision":0,"destination":"`+dest.URL+`","answer":"Adapted incident wording for the new employer.","origin_entry_id":`+strconv.FormatInt(entry.ID, 10)+`,"origin_revision":1}`)
	if save.Code != http.StatusOK {
		t.Fatalf("save with origin: %d %s", save.Code, save.Body.String())
	}
	var stored string
	if err := a.db.QueryRow(`SELECT reuse_source_json FROM application_answers WHERE job_id=2`).Scan(&stored); err != nil || !strings.Contains(stored, strconv.FormatInt(entry.ID, 10)) {
		t.Fatalf("origin not persisted: %q %v", stored, err)
	}
	// Legacy edit without origin fields preserves the stored origin.
	legacy := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/answers",
		`{"question_key":"`+destKey+`","revision":1,"destination":"`+dest.URL+`","answer":"Edited wording without touching origin."}`)
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy edit: %d %s", legacy.Code, legacy.Body.String())
	}
	var preserved string
	if err := a.db.QueryRow(`SELECT reuse_source_json FROM application_answers WHERE job_id=2`).Scan(&preserved); err != nil || preserved != stored {
		t.Fatalf("legacy edit should preserve origin: %q vs %q", preserved, stored)
	}
	// Stale source revision refuses the provenance-bearing save and retains draft.
	stale := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/answers",
		`{"question_key":"`+destKey+`","revision":2,"destination":"`+dest.URL+`","answer":"Another adaptation.","origin_entry_id":`+strconv.FormatInt(entry.ID, 10)+`,"origin_revision":999}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale origin accepted: %d %s", stale.Code, stale.Body.String())
	}
	var kept string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=2`).Scan(&kept); err != nil || kept != "Edited wording without touching origin." {
		t.Fatalf("stale origin overwrote body: %q %v", kept, err)
	}
}

func TestS2ReconciliationCarriesEarlyOrigin(t *testing.T) {
	a, _ := testWorkspace(t)
	job, publicResult := earlyGreenhouseFixture(t, a)
	if _, err := a.saveInspection(job.ID, publicResult); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fields: []schema.Field{
		{ID: "browser:story", Label: "Describe a time you improved a system", Kind: "textarea", Required: true, Group: "rendered form"},
	}}); err != nil {
		t.Fatal(err)
	}
	profile, _ := a.candidateProfile()
	preparedLive, err := a.prepareQuestions(job, profile)
	if err != nil || len(preparedLive.Questions) != 1 {
		t.Fatalf("live preparation: %+v %v", preparedLive.Questions, err)
	}
	liveKey := preparedLive.Questions[0].Key
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+liveKey+`","revision":0,"destination":"`+earlyGreenhousePosting+`","answer":"Live baseline before accepting the early draft."}`); rec.Code != http.StatusOK {
		t.Fatalf("live save: %d %s", rec.Code, rec.Body.String())
	}
	promoteReq := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/1/answers/library", strings.NewReader(url.Values{"question_key": {liveKey}, "revision": {"1"}}.Encode()))
	promoteReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	promoteReq.Header.Set("Accept", "application/json")
	promoteRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(promoteRec, promoteReq)
	if promoteRec.Code != http.StatusOK {
		t.Fatalf("promote: %d %s", promoteRec.Code, promoteRec.Body.String())
	}
	var promoted struct {
		EntryID int64 `json:"entryId"`
	}
	if err := json.Unmarshal(promoteRec.Body.Bytes(), &promoted); err != nil || promoted.EntryID == 0 {
		t.Fatalf("promotion id: %s %v", promoteRec.Body.String(), err)
	}
	q := earlyQuestionByID(t, a, job, "application:story", "")
	earlyAnswer := "I reviewed the alert history and documented the incident response."
	earlySave := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers",
		`{"question_key":"`+q.Key+`","revision":0,"destination":"`+earlyGreenhousePosting+`","answer":`+jsonString(earlyAnswer)+`,"early":true,"discovery_source":"public_api","discovery_fingerprint":"`+q.DiscoveryFingerprint+`","source_question_id":"application:story","origin_entry_id":`+strconv.FormatInt(promoted.EntryID, 10)+`,"origin_revision":1}`)
	if earlySave.Code != http.StatusOK {
		t.Fatalf("early save with origin: %d %s", earlySave.Code, earlySave.Body.String())
	}
	preparation, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1?early_question="+q.Key, nil))
	if err != nil {
		t.Fatal(err)
	}
	var prepared earlyApplicationQuestion
	for _, candidate := range preparation.Questions {
		if candidate.SourceQuestionID == "application:story" {
			prepared = candidate
		}
	}
	if !prepared.Reconciliation.CanAccept {
		t.Fatalf("no reconciliation candidate: %+v", prepared.Reconciliation)
	}
	accept := postApp(newMux(a), "/jobs/1/answers/early-accept", url.Values{
		"early_answer_id":             {strconv.FormatInt(prepared.EarlyAnswerID, 10)},
		"early_revision":              {strconv.FormatInt(prepared.Revision, 10)},
		"destination":                 {job.URL},
		"live_inspection_fingerprint": {prepared.Reconciliation.LiveInspectionFingerprint},
		"live_question_key":           {prepared.Reconciliation.LiveQuestionKey},
		"live_answer_revision":        {strconv.FormatInt(prepared.Reconciliation.LiveAnswerRevision, 10)},
	}, "", "")
	if accept.Code != http.StatusSeeOther {
		t.Fatalf("accept: %d %s", accept.Code, accept.Body.String())
	}
	var liveOrigin string
	if err := a.db.QueryRow(`SELECT COALESCE(reuse_source_json,'') FROM application_answers WHERE job_id=1`).Scan(&liveOrigin); err != nil {
		t.Fatal(err)
	}
	var carried answerReuseOrigin
	if err := json.Unmarshal([]byte(liveOrigin), &carried); err != nil || carried.LibraryEntryID != promoted.EntryID || carried.DestinationRev != 2 {
		t.Fatalf("live answer did not carry rebuilt early origin: %q %v", liveOrigin, err)
	}
}

func TestS2LetterHistoryIncludesSelectedBeyond30(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Selection"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 32; i++ {
		body := "Dear hiring team, this is synthetic saved letter number " + strconv.Itoa(i+1) + " with enough wording to pass validation."
		if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":`+jsonString(body)+`}`); rec.Code != http.StatusOK {
			t.Fatalf("create %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/select", `{"draftId":1}`); rec.Code != http.StatusOK {
		t.Fatalf("select: %d %s", rec.Code, rec.Body.String())
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/letters", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("letters: %d %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Drafts []map[string]any `json:"drafts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range payload.Drafts {
		if d["id"] == float64(1) {
			found = true
		}
	}
	if !found {
		t.Fatalf("selected letter 1 missing from drafts: %v", payload.Drafts)
	}
	history := httptest.NewRecorder()
	newMux(a).ServeHTTP(history, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/letters/history?limit=20", nil))
	var page struct {
		Entries    []map[string]any `json:"entries"`
		HasMore    bool             `json:"hasMore"`
		SelectedId float64          `json:"selectedId"`
	}
	if err := json.Unmarshal(history.Body.Bytes(), &page); err != nil || page.SelectedId != 1 || !page.HasMore {
		t.Fatalf("history page: %s %v", history.Body.String(), err)
	}
}

func TestS2LetterCatalogAndPriorReuse(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support A", Company: "Fixture A"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000002", Host: "jobs.ashbyhq.com", Title: "Support B", Company: "Fixture B"}); err != nil {
		t.Fatal(err)
	}
	bodyA := "Dear Fixture A team, I investigated a production incident, restored service, and documented the follow-up for customers."
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":`+jsonString(bodyA)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("create A: %d %s", rec.Code, rec.Body.String())
	}
	catalog := httptest.NewRecorder()
	newMux(a).ServeHTTP(catalog, httptest.NewRequest(http.MethodGet, "http://localhost/api/cover-letters?limit=20", nil))
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), "Fixture A") {
		t.Fatalf("catalog: %d %s", catalog.Code, catalog.Body.String())
	}
	detail := httptest.NewRecorder()
	newMux(a).ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "http://localhost/api/cover-letters/1", nil))
	var opened struct {
		Letter map[string]any `json:"letter"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &opened); err != nil || opened.Letter["body"] != bodyA {
		t.Fatalf("catalog detail: %s %v", detail.Body.String(), err)
	}
	// Preview performs no writes: selection for job 2 stays empty.
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters WHERE job_id=2`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("preview wrote: %d %v", count, err)
	}
	reuse := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/letters/manual",
		`{"body":"Dear Fixture B team, I adapted the prior incident wording for this new opportunity with sufficient length.","prior_letter_id":1,"prior_job_id":1,"prior_body_hash":"`+opened.Letter["contentHash"].(string)+`"}`)
	if reuse.Code != http.StatusOK {
		t.Fatalf("prior reuse: %d %s", reuse.Code, reuse.Body.String())
	}
	var newBody, origin string
	if err := a.db.QueryRow(`SELECT body,reuse_source_json FROM cover_letters WHERE job_id=2`).Scan(&newBody, &origin); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(newBody, "Fixture B") || !strings.Contains(origin, `"source_letter_id":1`) {
		t.Fatalf("new letter origin: %q %q", newBody, origin)
	}
	var oldBody string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=1`).Scan(&oldBody); err != nil || oldBody != bodyA {
		t.Fatalf("original changed: %q %v", oldBody, err)
	}
	var provider string
	var masterID, roleID int64
	if err := a.db.QueryRow(`SELECT provider,COALESCE(master_resume_id,0),COALESCE(role_resume_id,0) FROM cover_letters WHERE job_id=2`).Scan(&provider, &masterID, &roleID); err != nil || provider != "manual" || masterID != 0 || roleID != 0 {
		t.Fatalf("new letter binding: %q %d %d %v", provider, masterID, roleID, err)
	}
}

func TestS2LetterSaveConcurrency(t *testing.T) {
	seedLetterJobForS2(t, newTestAppForS2(t))
}

func seedLetterJobForS2(t *testing.T, a *App) {
	t.Helper()
	seedLetterJob(t, a)
	save := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":"Dear hiring team, this manually written letter belongs to this application and is long enough."}`)
	if save.Code != http.StatusOK {
		t.Fatalf("setup: %d %s", save.Code, save.Body.String())
	}
	var created struct {
		DraftId int64 `json:"draftId"`
	}
	if err := json.Unmarshal(save.Body.Bytes(), &created); err != nil || created.DraftId == 0 {
		t.Fatalf("draft id: %s %v", save.Body.String(), err)
	}
	path := "/api/jobs/1/letters/" + strconv.FormatInt(created.DraftId, 10)
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
	var draft struct {
		Body string `json:"body"`
		Hash string `json:"contentHash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	bad := postWritingJSON(t, a, http.MethodPost, path, `{"body":"Dear hiring team, this concurrent edit should be refused because the hash is stale and long enough.","expected_hash":"deadbeef"}`)
	if bad.Code != http.StatusConflict {
		t.Fatalf("stale hash accepted: %d %s", bad.Code, bad.Body.String())
	}
	var kept string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=?`, created.DraftId).Scan(&kept); err != nil || kept != draft.Body {
		t.Fatalf("concurrent edit overwrote: %q %v", kept, err)
	}
	good := postWritingJSON(t, a, http.MethodPost, path, `{"body":"Dear hiring team, this concurrent edit uses the correct hash and is long enough to save.","expected_hash":"`+draft.Hash+`"}`)
	if good.Code != http.StatusOK {
		t.Fatalf("correct hash refused: %d %s", good.Code, good.Body.String())
	}
}

func newTestAppForS2(t *testing.T) *App {
	t.Helper()
	a, _ := testWorkspace(t)
	return a
}
