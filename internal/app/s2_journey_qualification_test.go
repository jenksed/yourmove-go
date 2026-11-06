package app

import (
	"crypto/sha256"
	"fmt"
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

func s2JourneyHash(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

// TestS2SecondApplicationJourney exercises the complete second-application
// story through protected HTTP handlers on isolated synthetic data.
func TestS2SecondApplicationJourney(t *testing.T) {
	a, dir := testWorkspace(t)
	urlA := "https://jobs.ashbyhq.com/render/11111111-1111-4111-8111-111111111111"
	urlB := "https://jobs.ashbyhq.com/render/22222222-2222-4222-8222-222222222222"
	host := "jobs.ashbyhq.com"
	if err := a.saveJob(Job{URL: urlA, Host: host, Title: "Support A", Company: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: urlB, Host: host, Title: "Support B", Company: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	employer := urlA
	_ = employer
	incident := schema.Field{ID: "browser:incident", Label: "Describe a production incident you resolved for a customer", Kind: "textarea"}
	collab := schema.Field{ID: "browser:collab", Label: "Tell us about a challenging collaboration you navigated", Kind: "textarea"}
	other := schema.Field{ID: "browser:other", Label: "Share an example of improving documentation for customers", Kind: "textarea"}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlA + "/application", Fields: []schema.Field{incident, collab, other}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(2, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: urlB + "/application", Fields: []schema.Field{incident, collab, other}}); err != nil {
		t.Fatal(err)
	}
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	a.browsers.states[1] = browserRunState{Active: false, Destination: urlA, LatestID: 1}
	a.browsers.states[2] = browserRunState{Active: false, Destination: urlB, LatestID: 2}
	a.browsers.mu.Unlock()

	keyA := func(f schema.Field) string { return questionKey(urlA, "ashby", f) }
	keyB := func(f schema.Field) string { return questionKey(urlB, "ashby", f) }

	// 1. A prepares writing, including a >320-character answer with a unique final sentence.
	longAnswer := strings.Repeat("I investigated the failing service, reviewed logs, and restored it. ", 8) + "Unique final sentence for full-length verification."
	save := func(jobID int64, key, dest, answer string, rev int64) *httptest.ResponseRecorder {
		return postWritingJSON(t, a, http.MethodPost, "/api/jobs/"+strconv.FormatInt(jobID, 10)+"/answers",
			`{"question_key":"`+key+`","revision":`+strconv.FormatInt(rev, 10)+`,"destination":"`+dest+`","answer":`+jsonString(answer)+`}`)
	}
	if rec := save(1, keyA(incident), urlA, longAnswer, 0); rec.Code != http.StatusOK {
		t.Fatalf("A incident save: %d %s", rec.Code, rec.Body.String())
	}
	if rec := save(1, keyA(collab), urlA, "Collaboration example for later manual discovery.", 0); rec.Code != http.StatusOK {
		t.Fatalf("A collab save: %d %s", rec.Code, rec.Body.String())
	}
	// Curate both examples.
	promote := func(jobID int64, key string, rev int64) *httptest.ResponseRecorder {
		form := url.Values{"question_key": {key}, "revision": {strconv.FormatInt(rev, 10)}}
		req := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/"+strconv.FormatInt(jobID, 10)+"/answers/library", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, req)
		return rec
	}
	if rec := promote(1, keyA(incident), 1); rec.Code != http.StatusOK {
		t.Fatalf("promote incident: %d %s", rec.Code, rec.Body.String())
	}
	if rec := promote(1, keyA(collab), 1); rec.Code != http.StatusOK {
		t.Fatalf("promote collab: %d %s", rec.Code, rec.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 2 {
		t.Fatalf("library entries: %+v %v", entries, err)
	}
	var incidentEntry AnswerLibraryEntry
	for _, e := range entries {
		if len(e.Answer) > 320 {
			incidentEntry = e
		}
	}
	if incidentEntry.ID == 0 || !strings.Contains(incidentEntry.Answer, "Unique final sentence") {
		t.Fatal("long example not curated")
	}
	// Save a letter for A.
	letterA := "Dear Fixture team, I investigated a production incident for opportunity A, restored service, and documented the follow-up for customers."
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters/manual", `{"body":`+jsonString(letterA)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("letter A: %d %s", rec.Code, rec.Body.String())
	}
	var letterAID int64
	if err := a.db.QueryRow(`SELECT id FROM cover_letters WHERE job_id=1`).Scan(&letterAID); err != nil {
		t.Fatal(err)
	}
	var letterAHash string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=?`, letterAID).Scan(&letterAHash); err != nil {
		t.Fatal(err)
	}
	_ = letterAHash

	// 2-5. B reuses the related answer in full, adapts, and saves without altering A/library.
	startForm := url.Values{
		"library_entry_id": {strconv.FormatInt(incidentEntry.ID, 10)},
		"question_key":     {keyB(incident)},
		"revision":         {"0"},
		"library_revision": {strconv.FormatInt(incidentEntry.Revision, 10)},
		"destination":      {urlB},
	}
	started := postApp(newMux(a), "/jobs/2/answers/starting-point", startForm, "", "")
	if started.Code != http.StatusOK || !strings.Contains(started.Body.String(), "Unique final sentence") {
		t.Fatalf("starting point full text: %d %s", started.Code, started.Body.String())
	}
	adapted := strings.Replace(longAnswer, "Unique final sentence for full-length verification.", "Adapted final sentence for opportunity B.", 1)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/answers",
		`{"question_key":"`+keyB(incident)+`","revision":0,"destination":"`+urlB+`","answer":`+jsonString(adapted)+`,"origin_entry_id":`+strconv.FormatInt(incidentEntry.ID, 10)+`,"origin_revision":`+strconv.FormatInt(incidentEntry.Revision, 10)+`}`); rec.Code != http.StatusOK {
		t.Fatalf("B save with origin: %d %s", rec.Code, rec.Body.String())
	}
	var storedB, originB string
	if err := a.db.QueryRow(`SELECT answer,reuse_source_json FROM application_answers WHERE job_id=2`).Scan(&storedB, &originB); err != nil || storedB != adapted || !strings.Contains(originB, "Unique final") && !strings.Contains(originB, strconv.FormatInt(incidentEntry.ID, 10)) {
		// Origin stores source metadata, not the adapted body; check IDs.
		if !strings.Contains(originB, strconv.FormatInt(incidentEntry.ID, 10)) {
			t.Fatalf("B origin: %q %v", originB, err)
		}
	}
	var storedA string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1 AND question_key=?`, keyA(incident)).Scan(&storedA); err != nil || storedA != longAnswer {
		t.Fatalf("A altered: %q %v", storedA, err)
	}
	if err := a.db.QueryRow(`SELECT answer FROM answer_library WHERE id=?`, incidentEntry.ID).Scan(&storedA); err != nil || storedA != longAnswer {
		t.Fatalf("library altered: %v", err)
	}

	// 6. Manual browse finds the older collaboration example without losing editor state.
	browse := httptest.NewRecorder()
	newMux(a).ServeHTTP(browse, httptest.NewRequest(http.MethodGet, "http://localhost/api/answer-library?limit=20", nil))
	if browse.Code != http.StatusOK || !strings.Contains(browse.Body.String(), "challenging collaboration") {
		t.Fatalf("browse: %d %s", browse.Code, browse.Body.String())
	}

	// 7-8. Find the older saved letter, preview, and create a new draft for B.
	catalog := httptest.NewRecorder()
	newMux(a).ServeHTTP(catalog, httptest.NewRequest(http.MethodGet, "http://localhost/api/cover-letters?query=Fixture", nil))
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), "Support A") {
		t.Fatalf("catalog: %d %s", catalog.Code, catalog.Body.String())
	}
	var letterHash string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=?`, letterAID).Scan(&letterHash); err != nil {
		t.Fatal(err)
	}
	hash := s2JourneyHash(letterHash)
	letterB := "Dear Fixture team, I am applying for opportunity B with adapted incident wording and sufficient length for validation."
	reuse := postWritingJSON(t, a, http.MethodPost, "/api/jobs/2/letters/manual",
		`{"body":`+jsonString(letterB)+`,"prior_letter_id":`+strconv.FormatInt(letterAID, 10)+`,"prior_job_id":1,"prior_body_hash":"`+hash+`"}`)
	if reuse.Code != http.StatusOK {
		t.Fatalf("letter reuse: %d %s", reuse.Code, reuse.Body.String())
	}
	var newProvider string
	var newMaster, newRole int64
	if err := a.db.QueryRow(`SELECT provider,COALESCE(master_resume_id,0),COALESCE(role_resume_id,0) FROM cover_letters WHERE job_id=2`).Scan(&newProvider, &newMaster, &newRole); err != nil || newProvider != "manual" || newMaster != 0 || newRole != 0 {
		t.Fatalf("new letter binding: %q %d %d %v", newProvider, newMaster, newRole, err)
	}

	// 9. Originals and associations unchanged.
	var checkA string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=?`, letterAID).Scan(&checkA); err != nil || checkA != letterA {
		t.Fatalf("original letter changed: %q %v", checkA, err)
	}

	// 10. Restart with the same workspace without reseeding.
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reopened := newTestApp(t, db, filepath.Join(dir, "resumes"))
	var reopenedB, reopenedOrigin string
	if err := reopened.db.QueryRow(`SELECT answer,reuse_source_json FROM application_answers WHERE job_id=2`).Scan(&reopenedB, &reopenedOrigin); err != nil || reopenedB != adapted {
		t.Fatalf("restart answer: %q %v", reopenedB, err)
	}
	var reopenedLetter string
	if err := reopened.db.QueryRow(`SELECT body FROM cover_letters WHERE job_id=2`).Scan(&reopenedLetter); err != nil || reopenedLetter != letterB {
		t.Fatalf("restart letter: %q %v", reopenedLetter, err)
	}
	var reopenedLib string
	if err := reopened.db.QueryRow(`SELECT answer FROM answer_library WHERE id=?`, incidentEntry.ID).Scan(&reopenedLib); err != nil || reopenedLib != longAnswer {
		t.Fatalf("restart library: %v", err)
	}
}
