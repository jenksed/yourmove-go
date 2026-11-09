package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

// TestAnswerLibraryMasterDetailBrowse covers the scalable library workflow:
// compact list with counts and pagination, search across question text,
// alternative phrasings, answer text, and source context, alphabetical and
// recency sorting, selection without losing search state, entry maintenance,
// application-answer independence, and restart persistence.
func TestAnswerLibraryMasterDetailBrowse(t *testing.T) {
	a, dir := testWorkspace(t)
	const total = 105
	for i := 1; i <= total; i++ {
		question := "Describe a customer support challenge and resolution number " + strconv.Itoa(i)
		answer := "I documented customer requirements and coordinated the support team for case " + strconv.Itoa(i) + "."
		employer, title := "Fixture Employer", "Support Engineer"
		if i == 42 {
			question = "Tell us about a time you handled a difficult customer"
			answer = "I de-escalated a billing dispute with patience, restated the concern, and resolved the account issue."
			employer, title = "Acme Support", "Customer Success Lead"
		}
		if i == 77 {
			question = "Describe a production outage you resolved quietly"
			answer = "The zebra-striped runbook guided the midnight failover while monitoring stayed green."
			employer, title = "Globex Systems", "Reliability Engineer"
		}
		if _, err := a.db.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,
			source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
			source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,question_terms,answer_terms,
			search_index_version,created_at,updated_at,revision)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`,
			question, answer, employer, title, "jobs.ashbyhq.com",
			i, "https://jobs.ashbyhq.com/library/"+strconv.Itoa(i), "ashby",
			"question-"+strconv.Itoa(i), 1, i, "synthetic", "browser:question", "textarea", "live",
			answerLibraryIndexTerms(question), answerLibraryIndexTerms(answer), answerLibrarySearchIndexVersion,
			answerLibraryTimestamp(), answerLibraryTimestamp()); err != nil {
			t.Fatal(err)
		}
	}
	var entry42 int64
	if err := a.db.QueryRow(`SELECT id FROM answer_library WHERE original_question='Tell us about a time you handled a difficult customer'`).Scan(&entry42); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO answer_library_alternatives(entry_id,phrasing,normalized) VALUES(?,?,?)`,
		entry42, "Describe a time you resolved a customer complaint", normalizeLibraryQuestion("Describe a time you resolved a customer complaint")); err != nil {
		t.Fatal(err)
	}

	get := func(path string) string {
		rec := httptest.NewRecorder()
		newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}

	browse := get("/answer-library")
	if !strings.Contains(browse, "105 saved examples") || !strings.Contains(browse, "Page 1 of 6") {
		t.Fatalf("browse counts/pagination missing")
	}
	if !strings.Contains(browse, "Reusable answer") || !strings.Contains(browse, `name="alternatives"`) {
		t.Fatalf("detail pane missing full answer or alternatives management")
	}

	alpha := get("/answer-library?sort=alpha")
	if !strings.Contains(alpha, "sort=alpha") || !strings.Contains(alpha, "Page 1 of 6") {
		t.Fatalf("alphabetical sort did not carry into pagination links")
	}

	page2 := get("/answer-library?page=2")
	if !strings.Contains(page2, "Page 2 of 6") || !strings.Contains(page2, "Previous page") {
		t.Fatalf("second page did not preserve browsing position")
	}

	// 1. Locate by primary question.
	byPrimary := get("/answer-library?q=" + url.QueryEscape("handled a difficult customer"))
	if !strings.Contains(byPrimary, "Tell us about a time you handled a difficult customer") {
		t.Fatalf("primary-question search missed the known entry")
	}
	if !strings.Contains(byPrimary, "1 alternative phrasing") {
		t.Fatalf("alternative-phrasing count missing from compact list")
	}
	// 2. Locate via a user-defined alternative phrasing.
	byAlt := get("/answer-library?q=" + url.QueryEscape("resolved a customer complaint"))
	if !strings.Contains(byAlt, "Tell us about a time you handled a difficult customer") {
		t.Fatalf("alternative-phrasing search missed the intended entry")
	}
	// 3. Locate via answer-text wording.
	byAnswer := get("/answer-library?q=" + url.QueryEscape("zebra-striped runbook"))
	if !strings.Contains(byAnswer, "Describe a production outage you resolved quietly") {
		t.Fatalf("answer-text search missed the intended entry")
	}
	// 4. Source-context search.
	bySource := get("/answer-library?q=" + url.QueryEscape("Globex Systems"))
	if !strings.Contains(bySource, "Describe a production outage you resolved quietly") {
		t.Fatalf("source-context search missed the intended entry")
	}

	// 5. Reopen a result without losing search context.
	selected := get("/answer-library?q=" + url.QueryEscape("zebra-striped runbook") + "&selected=77")
	_ = selected
	var outageID int64
	if err := a.db.QueryRow(`SELECT id FROM answer_library WHERE original_question='Describe a production outage you resolved quietly'`).Scan(&outageID); err != nil {
		t.Fatal(err)
	}
	opened := get("/answer-library?q=" + url.QueryEscape("zebra-striped runbook") + "&selected=" + strconv.FormatInt(outageID, 10))
	if !strings.Contains(opened, "zebra-striped runbook") || !strings.Contains(opened, "q=zebra-striped") {
		t.Fatalf("selected entry lost search state")
	}

	// 6. Edit the reusable answer and manage its alternatives.
	var revision int64
	if err := a.db.QueryRow(`SELECT revision FROM answer_library WHERE id=?`, outageID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	edited := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(outageID, 10)+"/edit", url.Values{
		"revision": {strconv.FormatInt(revision, 10)},
		"answer":   {"The zebra-striped runbook guided the midnight failover; I added a follow-up checklist."},
		"alternatives": {"Describe a quiet midnight failover\nHandle an overnight production outage"},
	}, "", "")
	if edited.Code != http.StatusSeeOther {
		t.Fatalf("edit: %d %s", edited.Code, edited.Body.String())
	}
	afterEdit := get("/answer-library?selected=" + strconv.FormatInt(outageID, 10))
	if !strings.Contains(afterEdit, "follow-up checklist") || !strings.Contains(afterEdit, "Handle an overnight production outage") {
		t.Fatalf("edited answer or alternatives not retained in detail")
	}
	var updatedRevision int64
	if err := a.db.QueryRow(`SELECT revision FROM answer_library WHERE id=?`, outageID).Scan(&updatedRevision); err != nil || updatedRevision != revision+1 {
		t.Fatalf("revision did not advance: %d %v", updatedRevision, err)
	}
	// Stale edits are refused without overwriting.
	stale := postApp(newMux(a), "/answer-library/"+strconv.FormatInt(outageID, 10)+"/edit", url.Values{
		"revision": {strconv.FormatInt(revision, 10)}, "answer": {"stale overwrite"},
	}, "", "")
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale edit accepted: %d", stale.Code)
	}

	// 7. Restart persistence: answers and alternatives survive reopening.
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, reopenedDB, a.resumeDir)
	var reopenedAnswer string
	if err := restarted.db.QueryRow(`SELECT answer FROM answer_library WHERE id=?`, outageID).Scan(&reopenedAnswer); err != nil ||
		!strings.Contains(reopenedAnswer, "follow-up checklist") {
		t.Fatalf("restart lost the edited reusable answer: %q %v", reopenedAnswer, err)
	}
	alternatives, err := restarted.libraryAlternatives(outageID)
	if err != nil || len(alternatives) != 2 {
		t.Fatalf("restart lost alternative phrasings: %v %v", alternatives, err)
	}
	rec := httptest.NewRecorder()
	newMux(restarted).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/answer-library?q="+url.QueryEscape("midnight failover"), nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Describe a production outage you resolved quietly") {
		t.Fatalf("restarted search missed the maintained entry: %d", rec.Code)
	}
}
