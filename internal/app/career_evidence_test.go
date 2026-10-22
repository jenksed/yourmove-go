package app

import (
	"database/sql"
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func careerEvidenceForm(revision int64, content careerEvidenceContent) url.Values {
	outcomeState := content.OutcomeState
	if outcomeState == "" {
		outcomeState = "not_recorded"
	}
	return url.Values{
		"revision": {strconv.FormatInt(revision, 10)}, "title": {content.Title},
		"situation": {content.Situation}, "actions": {content.Actions},
		"employer_context": {content.EmployerContext}, "role_context": {content.RoleContext},
		"technologies": {content.Technologies}, "outcome": {content.Outcome}, "outcome_state": {outcomeState},
		"period": {content.Period}, "source_notes": {content.SourceNotes},
	}
}

func postCareerEvidence(mux http.Handler, path string, values url.Values) *httptest.ResponseRecorder {
	return postApp(mux, path, values, "", "")
}

func TestCareerEvidenceMigrationAndRepeatedStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE jobs(id INTEGER PRIMARY KEY,url TEXT NOT NULL,host TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`INSERT INTO jobs(id,url,host) VALUES(1,'https://example.test/job','example.test')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	for startup := 0; startup < 3; startup++ {
		db, err := storage.OpenDB(path)
		if err != nil {
			t.Fatalf("startup %d: %v", startup+1, err)
		}
		var tableCount, jobCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='career_evidence'`).Scan(&tableCount); err != nil || tableCount != 1 {
			t.Fatalf("career evidence migration on startup %d: count=%d err=%v", startup+1, tableCount, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE id=1 AND url='https://example.test/job'`).Scan(&jobCount); err != nil || jobCount != 1 {
			t.Fatalf("migration did not preserve the existing job on startup %d: count=%d err=%v", startup+1, jobCount, err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCareerEvidenceDraftReviewRevisionAndRestart(t *testing.T) {
	a, dir := testWorkspace(t)
	mux := newMux(a)
	created := postCareerEvidence(mux, "/career-evidence", url.Values{})
	if created.Code != http.StatusSeeOther || created.Header().Get("Location") != "/career-evidence/1?notice=created" {
		t.Fatalf("create manual draft: %d %s", created.Code, created.Header().Get("Location"))
	}
	page := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://localhost/career-evidence/1", nil)
	request.Host = "localhost"
	mux.ServeHTTP(page, request)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `action="/career-evidence/1/save"`) ||
		!strings.Contains(page.Body.String(), `name="situation"`) || !strings.Contains(page.Body.String(), `name="actions"`) ||
		!strings.Contains(page.Body.String(), "Mark as reviewed") || !strings.Contains(page.Body.String(), "Draft") {
		t.Fatalf("draft page is not a usable server-rendered form: %d %s", page.Code, page.Body.String())
	}

	incomplete := careerEvidenceContent{Title: "Database incident", Situation: "The service slowed down.", Technologies: "MySQL"}
	if saved := postCareerEvidence(mux, "/career-evidence/1/save", careerEvidenceForm(1, incomplete)); saved.Code != http.StatusSeeOther {
		t.Fatalf("incomplete draft could not be saved: %d %s", saved.Code, saved.Body.String())
	}
	values := careerEvidenceForm(2, incomplete)
	if review := postCareerEvidence(mux, "/career-evidence/1/review", values); review.Code != http.StatusBadRequest {
		t.Fatalf("review without applicant confirmation accepted: %d", review.Code)
	}
	values.Set("confirm_review", "yes")
	if review := postCareerEvidence(mux, "/career-evidence/1/review", values); review.Code != http.StatusUnprocessableEntity || !strings.Contains(review.Body.String(), incomplete.Title) {
		t.Fatalf("incomplete draft was marked reviewed: %d", review.Code)
	}
	evidence, err := careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "draft" || evidence.Revision != 2 || evidence.Title != incomplete.Title || evidence.OutcomeState != "not_recorded" {
		t.Fatalf("incomplete draft did not persist as draft: %+v %v", evidence, err)
	}

	content := careerEvidenceContent{
		Title: "MySQL performance investigation — résumé", Situation: "A customer-facing service slowed down.\nSeveral database operations became unusually slow.",
		Actions:         "Reviewed slow-query logs.\nUsed EXPLAIN to compare query plans and recommended indexing changes.",
		EmployerContext: "Synthetic Example", RoleContext: "Support engineer", Technologies: "Linux, MySQL, SSH",
		OutcomeState: "unknown", Period: "Approx. 2024", SourceNotes: "Synthetic notes only.",
	}
	if saved := postCareerEvidence(mux, "/career-evidence/1/save", careerEvidenceForm(2, content)); saved.Code != http.StatusSeeOther {
		t.Fatalf("complete draft save: %d %s", saved.Code, saved.Body.String())
	}
	evidence, err = careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "draft" || evidence.Revision != 3 || evidence.Outcome != "" || evidence.OutcomeState != "unknown" || evidence.Situation != content.Situation || evidence.Actions != content.Actions {
		t.Fatalf("unicode/multiline or unknown outcome did not persist: %+v %v", evidence, err)
	}
	recordedWithoutText := content
	recordedWithoutText.OutcomeState = "recorded"
	values = careerEvidenceForm(3, recordedWithoutText)
	values.Set("confirm_review", "yes")
	if review := postCareerEvidence(mux, "/career-evidence/1/review", values); review.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty recorded outcome was accepted as reviewed: %d", review.Code)
	}
	values = careerEvidenceForm(3, content)
	values.Set("confirm_review", "yes")
	if reviewed := postCareerEvidence(mux, "/career-evidence/1/review", values); reviewed.Code != http.StatusSeeOther {
		t.Fatalf("explicit review failed: %d %s", reviewed.Code, reviewed.Body.String())
	}
	evidence, err = careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "reviewed" || evidence.Revision != 4 {
		t.Fatalf("review state was not persisted: %+v %v", evidence, err)
	}

	edited := content
	edited.Actions += "\nRecorded the follow-up for the team."
	if saved := postCareerEvidence(mux, "/career-evidence/1/save", careerEvidenceForm(4, edited)); saved.Code != http.StatusSeeOther {
		t.Fatalf("reviewed content edit failed: %d %s", saved.Code, saved.Body.String())
	}
	evidence, err = careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "draft" || evidence.Revision != 5 || evidence.Actions != edited.Actions {
		t.Fatalf("substantive edit did not invalidate review: %+v %v", evidence, err)
	}
	stale := careerEvidenceForm(4, content)
	if conflict := postCareerEvidence(mux, "/career-evidence/1/save", stale); conflict.Code != http.StatusConflict {
		t.Fatalf("stale content edit accepted: %d", conflict.Code)
	}
	values = careerEvidenceForm(5, edited)
	values.Set("confirm_review", "yes")
	if reviewed := postCareerEvidence(mux, "/career-evidence/1/review", values); reviewed.Code != http.StatusSeeOther {
		t.Fatalf("second review failed: %d", reviewed.Code)
	}
	staleDelete := postCareerEvidence(mux, "/career-evidence/1/delete", url.Values{"confirm": {"yes"}, "revision": {"5"}})
	if staleDelete.Code != http.StatusConflict {
		t.Fatalf("stale evidence delete accepted: %d", staleDelete.Code)
	}

	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	restartedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, restartedDB, filepath.Join(dir, "resumes"))
	evidence, err = careerEvidenceByID(restarted.db, 1)
	if err != nil || evidence.Status != "reviewed" || evidence.Revision != 6 || evidence.Actions != edited.Actions {
		t.Fatalf("reviewed evidence did not survive restart: %+v %v", evidence, err)
	}
	deleted := postCareerEvidence(newMux(restarted), "/career-evidence/1/delete", url.Values{"confirm": {"yes"}, "revision": {"6"}})
	if deleted.Code != http.StatusSeeOther {
		t.Fatalf("unchanged evidence delete failed: %d %s", deleted.Code, deleted.Body.String())
	}
	if _, err := careerEvidenceByID(restarted.db, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted evidence still exists: %v", err)
	}
}

func TestCareerEvidenceFromAnswerLibraryKeepsIndependentSnapshot(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	originalAnswer := "I reviewed the synthetic logs.\nI used EXPLAIN to inspect query plans."
	saveLibrarySourceAnswer(t, a, job, key, originalAnswer, 0)
	if promoted := promoteLibrarySource(t, a, job.ID, key, 1); promoted.Code != http.StatusSeeOther {
		t.Fatalf("promote synthetic source answer: %d", promoted.Code)
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("source Answer Library entry: %d %v", len(entries), err)
	}
	createPath := "/answer-library/" + strconv.FormatInt(entries[0].ID, 10) + "/career-evidence"
	first := postCareerEvidence(newMux(a), createPath, url.Values{})
	if first.Code != http.StatusSeeOther || !strings.HasPrefix(first.Header().Get("Location"), "/career-evidence/") {
		t.Fatalf("create evidence from writing: %d %s", first.Code, first.Header().Get("Location"))
	}
	repeated := postCareerEvidence(newMux(a), createPath, url.Values{})
	if repeated.Code != http.StatusSeeOther || repeated.Header().Get("Location") != first.Header().Get("Location") {
		t.Fatalf("repeated source action duplicated the draft: %d %s", repeated.Code, repeated.Header().Get("Location"))
	}
	evidence, err := careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "draft" || evidence.Title != "" || evidence.Situation != "" || evidence.Actions != "" ||
		evidence.SourceLibraryEntryID != entries[0].ID || evidence.SourceQuestion != answerLibraryStory || evidence.SourceWriting != originalAnswer || evidence.SourceEmployer != job.Company {
		t.Fatalf("source draft interpreted or lost its source snapshot: %+v %v", evidence, err)
	}
	var libraryAnswer string
	var libraryRevision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM answer_library WHERE id=?`, entries[0].ID).Scan(&libraryAnswer, &libraryRevision); err != nil || libraryAnswer != originalAnswer || libraryRevision != 1 {
		t.Fatalf("creating evidence changed the original library entry: %q revision=%d err=%v", libraryAnswer, libraryRevision, err)
	}
	page := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://localhost/career-evidence/1", nil)
	request.Host = "localhost"
	newMux(a).ServeHTTP(page, request)
	if !strings.Contains(page.Body.String(), html.EscapeString(originalAnswer)) || !strings.Contains(page.Body.String(), html.EscapeString(answerLibraryStory)) {
		t.Fatalf("source wording was not inspectable: %s", page.Body.String())
	}
	deleteSource := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(entries[0].ID, 10)+"/delete", url.Values{"confirm": {"yes"}, "revision": {"1"}}, "", "")
	if deleteSource.Code != http.StatusSeeOther {
		t.Fatalf("delete source library row: %d %s", deleteSource.Code, deleteSource.Body.String())
	}
	if _, err := careerEvidenceByID(a.db, evidence.ID); err != nil {
		t.Fatalf("source deletion removed career evidence: %v", err)
	}
	var applicationAnswer string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=? AND question_key=?`, job.ID, key).Scan(&applicationAnswer); err != nil || applicationAnswer != originalAnswer {
		t.Fatalf("source lifecycle changed application history: %q %v", applicationAnswer, err)
	}
	deletionID, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	confirmed := postCareerEvidence(newMux(a), "/career-evidence/"+strconv.FormatInt(deletionID, 10)+"/delete", url.Values{"confirm": {"yes"}, "revision": {"1"}})
	if confirmed.Code != http.StatusSeeOther {
		t.Fatalf("delete independent draft: %d", confirmed.Code)
	}
	if _, err := careerEvidenceByID(a.db, evidence.ID); err != nil {
		t.Fatalf("evidence sourced from the library was removed with a separate deletion: %v", err)
	}
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=? AND question_key=?`, job.ID, key).Scan(&applicationAnswer); err != nil || applicationAnswer != originalAnswer {
		t.Fatalf("evidence deletion changed application history: %q %v", applicationAnswer, err)
	}
	newWriting := "A later library entry that reused a deleted local ID."
	newSourceTime := "2099-01-01T00:00:00Z"
	_, err = a.db.Exec(`INSERT INTO answer_library(id,original_question,answer,source_employer,source_job_title,source_host,
		source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
		source_inspection_fingerprint,source_field_id,source_field_kind,question_terms,answer_terms,search_index_version,created_at,updated_at,revision)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`, entries[0].ID, "Describe a new synthetic production project", newWriting,
		job.Company, job.Title, job.Host, job.ID, job.URL, "ashby", key, 2, entries[0].SourceInspectionID,
		entries[0].SourceInspectionFingerprint, entries[0].SourceFieldID, entries[0].SourceFieldKind, "", "", 1, newSourceTime, newSourceTime)
	if err != nil {
		t.Fatal(err)
	}
	newEvidenceID, err := a.createCareerEvidenceFromLibrary(entries[0].ID)
	if err != nil || newEvidenceID == evidence.ID {
		t.Fatalf("reused Answer Library ID collided with the old source snapshot: id=%d old=%d err=%v", newEvidenceID, evidence.ID, err)
	}
	newEvidence, err := careerEvidenceByID(a.db, newEvidenceID)
	if err != nil || newEvidence.SourceWriting != newWriting {
		t.Fatalf("new source wording was not stored independently: %+v %v", newEvidence, err)
	}
	independentID, err := a.createCareerEvidenceDraft()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.deleteJobs([]int64{job.ID}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := careerEvidenceByID(a.db, independentID); err != nil {
		t.Fatalf("job deletion removed career evidence: %v", err)
	}
	if _, err := careerEvidenceByID(a.db, evidence.ID); err != nil {
		t.Fatalf("job deletion removed source snapshot evidence: %v", err)
	}
	if _, err := careerEvidenceByID(a.db, newEvidenceID); err != nil {
		t.Fatalf("job deletion removed evidence from a later source: %v", err)
	}
}

func TestCareerEvidenceLocalBoundariesAndEscaping(t *testing.T) {
	a, _ := testWorkspace(t)
	mux := newMux(a)
	for _, tc := range []struct{ method, path, host, origin string }{
		{http.MethodGet, "/career-evidence", "evil.test", ""},
		{http.MethodPost, "/career-evidence", "localhost", "https://evil.test"},
		{http.MethodPost, "/career-evidence/1/save", "localhost", "https://evil.test"},
	} {
		req := httptest.NewRequest(tc.method, "http://localhost"+tc.path, strings.NewReader(""))
		req.Host = tc.host
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if tc.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("local request boundary accepted %s %s %q/%q: %d", tc.method, tc.path, tc.host, tc.origin, rec.Code)
		}
	}
	if created := postCareerEvidence(mux, "/career-evidence", url.Values{}); created.Code != http.StatusSeeOther {
		t.Fatal(created.Code)
	}
	content := careerEvidenceContent{Title: "<script>alert('synthetic')</script>", Situation: "First line\nSecond line — résumé", Actions: "Reviewed synthetic data."}
	if saved := postCareerEvidence(mux, "/career-evidence/1/save", careerEvidenceForm(1, content)); saved.Code != http.StatusSeeOther {
		t.Fatal(saved.Code, saved.Body.String())
	}
	page := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/career-evidence/1", nil)
	req.Host = "localhost"
	mux.ServeHTTP(page, req)
	body := page.Body.String()
	if page.Code != http.StatusOK || !strings.Contains(body, html.EscapeString(content.Title)) || strings.Contains(body, content.Title) ||
		!strings.Contains(body, html.EscapeString(content.Situation)) || !strings.Contains(body, `method="POST"`) || !strings.Contains(body, `name="confirm_review"`) {
		t.Fatalf("evidence page escaping or keyboard/no-JavaScript forms failed: %d %s", page.Code, body)
	}
}

func TestCareerEvidenceDesktopPostRedirects(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	saveLibrarySourceAnswer(t, a, job, key, "Synthetic source wording.", 0)
	if promoted := promoteLibrarySource(t, a, job.ID, key, 1); promoted.Code != http.StatusSeeOther {
		t.Fatal(promoted.Code)
	}
	response := postApp(newDesktopMux(a), "/career-evidence", url.Values{}, "wails://localhost", "1")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `window.location.replace("/career-evidence/1?notice=created")`) ||
		!safeDesktopRedirectTarget("/career-evidence/1?notice=reviewed") || !safeDesktopRedirectTarget("/career-evidence?notice=deleted") ||
		safeDesktopRedirectTarget("/career-evidence/1?notice=unknown") || safeDesktopRedirectTarget("/career-evidence/1?notice=created#external") ||
		safeDesktopRedirectTarget("/career-evidence/1?source=private") {
		t.Fatalf("Career Evidence Wails navigation or redirect allowlist failed: %d %s", response.Code, response.Body.String())
	}
	sourceRedirect := postApp(newDesktopMux(a), "/answer-library/1/career-evidence", url.Values{}, "wails://localhost", "1")
	if sourceRedirect.Code != http.StatusOK || !strings.Contains(sourceRedirect.Body.String(), `window.location.replace("/career-evidence/2?notice=source")`) {
		t.Fatalf("source draft Wails navigation: %d %s", sourceRedirect.Code, sourceRedirect.Body.String())
	}
}
