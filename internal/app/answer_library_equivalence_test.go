package app

import (
	"net/http"
	"testing"
)

func TestCombinedLibrarySaveAndEquivalence(t *testing.T) {
	a, _ := testWorkspace(t)
	key, _ := seedWritingJob(t, a)
	save := func(rev int64, answer string) int {
		return postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+key+`","revision":`+itoa(rev)+`,"destination":"`+testAshbyPosting+`","answer":`+jsonString(answer)+`,"save_to_library":true}`).Code
	}
	if got := save(0, "Original reusable story"); got != 200 {
		t.Fatalf("save: %d", got)
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries: %v %v", entries, err)
	}
	e := entries[0]
	if err := a.updateAnswerLibraryWithAlternatives(e.ID, e.Revision, e.Answer, []string{"Tell us about a challenging customer interaction."}); err != nil {
		t.Fatal(err)
	}
	ids, err := equivalentLibraryIDs(a.db, "  TELL us about a challenging customer interaction.  ")
	if err != nil || len(ids) != 1 || ids[0] != e.ID {
		t.Fatalf("equivalence: %v %v", ids, err)
	}
	ids, _ = equivalentLibraryIDs(a.db, "Tell us about a difficult customer interaction.")
	if len(ids) != 0 {
		t.Fatal("unapproved synonyms matched")
	}
	if got := save(1, "Employer-specific adaptation"); got != 200 {
		t.Fatalf("adaptation: %d", got)
	}
	entries, _ = a.answerLibraryEntries()
	if len(entries) != 1 || entries[0].Answer != e.Answer {
		t.Fatal("library duplicated or overwritten")
	}
	if got := save(0, "Stale"); got != 409 {
		t.Fatalf("stale: %d", got)
	}
	if err := a.updateAnswerLibraryWithAlternatives(e.ID, 1, "stale", []string{}); err != ErrAnswerLibraryStale {
		t.Fatalf("stale edit: %v", err)
	}
	if reviewCount(t, a) != 0 {
		t.Fatal("save created employer review")
	}
}

func TestCombinedSaveRollsBackOnLibraryFailure(t *testing.T) {
	a, _ := testWorkspace(t)
	key, _ := seedWritingJob(t, a)
	if _, err := a.db.Exec(`CREATE TRIGGER reject_library BEFORE INSERT ON answer_library BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
		t.Fatal(err)
	}
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+key+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Synthetic story","save_to_library":true}`)
	if rec.Code == 200 {
		t.Fatal("failure reported success")
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial save: %d %v", count, err)
	}
}

func TestConflictingEquivalentsRequireReview(t *testing.T) {
	a, _, job, field, key := newAnswerLibraryFixture(t)
	saveLibrarySourceAnswer(t, a, job, key, "First story", 0)
	first, _, err := a.promoteApplicationAnswer(job.ID, key, 1)
	if err != nil {
		t.Fatal(err)
	}
	saveLibrarySourceAnswer(t, a, job, key, "Second story", 1)
	second, _, err := a.promoteApplicationAnswer(job.ID, key, 2)
	if err != nil {
		t.Fatal(err)
	}
	alias := "Tell us about a challenging customer interaction."
	for _, e := range []AnswerLibraryEntry{first, second} {
		if err := a.updateAnswerLibraryWithAlternatives(e.ID, e.Revision, e.Answer, []string{alias}); err != nil {
			t.Fatal(err)
		}
	}
	field.Label = alias
	suggestions, err := a.answerLibrarySuggestionsForQuestions([]preparedQuestion{{Key: "target", Field: field, Editable: true}})
	if err != nil || len(suggestions["target"]) != 2 {
		t.Fatalf("conflicts: %v %v", suggestions, err)
	}
	for _, s := range suggestions["target"] {
		if s.MatchKind != "Conflicting equivalents — choose after review" {
			t.Fatalf("unmarked conflict: %+v", s)
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := saveWritingLibrary(tx, job, writingSaveInput{Answer: "Adaptation"}, alias, "textarea", true); err != ErrAnswerLibraryInvalid {
		t.Fatalf("ambiguous save: %v", err)
	}
}
