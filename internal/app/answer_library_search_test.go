package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func answerLibraryDoc(id int64, question, answer string) answerLibrarySearchDocument {
	return answerLibrarySearchDocument{
		EntryID: id, OriginalQuestion: question,
		questionTerms: answerLibraryTermSet(answerLibraryIndexTerms(question), ""),
		answerTerms:   answerLibraryTermSet(answerLibraryIndexTerms(answer), ""),
	}
}

func TestAnswerLibraryRetrievalUsesRelatedQuestionWording(t *testing.T) {
	docs := []answerLibrarySearchDocument{
		answerLibraryDoc(10, "Describe a technical problem you resolved for a customer", "I investigated a challenging Linux production incident by reviewing service logs and database diagnostics, then explained the fix to the customer."),
		answerLibraryDoc(11, "How do you keep stakeholders informed about project budgets", "I share concise updates about timelines, costs, and decisions."),
		answerLibraryDoc(12, "Tell us about your experience", "A broad description of my career and experience."),
	}
	ranked := rankAnswerLibraryDocuments("Tell us about a challenging Linux incident you investigated", docs, answerLibraryRelatedLimit)
	if len(ranked) != 1 || ranked[0].document.EntryID != 10 {
		t.Fatalf("related technical writing was not surfaced: %+v", ranked)
	}
	if got := rankAnswerLibraryDocuments("Tell us about your experience", docs, answerLibraryRelatedLimit); len(got) != 0 {
		t.Fatalf("generic experience wording should not establish relevance: %+v", got)
	}
	if got := rankAnswerLibraryDocuments("Negotiate enterprise pricing strategy", docs, answerLibraryRelatedLimit); len(got) != 0 {
		t.Fatalf("unrelated writing should not be suggested: %+v", got)
	}
}

func TestAnswerLibraryQuestionOverlapOutweighsLongAnswerOverlap(t *testing.T) {
	docs := []answerLibrarySearchDocument{
		answerLibraryDoc(1, "Linux production incident response", "A short note."),
		answerLibraryDoc(2, "Customer escalation", strings.Repeat("Linux production incident response ", 500)),
	}
	ranked := rankAnswerLibraryDocuments("Linux production incident response customer escalation", docs, answerLibraryRelatedLimit)
	if len(ranked) != 2 || ranked[0].document.EntryID != 1 {
		t.Fatalf("long answer tokens overwhelmed original-question overlap: %+v", ranked)
	}
}

func TestAnswerLibraryRankingIsStableAndBounded(t *testing.T) {
	docs := []answerLibrarySearchDocument{
		answerLibraryDoc(9, "Describe a Linux incident you resolved", "Linux logs."),
		answerLibraryDoc(3, "Describe a Linux incident you resolved", "Linux logs."),
		answerLibraryDoc(7, "Describe a Linux incident you resolved", "Linux logs."),
		answerLibraryDoc(5, "Describe a Linux incident you resolved", "Linux logs."),
	}
	ranked := rankAnswerLibraryDocuments("Describe the Linux incident", docs, 40)
	if len(ranked) != answerLibrarySearchPageLimit {
		t.Fatalf("explicit search result count=%d want %d", len(ranked), answerLibrarySearchPageLimit)
	}
	if ranked[0].document.EntryID != 3 || ranked[1].document.EntryID != 5 || ranked[2].document.EntryID != 7 {
		t.Fatalf("tie break is not stable by entry ID: %+v", ranked)
	}
	if got := rankAnswerLibraryDocuments("Describe the Linux incident", docs, answerLibraryRelatedLimit); len(got) != answerLibraryRelatedLimit {
		t.Fatalf("workbench did not honor its three-result bound: %d", len(got))
	}
	for id := int64(20); id < 100; id++ {
		docs = append(docs, answerLibraryDoc(id, "Describe a Linux incident you resolved", "Linux logs."))
	}
	if got := rankAnswerLibraryDocuments("Describe the Linux incident", docs, 100); len(got) != answerLibrarySearchPageLimit {
		t.Fatalf("library search did not honor its page bound: %d", len(got))
	}
}

func TestAnswerLibraryUnicodeNormalizationAndPhraseOverlap(t *testing.T) {
	doc := answerLibraryDoc(1, "Describe the résumé review process", "I checked the application carefully.")
	if words := answerLibraryWords("Résumé — naïve café"); strings.Join(words, " ") != "résumé naïve café" {
		t.Fatalf("Unicode tokens changed: %q", words)
	}
	if got := answerLibraryPhraseMatches("How did you handle résumé review?", doc.OriginalQuestion); got == 0 {
		t.Fatalf("informative adjacent phrase was not recognized: %d", got)
	}
}

func TestAnswerLibrarySearchAndStartingPointAreReadOnlyUntilSave(t *testing.T) {
	a, _, sourceJob, _, sourceKey := newAnswerLibraryFixture(t)
	answer := "I investigated a challenging Linux incident using logs and database diagnostics.\nI explained the fix to the customer."
	saveLibrarySourceAnswer(t, a, sourceJob, sourceKey, answer, 0)
	if got := promoteLibrarySource(t, a, sourceJob.ID, sourceKey, 1); got.Code != http.StatusSeeOther {
		t.Fatalf("promotion failed: %d %s", got.Code, got.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("promoted entries=%d err=%v", len(entries), err)
	}

	result := postApp(newMux(a), "/answer-library/search", url.Values{"query": {"Linux logs customer database diagnostics"}}, "", "")
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), answerLibraryStory) ||
		!strings.Contains(result.Body.String(), "Fixture Systems") || !strings.Contains(result.Body.String(), "Open full example") {
		t.Fatalf("explicit library search did not retain context: %d %s", result.Code, result.Body.String())
	}
	if result.Header().Get("Location") != "" {
		t.Fatalf("search query was exposed in redirect URL: %q", result.Header().Get("Location"))
	}
	desktopSearch := postApp(newDesktopMux(a), "/answer-library/search", url.Values{"query": {"Linux logs customer database diagnostics"}}, "wails://localhost", "1")
	if desktopSearch.Code != http.StatusOK || desktopSearch.Header().Get("Location") != "" || !strings.Contains(desktopSearch.Body.String(), "Fixture Systems") {
		t.Fatalf("Wails did not render the local search response: %d %s %s", desktopSearch.Code, desktopSearch.Header().Get("Location"), desktopSearch.Body.String())
	}

	destination := Job{URL: "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000001", Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Other Fixture"}
	if err := a.saveJob(destination); err != nil {
		t.Fatal(err)
	}
	destination, err = a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:challenge", Label: "Tell us about a challenging Linux incident you investigated", Kind: "textarea"}
	if _, err := a.saveInspection(destination.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: destination.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	key := questionKey(destination.URL, "ashby", field)
	saveLibrarySourceAnswer(t, a, destination, key, "Existing saved response", 0)
	startingPointForm := url.Values{
		"library_entry_id": {strconv.FormatInt(entries[0].ID, 10)}, "question_key": {key}, "revision": {"1"},
	}
	startingPoint := postApp(newMux(a), "/jobs/2/answers/starting-point", startingPointForm, "", "")
	if startingPoint.Code != http.StatusOK || !strings.Contains(startingPoint.Body.String(), "challenging Linux incident") {
		t.Fatalf("starting point could not be read: %d %s", startingPoint.Code, startingPoint.Body.String())
	}
	stale := url.Values{"library_entry_id": {strconv.FormatInt(entries[0].ID, 10)}, "question_key": {key}, "revision": {"0"}}
	if response := postApp(newMux(a), "/jobs/2/answers/starting-point", stale, "", ""); response.Code != http.StatusConflict {
		t.Fatalf("stale question revision accepted as a starting point: %d", response.Code)
	}
	foreign := url.Values{"library_entry_id": {strconv.FormatInt(entries[0].ID, 10)}, "question_key": {strings.Repeat("0", 64)}, "revision": {"1"}}
	if response := postApp(newMux(a), "/jobs/2/answers/starting-point", foreign, "", ""); response.Code != http.StatusConflict {
		t.Fatalf("different question identity accepted as a starting point: %d", response.Code)
	}
	missing := url.Values{"library_entry_id": {"99999"}, "question_key": {key}, "revision": {"1"}}
	if response := postApp(newMux(a), "/jobs/2/answers/starting-point", missing, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("missing library source accepted: %d", response.Code)
	}
	desktopStartingPoint := postApp(newDesktopMux(a), "/jobs/2/answers/starting-point", startingPointForm, "wails://localhost", "1")
	if desktopStartingPoint.Code != http.StatusOK || !strings.Contains(desktopStartingPoint.Header().Get("Content-Type"), "application/json") || desktopStartingPoint.Header().Get("Location") != "" {
		t.Fatalf("Wails JSON starting-point response was redirected: %d %s %s", desktopStartingPoint.Code, desktopStartingPoint.Header().Get("Content-Type"), desktopStartingPoint.Header().Get("Location"))
	}
	var destinationAnswer string
	var destinationRevision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=? AND question_key=?`, destination.ID, key).Scan(&destinationAnswer, &destinationRevision); err != nil {
		t.Fatal(err)
	}
	if destinationAnswer != "Existing saved response" || destinationRevision != 1 {
		t.Fatalf("selecting a suggestion persisted it: %q revision=%d", destinationAnswer, destinationRevision)
	}

	page := httptestGet(t, newMux(a), "/jobs/2")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Related previous writing") ||
		!strings.Contains(page.Body.String(), "Original question") || !strings.Contains(page.Body.String(), "Use as starting point") {
		t.Fatalf("Question Workbench did not present related writing: %d %s", page.Code, page.Body.String())
	}
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=? AND question_key=?`, destination.ID, key).Scan(&destinationAnswer, &destinationRevision); err != nil {
		t.Fatal(err)
	}
	if destinationAnswer != "Existing saved response" || destinationRevision != 1 {
		t.Fatalf("rendering workbench changed saved answer: %q revision=%d", destinationAnswer, destinationRevision)
	}
}

func TestAnswerLibrarySearchPerformanceWithHundredsOfEntries(t *testing.T) {
	a, _, _, _, _ := newAnswerLibraryFixture(t)
	for i := 1; i <= 500; i++ {
		question := "Describe a customer support challenge and resolution"
		answer := "I documented customer requirements and coordinated the support team."
		if i <= 12 {
			question = "Describe a difficult production incident you resolved"
			answer = "I investigated Linux logs and database diagnostics to restore the production service."
		}
		_, err := a.db.Exec(`INSERT INTO answer_library(original_question,answer,source_job_id,source_destination,source_provider,
			source_question_key,source_revision,source_inspection_id,source_inspection_fingerprint,source_field_id,source_field_kind,
			question_terms,answer_terms,search_index_version,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, question, answer, i, "https://jobs.ashbyhq.com/library/"+strconv.Itoa(i), "ashby",
			"question-"+strconv.Itoa(i), 1, i, "synthetic", "browser:question", "textarea",
			answerLibraryIndexTerms(question), answerLibraryIndexTerms(answer), answerLibrarySearchIndexVersion, answerLibraryTimestamp(), answerLibraryTimestamp())
		if err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	results, err := a.searchAnswerLibrary("Tell us about a challenging Linux production incident", answerLibraryRelatedLimit)
	elapsed := time.Since(started)
	if err != nil || len(results) != answerLibraryRelatedLimit {
		t.Fatalf("500-entry retrieval returned %d results, err=%v", len(results), err)
	}
	t.Logf("500 curated entries, one question: search, rank, and excerpt retrieval completed in %s", elapsed)
	questions := make([]preparedQuestion, 20)
	for i := range questions {
		questions[i] = preparedQuestion{Key: strconv.Itoa(i), Editable: true, Field: schema.Field{
			ID: "browser:incident-" + strconv.Itoa(i), Label: "Tell us about a challenging Linux production incident you investigated", Kind: "textarea",
		}}
	}
	started = time.Now()
	suggestions, err := a.answerLibrarySuggestionsForQuestions(questions)
	elapsed = time.Since(started)
	if err != nil || len(suggestions) != len(questions) {
		t.Fatalf("retrieval for 20 questions produced %d groups, err=%v", len(suggestions), err)
	}
	t.Logf("500 curated entries, 20 questions: Prepare retrieval completed in %s", elapsed)
}

func TestAnswerLibraryIndexesExistingCuratedSnapshotsOnStartup(t *testing.T) {
	a, dir := testWorkspace(t)
	question := "Describe a Linux incident you investigated"
	answer := "I investigated Linux system logs and recovered the service."
	if _, err := a.db.Exec(`INSERT INTO answer_library(original_question,answer,source_job_id,source_destination,source_provider,
		source_question_key,source_revision,source_inspection_id,source_inspection_fingerprint,source_field_id,source_field_kind,
		created_at,updated_at,search_index_version) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,0)`,
		question, answer, 50, "https://jobs.ashbyhq.com/library/old", "ashby", "old-question", 1, 55, "old-fingerprint", "browser:old", "textarea", answerLibraryTimestamp(), answerLibraryTimestamp()); err != nil {
		t.Fatal(err)
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, reopenedDB, a.resumeDir)
	var indexedQuestion, indexedAnswer string
	var version int
	if err := restarted.db.QueryRow(`SELECT question_terms,answer_terms,search_index_version FROM answer_library WHERE source_job_id=50`).Scan(&indexedQuestion, &indexedAnswer, &version); err != nil {
		t.Fatal(err)
	}
	if version != answerLibrarySearchIndexVersion || !strings.Contains(indexedQuestion, "linux") || !strings.Contains(indexedAnswer, "recovered") {
		t.Fatalf("startup did not index the already-curated snapshot: %q %q version=%d", indexedQuestion, indexedAnswer, version)
	}
	results, err := restarted.searchAnswerLibrary("Linux incident recovery", answerLibraryRelatedLimit)
	if err != nil || len(results) != 1 || results[0].OriginalQuestion != question {
		t.Fatalf("backfilled snapshot is not searchable: %+v %v", results, err)
	}
}

func TestAnswerLibraryStartingDraftSurvivesStaleSaveError(t *testing.T) {
	a, _, sourceJob, _, sourceKey := newAnswerLibraryFixture(t)
	sourceAnswer := "I investigated a Linux production incident using service logs."
	saveLibrarySourceAnswer(t, a, sourceJob, sourceKey, sourceAnswer, 0)
	if got := promoteLibrarySource(t, a, sourceJob.ID, sourceKey, 1); got.Code != http.StatusSeeOther {
		t.Fatalf("promote source: %d %s", got.Code, got.Body.String())
	}
	entry, err := a.answerLibraryEntries()
	if err != nil || len(entry) != 1 {
		t.Fatalf("library entries: %d %v", len(entry), err)
	}
	destination := Job{URL: "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000002", Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Second Fixture"}
	if err := a.saveJob(destination); err != nil {
		t.Fatal(err)
	}
	destination, err = a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:challenge", Label: "Tell us about a challenging Linux incident you investigated", Kind: "textarea"}
	if _, err := a.saveInspection(destination.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: destination.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	key := questionKey(destination.URL, "ashby", field)
	initial := "My original current-application answer."
	saveLibrarySourceAnswer(t, a, destination, key, initial, 0)
	selected := postApp(newMux(a), "/jobs/2/answers/starting-point", url.Values{
		"library_entry_id": {strconv.FormatInt(entry[0].ID, 10)}, "question_key": {key}, "revision": {"1"},
	}, "", "")
	if selected.Code != http.StatusOK || !strings.Contains(selected.Body.String(), sourceAnswer) {
		t.Fatalf("read starting text: %d %s", selected.Code, selected.Body.String())
	}
	newer := "A newer answer from another window."
	if _, err := a.db.Exec(`UPDATE application_answers SET answer=?,revision=revision+1 WHERE job_id=? AND question_key=?`, newer, destination.ID, key); err != nil {
		t.Fatal(err)
	}
	recoveryText := "Adapted draft text that must remain available for recovery."
	failed := postApp(newMux(a), "/jobs/2/answers/save", url.Values{
		"destination": {destination.URL}, "question_key": {key}, "revision": {"1"}, "answer": {recoveryText},
	}, "", "")
	if failed.Code != http.StatusConflict || !strings.Contains(failed.Body.String(), recoveryText) || !strings.Contains(failed.Body.String(), "saved answer changed") {
		t.Fatalf("stale save did not preserve the draft for recovery: %d %s", failed.Code, failed.Body.String())
	}
	var stored string
	var revision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=? AND question_key=?`, destination.ID, key).Scan(&stored, &revision); err != nil || stored != newer || revision != 2 {
		t.Fatalf("stale save overwrote the newer answer: %q revision=%d err=%v", stored, revision, err)
	}
}

func httptestGet(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
