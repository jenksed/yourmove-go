package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

const unsupportedPostingURL = "https://careers.example.test/jobs/operations-1"

func manualPayload(t *testing.T, a *App, jobID int64) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/"+strconv.FormatInt(jobID, 10)+"/questions", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("manual questions status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func postManualJSON(t *testing.T, a *App, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	return rec
}

func manualRowsByLabel(payload map[string]any) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	questions, _ := payload["questions"].([]any)
	for _, item := range questions {
		q := item.(map[string]any)
		label, _ := q["label"].(string)
		out[label] = append(out[label], q)
	}
	return out
}

func seedLibraryNarrative(t *testing.T, a *App) (Job, int64) {
	t.Helper()
	job := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Fixture Systems"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:motivation", Label: "Why do you want to join this team?", Kind: "textarea", Required: true}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	key := questionKey(job.URL, "ashby", field)
	saveLibrarySourceAnswer(t, a, job, key, "I enjoy resolving difficult technical problems for customers.", 0)
	if rec := promoteLibrarySource(t, a, job.ID, key, 1); rec.Code != http.StatusSeeOther {
		t.Fatalf("seed promotion: %d %s", rec.Code, rec.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("seed library: %+v %v", entries, err)
	}
	return job, entries[0].ID
}

func TestManualQuestionWorkflowOnUnsupportedPosting(t *testing.T) {
	a, dir := testWorkspace(t)
	if err := a.saveJob(Job{URL: unsupportedPostingURL, Host: "careers.example.test", Title: "Operations Lead"}); err != nil {
		t.Fatal(err)
	}
	_, libraryEntryID := seedLibraryNarrative(t, a)

	payload := manualPayload(t, a, 1)
	if payload["ownsPosting"] != false {
		t.Fatalf("unsupported posting reports ownsPosting=%v", payload["ownsPosting"])
	}
	if questions, _ := payload["questions"].([]any); len(questions) != 0 {
		t.Fatalf("unsupported posting shows questions before manual creation: %v", questions)
	}

	// Create a manual question matching an approved equivalent: the exact
	// library answer is offered for review, unsaved, with no Fill authority.
	rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", `{"question":"Why do you want to join this team?"}`)
	if rec.Code != 200 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Saved    bool           `json:"saved"`
		Question map[string]any `json:"question"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || !created.Saved {
		t.Fatalf("create response: %s %v", rec.Body.String(), err)
	}
	key, _ := created.Question["key"].(string)
	if !validManualQuestionKey(key) {
		t.Fatalf("manual key is not application-owned: %q", key)
	}
	if strings.HasPrefix(key, "browser:") || strings.HasPrefix(key, "preview:") {
		t.Fatalf("manual key fabricates an observed identity: %q", key)
	}

	payload = manualPayload(t, a, 1)
	rows := manualRowsByLabel(payload)
	whyRows := rows["Why do you want to join this team?"]
	if len(whyRows) != 1 {
		t.Fatalf("manual question missing from projection: %v", rows)
	}
	why := whyRows[0]
	for field, want := range map[string]any{"isManual": true, "editable": true, "triageCategory": "answer", "kind": "manual"} {
		if why[field] != want {
			t.Fatalf("manual row %s=%v, want %v (%v)", field, why[field], want, why)
		}
	}
	if why["value"] != "I enjoy resolving difficult technical problems for customers." || why["libraryAutoPrepared"] != true {
		t.Fatalf("exact library equivalent was not offered for review: %v", why)
	}

	// Save the reviewed answer with provenance, then keep preparing it.
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+key, `{"answer":"I enjoy resolving difficult technical problems for customers.","revision":1,"origin_entry_id":`+strconv.FormatInt(libraryEntryID, 10)+`,"origin_revision":1}`)
	if rec.Code != 200 {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload = manualPayload(t, a, 1)
	why = manualRowsByLabel(payload)["Why do you want to join this team?"][0]
	if why["revision"] != float64(2) || why["value"] != "I enjoy resolving difficult technical problems for customers." {
		t.Fatalf("manual save not reflected: %v", why)
	}
	if why["originRecorded"] != true {
		t.Fatalf("manual save lost starting-source provenance: %v", why)
	}

	// Stale writes never overwrite newer applicant work.
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+key, `{"answer":"Stale overwrite.","revision":0}`)
	if rec.Code != 409 {
		t.Fatalf("stale save status=%d, want 409", rec.Code)
	}

	// A second manual question without an equivalent stays fully preparable.
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", `{"question":"Describe the support tools you have used"}`)
	if rec.Code != 200 {
		t.Fatalf("second create status=%d body=%s", rec.Code, rec.Body.String())
	}
	var second struct {
		Question map[string]any `json:"question"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	toolsKey, _ := second.Question["key"].(string)
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+toolsKey, `{"answer":"I used the search console and the support inbox to resolve customer issues.","revision":1}`)
	if rec.Code != 200 {
		t.Fatalf("second save status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Editing the question text preserves exact employer wording control.
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+toolsKey, `{"question":"Describe the support tools you have used remotely","answer":"I used the search console and the support inbox to resolve customer issues.","revision":2}`)
	if rec.Code != 200 {
		t.Fatalf("question edit status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload = manualPayload(t, a, 1)
	rows = manualRowsByLabel(payload)
	if len(rows["Describe the support tools you have used remotely"]) != 1 || len(rows["Describe the support tools you have used"]) != 0 {
		t.Fatalf("question edit not reflected: %v", rows)
	}

	// Library starting points validate against the manual key and revision.
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+toolsKey+"/starting-point", `{"library_entry_id":`+strconv.FormatInt(libraryEntryID, 10)+`,"library_revision":1,"revision":3,"destination":"`+unsupportedPostingURL+`"}`)
	if rec.Code != 200 {
		t.Fatalf("starting point status=%d body=%s", rec.Code, rec.Body.String())
	}
	var point struct {
		Answer string `json:"answer"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &point); err != nil || point.Answer == "" {
		t.Fatalf("starting point answer: %s %v", rec.Body.String(), err)
	}
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+toolsKey+"/starting-point", `{"library_entry_id":`+strconv.FormatInt(libraryEntryID, 10)+`,"library_revision":999,"revision":3,"destination":"`+unsupportedPostingURL+`"}`)
	if rec.Code != 409 {
		t.Fatalf("stale library revision status=%d, want 409", rec.Code)
	}

	// Saving to the library honors the same eligibility rules without
	// duplicating exact equivalents.
	var libraryCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM answer_library`).Scan(&libraryCount); err != nil || libraryCount != 1 {
		t.Fatalf("library count=%d err=%v", libraryCount, err)
	}
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions/"+toolsKey, `{"answer":"I used the search console and the support inbox to resolve customer issues.","revision":3,"save_to_library":true}`)
	if rec.Code != 200 {
		t.Fatalf("library save status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM answer_library`).Scan(&libraryCount); err != nil || libraryCount != 2 {
		t.Fatalf("manual answer was not promoted: count=%d err=%v", libraryCount, err)
	}
	entries, err := a.answerLibraryEntries()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.OriginalQuestion == "Describe the support tools you have used remotely" && (entry.SourceKind != "manual" || entry.SourceFieldKind != "manual") {
			t.Fatalf("manual promotion lost provenance: %+v", entry)
		}
	}

	// Manual questions never enter transfer scope.
	scope := continuationScope(t, a, 1)
	for _, name := range []string{"pendingAnswers", "readyAnswers"} {
		items, _ := scope[name].([]any)
		for _, item := range items {
			if strings.HasPrefix(item.(map[string]any)["key"].(string), "manual-") {
				t.Fatalf("manual question entered Fill scope %s", name)
			}
		}
		if len(items) != 0 {
			t.Fatalf("unsupported posting has Fill scope %s: %v", name, items)
		}
	}

	// Restart on the same workspace preserves manual preparation.
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	db2, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	a2 := newTestApp(t, db2, filepath.Join(dir, "resumes"))
	payload = manualPayload(t, a2, 1)
	rows = manualRowsByLabel(payload)
	if len(rows["Why do you want to join this team?"]) != 1 || len(rows["Describe the support tools you have used remotely"]) != 1 {
		t.Fatalf("manual questions lost across restart: %v", rows)
	}
	if rows["Describe the support tools you have used remotely"][0]["value"] != "I used the search console and the support inbox to resolve customer issues." {
		t.Fatalf("manual answer lost across restart: %v", rows)
	}

	// Removing a manual question deletes only that row.
	a2.browsers.closeAll()
	if err := db2.Close(); err != nil {
		t.Fatal(err)
	}
	db3, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	a3 := newTestApp(t, db3, filepath.Join(dir, "resumes"))
	rec = postManualJSON(t, a3, http.MethodDelete, "/api/jobs/1/manual-questions/"+toolsKey, `{"revision":4}`)
	if rec.Code != 200 {
		t.Fatalf("remove status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload = manualPayload(t, a3, 1)
	if rows := manualRowsByLabel(payload); len(rows["Describe the support tools you have used remotely"]) != 0 || len(rows["Why do you want to join this team?"]) != 1 {
		t.Fatalf("remove damaged unrelated preparation: %v", rows)
	}
	if err := a3.db.QueryRow(`SELECT COUNT(*) FROM answer_library`).Scan(&libraryCount); err != nil || libraryCount != 2 {
		t.Fatalf("remove damaged the Answer Library: count=%d err=%v", libraryCount, err)
	}
	var answers int
	if err := a3.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=2`).Scan(&answers); err != nil || answers != 1 {
		t.Fatalf("remove damaged unrelated application answers: count=%d err=%v", answers, err)
	}
	rec = postManualJSON(t, a3, http.MethodDelete, "/api/jobs/1/manual-questions/"+toolsKey, `{"revision":4}`)
	if rec.Code != 409 {
		t.Fatalf("double remove status=%d, want 409", rec.Code)
	}
}

func TestManualQuestionsValidationAndSupportedCoexistence(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: unsupportedPostingURL, Host: "careers.example.test", Title: "Operations Lead"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{`{}`, `{"question":""}`, `{"question":"  "}`} {
		if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", body); rec.Code != 422 {
			t.Fatalf("invalid create %q status=%d, want 422", body, rec.Code)
		}
	}
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", `{"question":"`+strings.Repeat("x", 501)+`"}`); rec.Code != 422 {
		t.Fatalf("oversize create status=%d, want 422", rec.Code)
	}
	if rec := postManualJSON(t, a, http.MethodGet, "/api/jobs/1/manual-questions", `{}`); rec.Code != http.StatusSeeOther {
		t.Fatalf("create wrong method status=%d", rec.Code)
	}
	if rec := postManualJSON(t, a, "POST", "/api/jobs/1/manual-questions/browser:pattern", `{}`); rec.Code != 404 {
		t.Fatalf("fabricated key status=%d, want 404", rec.Code)
	}
	rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", `{"question":"Why do you want this role?"}`)
	if rec.Code != 200 {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}

	// A supported posting with incomplete capture keeps genuine observed
	// identities intact alongside manual preparation: same wording, two rows,
	// no silent merge or overwrite.
	liveField := schema.Field{ID: "browser:pattern", Label: "Preferred working pattern", Kind: "select", Required: true, Options: []string{"Remote", "Office", "Hybrid"}}
	if _, err := a.saveInspection(2, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{liveField}}); err != nil {
		t.Fatal(err)
	}
	rec = postManualJSON(t, a, http.MethodPost, "/api/jobs/2/manual-questions", `{"question":"Preferred working pattern"}`)
	if rec.Code != 200 {
		t.Fatalf("supported manual create status=%d body=%s", rec.Code, rec.Body.String())
	}
	payload := manualPayload(t, a, 2)
	rows := manualRowsByLabel(payload)
	patternRows := rows["Preferred working pattern"]
	if len(patternRows) != 2 {
		t.Fatalf("same wording must stay two distinct rows: %v", patternRows)
	}
	var sawLive, sawManual bool
	for _, row := range patternRows {
		switch {
		case row["isManual"] == true && row["editable"] == true:
			sawManual = true
		case row["isManual"] != true && row["editable"] == true && row["kind"] == "select":
			sawLive = true
		default:
			t.Fatalf("unexpected pattern row: %v", row)
		}
	}
	if !sawLive || !sawManual {
		t.Fatalf("live/manual rows misrepresented: live=%v manual=%v", sawLive, sawManual)
	}
	scope := continuationScope(t, a, 2)
	for _, name := range []string{"pendingAnswers", "readyAnswers"} {
		items, _ := scope[name].([]any)
		if len(items) != 0 {
			t.Fatalf("unsaved scope %s: %v", name, items)
		}
	}

	// Applied applications refuse manual writes.
	if err := a.saveJob(Job{URL: "https://careers.example.test/jobs/closed-1", Host: "careers.example.test", Title: "Closed"}); err != nil {
		t.Fatal(err)
	}
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmissionReview(3, token, 0, ""); err != nil {
		t.Fatal(err)
	}
	completed := postApp(newMux(a), "/jobs/3/submit", url.Values{"confirm": {"yes"}, "submission_token": {token}}, "", "")
	if completed.Code != http.StatusSeeOther {
		t.Fatalf("record completion: %d %s", completed.Code, completed.Body.String())
	}
	if rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/3/manual-questions", `{"question":"Why this role?"}`); rec.Code != 409 {
		t.Fatalf("applied create status=%d, want 409", rec.Code)
	}
}

func TestManualQuestionDeletionRemovesOnlyManualRows(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: unsupportedPostingURL, Host: "careers.example.test"}); err != nil {
		t.Fatal(err)
	}
	rec := postManualJSON(t, a, http.MethodPost, "/api/jobs/1/manual-questions", `{"question":"Why do you want this role?"}`)
	if rec.Code != 200 {
		t.Fatalf("create status=%d", rec.Code)
	}
	var created struct {
		Question map[string]any `json:"question"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	key, _ := created.Question["key"].(string)
	var before int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM manual_application_questions`).Scan(&before); err != nil || before != 1 {
		t.Fatalf("manual row missing: %d %v", before, err)
	}
	if _, err := a.deleteJobs([]int64{1}, false); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM manual_application_questions`).Scan(&after); err != nil || after != 0 {
		t.Fatalf("manual row survived job deletion: %d %v", after, err)
	}
	if _, err := a.manualQuestionByKey(Job{ID: 1, URL: unsupportedPostingURL}, key); err != sql.ErrNoRows {
		t.Fatalf("deleted manual question still readable: %v", err)
	}
}
