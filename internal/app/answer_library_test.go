package app

import (
	"html"
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

const answerLibraryStory = "Describe a production incident you resolved for a customer"

func newAnswerLibraryFixture(t *testing.T) (*App, string, Job, schema.Field, string) {
	t.Helper()
	a, dir := testWorkspace(t)
	job := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Technical Support Engineer", Company: "Fixture Systems"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:incident-story", Label: answerLibraryStory, Kind: "textarea"}
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	return a, dir, job, field, questionKey(job.URL, "ashby", field)
}

func saveLibrarySourceAnswer(t *testing.T, a *App, job Job, key, answer string, revision int64) {
	t.Helper()
	result := postApp(newMux(a), "/jobs/"+strconv.FormatInt(job.ID, 10)+"/answers/save", url.Values{
		"destination": {job.URL}, "question_key": {key}, "revision": {strconv.FormatInt(revision, 10)}, "answer": {answer},
	}, "", "")
	if result.Code != http.StatusSeeOther {
		t.Fatalf("save source answer: status=%d body=%s", result.Code, result.Body.String())
	}
}

func promoteLibrarySource(t *testing.T, a *App, jobID int64, key string, revision int64) *httptest.ResponseRecorder {
	t.Helper()
	return postApp(newMux(a), "/jobs/"+strconv.FormatInt(jobID, 10)+"/answers/library", url.Values{
		"question_key": {key}, "revision": {strconv.FormatInt(revision, 10)},
	}, "", "")
}

func getLibraryPage(t *testing.T, a *App) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/answer-library", nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	return rec
}

func TestAnswerLibraryPromotionSurvivesAppliedStateAndRestart(t *testing.T) {
	a, dir, job, _, key := newAnswerLibraryFixture(t)
	answer := "At 02:14, I reviewed the Linux logs and database diagnostics.\nI isolated the failing service, restored it, and documented the follow-up.\nI shared the findings with the customer. — résumé notes <script>alert('escaped')</script>"
	saveLibrarySourceAnswer(t, a, job, key, answer, 0)

	token := reviewedSubmissionToken(t, a, job.ID)
	completed := postApp(newMux(a), "/jobs/1/submit", url.Values{"confirm": {"yes"}, "submission_token": {token}}, "", "")
	if completed.Code != http.StatusSeeOther {
		t.Fatalf("record synthetic completion: %d %s", completed.Code, completed.Body.String())
	}
	if got, err := a.jobByID(job.ID); err != nil || got.Stage != StageApplied || got.SubmissionCount != 1 {
		t.Fatalf("source job not Applied: %+v %v", got, err)
	}

	promoted := promoteLibrarySource(t, a, job.ID, key, 1)
	if promoted.Code != http.StatusSeeOther || promoted.Header().Get("Location") != "/answer-library?notice=saved" {
		t.Fatalf("promotion redirect: %d %s", promoted.Code, promoted.Header().Get("Location"))
	}
	if strings.Contains(promoted.Header().Get("Location"), answer) {
		t.Fatal("private answer leaked through redirect URL")
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("library entries: %d %v", len(entries), err)
	}
	entry := entries[0]
	if entry.OriginalQuestion != answerLibraryStory || entry.Answer != answer || entry.SourceEmployer != "Fixture Systems" ||
		entry.SourceJobTitle != "Technical Support Engineer" || entry.SourceJobID != job.ID || entry.SourceRevision != 1 ||
		entry.SourceInspectionID == 0 || entry.SourceInspectionFingerprint == "" || entry.SourceFieldID != "browser:incident-story" || entry.SourceFieldKind != "textarea" {
		t.Fatalf("incomplete source snapshot: %+v", entry)
	}
	var saved string
	var answerRevision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=? AND question_key=?`, job.ID, key).Scan(&saved, &answerRevision); err != nil || saved != answer || answerRevision != 1 {
		t.Fatalf("promotion changed source answer: %q revision=%d err=%v", saved, answerRevision, err)
	}
	locked := postApp(newMux(a), "/jobs/1/answers/save", url.Values{"destination": {job.URL}, "question_key": {key}, "revision": {"1"}, "answer": {"must remain blocked"}}, "", "")
	if locked.Code != http.StatusConflict {
		t.Fatalf("completed source answer became editable: %d", locked.Code)
	}
	pageReq := httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil)
	pageReq.Host = "localhost"
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, pageReq)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Save to Answer Library") ||
		!strings.Contains(page.Body.String(), "Reuse at this employer") || strings.Contains(page.Body.String(), `action="/jobs/1/answers/save"`) {
		t.Fatalf("completed Prepare does not expose a read-only promotion: %d", page.Code)
	}

	libraryPage := getLibraryPage(t, a)
	if libraryPage.Code != http.StatusOK || !strings.Contains(libraryPage.Body.String(), "Fixture Systems") ||
		!strings.Contains(libraryPage.Body.String(), "Technical Support Engineer") || !strings.Contains(libraryPage.Body.String(), "&lt;script&gt;") {
		t.Fatalf("library rendering/provenance/escaping: %d %s", libraryPage.Code, libraryPage.Body.String())
	}
	entryReq := httptest.NewRequest(http.MethodGet, "http://localhost/answer-library/"+strconv.FormatInt(entry.ID, 10), nil)
	entryReq.Host = "localhost"
	entryPage := httptest.NewRecorder()
	newMux(a).ServeHTTP(entryPage, entryReq)
	if entryPage.Code != http.StatusOK || !strings.Contains(entryPage.Body.String(), html.EscapeString(answer)) {
		t.Fatalf("library entry retrieval failed: %d", entryPage.Code)
	}

	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	restartedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, restartedDB, filepath.Join(dir, "resumes"))
	restartedPage := getLibraryPage(t, restarted)
	if restartedPage.Code != http.StatusOK || !strings.Contains(restartedPage.Body.String(), html.EscapeString(answer)) {
		t.Fatal("Answer Library snapshot did not survive restart")
	}
}

func TestAnswerLibraryRejectsMissingStaleAndIneligibleSources(t *testing.T) {
	a, _, job, field, key := newAnswerLibraryFixture(t)
	if got := promoteLibrarySource(t, a, job.ID, key, 1); got.Code != http.StatusConflict {
		t.Fatalf("unsaved source answer accepted: %d", got.Code)
	}
	if got := promoteLibrarySource(t, a, job.ID, strings.Repeat("a", 64), 1); got.Code != http.StatusConflict {
		t.Fatalf("nonexistent source identity accepted: %d", got.Code)
	}
	saveLibrarySourceAnswer(t, a, job, key, "A deliberately synthetic incident story.", 0)
	if got := promoteLibrarySource(t, a, job.ID, key, 2); got.Code != http.StatusConflict {
		t.Fatalf("stale source revision accepted: %d", got.Code)
	}
	if _, err := a.db.Exec(`UPDATE application_answers SET revision=2 WHERE job_id=? AND question_key=?`, job.ID, key); err != nil {
		t.Fatal(err)
	}
	if got := promoteLibrarySource(t, a, job.ID, key, 1); got.Code != http.StatusConflict {
		t.Fatalf("stale revision after source changed accepted: %d", got.Code)
	}

	for _, unsupported := range []schema.Field{
		{ID: "browser:authorization-story", Label: "Describe your work authorization status and history", Kind: "textarea", Sensitive: true},
		{ID: "browser:salary-story", Label: "Describe your expected salary and compensation", Kind: "textarea"},
		{ID: "browser:choice-story", Label: "Describe an example from your work experience", Kind: "select", Options: []string{"One", "Two"}},
		{ID: "browser:field_3", Label: "Describe a technical incident", Kind: "textarea"},
	} {
		if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: job.URL + "/application", Fields: []schema.Field{unsupported}}); err != nil {
			t.Fatal(err)
		}
		unsupportedKey := questionKey(job.URL, "ashby", unsupported)
		if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(?,?,?,?,?,?,1) ON CONFLICT(job_id,destination,provider,question_key) DO UPDATE SET label=excluded.label,answer=excluded.answer,revision=excluded.revision`, job.ID, job.URL, "ashby", unsupportedKey, unsupported.Label, "synthetic answer content"); err != nil {
			t.Fatal(err)
		}
		if got := promoteLibrarySource(t, a, job.ID, unsupportedKey, 1); got.Code != http.StatusUnprocessableEntity {
			t.Errorf("ineligible field %q accepted: %d", unsupported.Label, got.Code)
		}
	}
	// A plausible label and saved row are not enough when the inspection's
	// exact field identity differs from the submitted key.
	identityKey := strings.Repeat("f", 64)
	if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(?,?,?,?,?,?,1)`, job.ID, job.URL, "ashby", identityKey, field.Label, "synthetic identity mismatch"); err != nil {
		t.Fatal(err)
	}
	if got := promoteLibrarySource(t, a, job.ID, identityKey, 1); got.Code != http.StatusConflict {
		t.Fatalf("unmatched question identity accepted: %d", got.Code)
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected content entered library: %d %v", len(entries), err)
	}
}

func TestAnswerLibraryDeduplicatesBySavedRevisionAndKeepsLifecycleIndependent(t *testing.T) {
	a, dir, job, _, key := newAnswerLibraryFixture(t)
	firstAnswer := "First version with a line break.\nI found the issue and explained the fix to the customer."
	saveLibrarySourceAnswer(t, a, job, key, firstAnswer, 0)
	if got := promoteLibrarySource(t, a, job.ID, key, 1); got.Code != http.StatusSeeOther {
		t.Fatalf("first promotion: %d %s", got.Code, got.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("first entry count: %d %v", len(entries), err)
	}
	edit := "Independently edited library copy — résumé.\nThe original remains untouched."
	edited := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(entries[0].ID, 10)+"/edit", url.Values{"revision": {"1"}, "answer": {edit}}, "", "")
	if edited.Code != http.StatusSeeOther {
		t.Fatalf("edit library copy: %d %s", edited.Code, edited.Body.String())
	}
	if duplicate := promoteLibrarySource(t, a, job.ID, key, 1); duplicate.Code != http.StatusSeeOther || duplicate.Header().Get("Location") != "/answer-library?notice=existing" {
		t.Fatalf("repeat promotion: %d %s", duplicate.Code, duplicate.Header().Get("Location"))
	}
	entries, _ = a.answerLibraryEntries()
	if len(entries) != 1 || entries[0].Answer != edit || entries[0].Revision != 2 {
		t.Fatalf("repeat promotion overwrote curated copy: %+v", entries)
	}
	var source string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=? AND question_key=?`, job.ID, key).Scan(&source); err != nil || source != firstAnswer {
		t.Fatalf("library edit changed original application answer: %q %v", source, err)
	}
	staleEdit := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(entries[0].ID, 10)+"/edit", url.Values{"revision": {"1"}, "answer": {"stale library edit"}}, "", "")
	if staleEdit.Code != http.StatusConflict {
		t.Fatalf("stale library revision accepted: %d", staleEdit.Code)
	}

	secondAnswer := "A newer saved revision.\nI tested the repair and shared the follow-up."
	saveLibrarySourceAnswer(t, a, job, key, secondAnswer, 1)
	if got := promoteLibrarySource(t, a, job.ID, key, 2); got.Code != http.StatusSeeOther {
		t.Fatalf("second revision promotion: %d %s", got.Code, got.Body.String())
	}
	entries, _ = a.answerLibraryEntries()
	if len(entries) != 2 {
		t.Fatalf("revised source did not get an independent snapshot: %d", len(entries))
	}
	var secondID int64
	for _, entry := range entries {
		if entry.SourceRevision == 2 {
			secondID = entry.ID
			if entry.Answer != secondAnswer {
				t.Fatalf("new revision snapshot = %q", entry.Answer)
			}
		}
	}
	if secondID == 0 {
		t.Fatal("new source revision is missing")
	}
	missingConfirmation := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(secondID, 10)+"/delete", url.Values{}, "", "")
	if missingConfirmation.Code != http.StatusBadRequest {
		t.Fatalf("unconfirmed library deletion accepted: %d", missingConfirmation.Code)
	}
	deleted := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(secondID, 10)+"/delete", url.Values{"confirm": {"yes"}}, "", "")
	if deleted.Code != http.StatusSeeOther {
		t.Fatalf("delete library copy: %d %s", deleted.Code, deleted.Body.String())
	}
	var revision int64
	if err := a.db.QueryRow(`SELECT revision FROM application_answers WHERE job_id=? AND question_key=?`, job.ID, key).Scan(&revision); err != nil || revision != 2 {
		t.Fatalf("library deletion changed application answer history: rev=%d err=%v", revision, err)
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	reopened := newTestApp(t, reopenedDB, filepath.Join(dir, "resumes"))
	entries, err = reopened.answerLibraryEntries()
	if err != nil || len(entries) != 1 || entries[0].Answer != edit {
		t.Fatalf("library deletion did not persist through restart: %+v %v", entries, err)
	}
	if _, err := reopened.deleteJobs([]int64{job.ID}, false); err != nil {
		t.Fatal(err)
	}
	entries, err = reopened.answerLibraryEntries()
	if err != nil || len(entries) != 1 || entries[0].Answer != edit || entries[0].SourceJobID != job.ID {
		t.Fatalf("job deletion removed or altered curated snapshot: %+v %v", entries, err)
	}
}

func TestAnswerLibraryRoutesKeepLocalHostOriginAndRedirectBoundaries(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	saveLibrarySourceAnswer(t, a, job, key, "Synthetic support story.", 0)
	mux := newMux(a)
	for _, tc := range []struct {
		method, path, host, origin string
	}{
		{http.MethodGet, "/answer-library", "evil.test", ""},
		{http.MethodPost, "/jobs/1/answers/library", "localhost", "https://evil.test"},
		{http.MethodPost, "/answer-library/1/edit", "localhost", "https://evil.test"},
		{http.MethodPost, "/answer-library/1/delete", "localhost", "https://evil.test"},
	} {
		req := httptest.NewRequest(tc.method, "http://localhost"+tc.path, strings.NewReader("confirm=yes"))
		req.Host = tc.host
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s accepted host/origin %q/%q: %d", tc.method, tc.path, tc.host, tc.origin, rec.Code)
		}
	}

	browserRedirect := promoteLibrarySource(t, a, job.ID, key, 1)
	if browserRedirect.Code != http.StatusSeeOther || browserRedirect.Header().Get("Location") != "/answer-library?notice=saved" {
		t.Fatalf("browser redirect: %d %s", browserRedirect.Code, browserRedirect.Header().Get("Location"))
	}
	desktopRedirect := postApp(newDesktopMux(a), "/jobs/1/answers/library", url.Values{"question_key": {key}, "revision": {"1"}}, "wails://localhost", "1")
	if desktopRedirect.Code != http.StatusOK || !strings.Contains(desktopRedirect.Body.String(), "window.location.replace") ||
		!strings.Contains(desktopRedirect.Body.String(), "/answer-library?notice=existing") {
		t.Fatalf("Wails-compatible library redirect: %d %s", desktopRedirect.Code, desktopRedirect.Body.String())
	}
}

func TestAnswerLibraryEligibilityIsConservative(t *testing.T) {
	tests := []struct {
		name         string
		field        schema.Field
		unique, want bool
	}{
		{"narrative prompt", schema.Field{ID: "browser:q", Label: "Describe a challenging project you led", Kind: "textarea"}, true, true},
		{"generic experience", schema.Field{ID: "browser:q", Label: "What is your experience?", Kind: "textarea"}, true, false},
		{"salary", schema.Field{ID: "browser:q", Label: "Describe your expected salary and compensation", Kind: "textarea"}, true, false},
		{"authorization", schema.Field{ID: "browser:q", Label: "Tell us about work authorization status", Kind: "textarea"}, true, false},
		{"sensitive flag", schema.Field{ID: "browser:q", Label: "Describe a technical incident", Kind: "textarea", Sensitive: true}, true, false},
		{"select", schema.Field{ID: "browser:q", Label: "Describe a project you led", Kind: "select"}, true, false},
		{"ambiguous identity", schema.Field{ID: "browser:q", Label: "Describe a project you led", Kind: "textarea"}, false, false},
		{"positional identity", schema.Field{ID: "browser:field_3", Label: "Describe a project you led", Kind: "textarea"}, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := answerLibraryEligible(tc.field, tc.unique); got != tc.want {
				t.Fatalf("eligibility=%v want %v", got, tc.want)
			}
		})
	}
}

func TestAnswerLibraryErrorsDoNotEchoSubmittedContent(t *testing.T) {
	a, _, job, _, key := newAnswerLibraryFixture(t)
	private := "private answer that must never appear in an error or URL"
	response := postApp(newMux(a), "/jobs/1/answers/library", url.Values{"question_key": {key}, "revision": {"1"}, "answer": {private}}, "", "")
	if response.Code != http.StatusConflict || strings.Contains(response.Body.String(), private) || strings.Contains(response.Header().Get("Location"), private) {
		t.Fatalf("error leaked submitted content: %d %s %s", response.Code, response.Body.String(), response.Header().Get("Location"))
	}
	if got := promoteLibrarySource(t, a, job.ID, key, 1); got.Code != http.StatusConflict {
		t.Fatalf("unsaved answer status changed: %d", got.Code)
	}
}
