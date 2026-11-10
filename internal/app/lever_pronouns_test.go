package app

import (
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Two observed controls for one Pronouns requirement (a choice collection
// plus a degenerate option-less control under the same employer wording)
// must present once under manual completion with the observed options and
// truthful guidance. This is display-only: both underlying records keep
// their identities, saved writing is preserved, and Fill scope is unchanged.
func TestLeverPronounsDuplicateManualRowCollapses(t *testing.T) {
	a, _ := testWorkspace(t)
	leverPosting := "https://jobs.lever.co/synthetic/3a6264ff-2a29-41d4-95fa-fa52b86e9c6a"
	if err := a.saveJob(Job{URL: leverPosting, Host: "jobs.lever.co", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.URL != leverPosting {
		t.Fatalf("unexpected job: %+v", job)
	}
	extended := []string{"He/him", "She/her", "They/them", "Xe/xem", "Ze/hir", "Ey/em", "Hir/hir", "Fae/faer", "Hu/hu", "Use name only", "Custom"}
	live := schema.Result{Platform: "lever", Source: "browser", Completeness: "rendered_partial", ApplicationURL: leverPosting + "/apply", Fields: []schema.Field{
		{ID: "browser:pronouns-extended", Label: "Pronouns", Kind: "radio", Required: true, Options: extended, Group: "rendered form"},
		{ID: "browser:pronouns-checkbox", Label: "Pronouns", Kind: "checkbox", Required: true, Group: "rendered form"},
	}}
	liveRowID, err := a.saveInspection(1, live)
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	if a.browsers.sessions == nil {
		a.browsers.sessions = map[int64]readOnlySession{}
	}
	a.browsers.states[1] = browserRunState{Active: true, Destination: leverPosting, LatestID: liveRowID}
	a.browsers.sessions[1] = &fakeBrowser{url: leverPosting + "/apply"}
	a.browsers.mu.Unlock()
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}

	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	var pronouns []map[string]any
	for _, item := range unified {
		q := item.(map[string]any)
		if q["label"] == "Pronouns" {
			pronouns = append(pronouns, q)
		}
	}
	if len(pronouns) != 1 {
		t.Fatalf("Pronouns requirement shown %d times, want once: %v", len(pronouns), pronouns)
	}
	shown := pronouns[0]
	if shown["triageCategory"] != "review" || shown["editable"] != false {
		t.Fatalf("merged Pronouns row misrepresented: %v", shown)
	}
	options, _ := shown["options"].([]any)
	if len(options) != len(extended) {
		t.Fatalf("merged Pronouns row lost observed options: %v", shown["options"])
	}
	for i, option := range extended {
		if options[i] != option {
			t.Fatalf("merged options=%v, want %v", options, extended)
		}
	}
	if reason, _ := shown["manualReason"].(string); reason == "" {
		t.Fatal("merged Pronouns row lacks manual-completion guidance")
	}
	if count, _ := payload["manualReviewCount"].(float64); count != 1 {
		t.Fatalf("manualReviewCount=%v, want 1", count)
	}
}

// The manual-row fold must never hide genuinely distinct questions: rows
// with disjoint option evidence, answer tasks, and observer-fallback wording
// all stay separate. Only a degenerate option-less row folds into the single
// option-bearing row that shares its exact wording.
func TestManualDuplicateRowsPreserveDistinctQuestions(t *testing.T) {
	// checkbox_group is never an editable answer control, so stable rows
	// triage as manual review with options preserved.
	review := func(id, label string, options []string) preparedQuestion {
		return preparedQuestion{
			Key: id, Field: schema.Field{ID: "browser:" + id, Label: label, Kind: "checkbox_group", Options: options},
		}
	}
	// Disjoint options: two distinct same-worded requirements stay separate.
	distinct := questionPreparation{Questions: []preparedQuestion{
		review("alpha", "Preferred shift", []string{"Days", "Nights"}),
		review("beta", "Preferred shift", []string{"Weekends", "Holidays"}),
	}}
	if got := len(distinct.ManualReviewQuestions()); got != 2 {
		t.Fatalf("distinct same-worded requirements merged: %d", got)
	}
	// Answer tasks are never folded, even with identical wording.
	mixed := questionPreparation{Questions: []preparedQuestion{
		{Key: "answer", Editable: true, Field: schema.Field{ID: "browser:story", Label: "Tell us about yourself", Kind: "textarea"}},
		review("manual", "Tell us about yourself", []string{"A", "B"}),
	}}
	if got := len(mixed.WorkbenchQuestions()); got != 2 {
		t.Fatalf("answer task folded: %d", got)
	}
	// Observer-fallback wording never merges.
	fallback := questionPreparation{Questions: []preparedQuestion{
		{Key: "first", Field: schema.Field{ID: "browser:notes-a", Label: "Unlabeled control", Kind: "text"}},
		{Key: "second", Field: schema.Field{ID: "browser:notes-b", Label: "Unlabeled control", Kind: "text"}},
	}}
	if got := len(fallback.ManualReviewQuestions()); got != 2 {
		t.Fatalf("fallback rows merged: %d", got)
	}
	// Saved values block folding.
	valued := questionPreparation{Questions: []preparedQuestion{
		review("group", "Pronouns", []string{"He/him"}),
		func() preparedQuestion {
			q := review("single", "Pronouns", nil)
			q.Value, q.Revision = "saved", 1
			return q
		}(),
	}}
	if got := len(valued.ManualReviewQuestions()); got != 2 {
		t.Fatalf("row with saved writing folded: %d", got)
	}
}
// Lever pronouns collections must never render as a Yes/No answer. Extended
// options (Xe/xem and beyond, custom text, use-name-only) have no qualified
// single-value profile mapping, so they stay manual-only with observed
// options preserved. Qualified He/him, She/her, They/them selections keep
// the existing profile path.
func TestLeverPronounsExtendedStaysManual(t *testing.T) {
	a, _ := testWorkspace(t)
	leverPosting := "https://jobs.lever.co/synthetic/3a6264ff-2a29-41d4-95fa-fa52b86e9c6a"
	if err := a.saveJob(Job{URL: leverPosting, Host: "jobs.lever.co", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	extended := []string{"He/him", "She/her", "They/them", "Xe/xem", "Ze/hir", "Ey/em", "Hir/hir", "Fae/faer", "Hu/hu", "Use name only", "Custom"}
	live := schema.Result{Platform: "lever", Source: "browser", Completeness: "rendered_partial", ApplicationURL: leverPosting + "/apply", Fields: []schema.Field{
		{ID: "browser:pronouns-extended", Label: "Pronouns", Kind: "radio", Required: true, Options: extended, Group: "rendered form"},
		{ID: "browser:pronouns-checkbox", Label: "Pronouns", Kind: "checkbox", Group: "rendered form"},
		{ID: "browser:pronouns-qualified", Label: "Pronouns", Kind: "select", Options: []string{"He/him", "She/her", "They/them", "Decline to state"}, Group: "rendered form"},
		{ID: "browser:hear-role", Label: "How did you hear about this role?", Kind: "radio", Required: true, Options: []string{"Search", "Friend", "Other"}, Group: "rendered form"},
	}}
	liveRowID, err := a.saveInspection(1, live)
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	if a.browsers.sessions == nil {
		a.browsers.sessions = map[int64]readOnlySession{}
	}
	a.browsers.states[1] = browserRunState{Active: true, Destination: leverPosting, LatestID: liveRowID}
	a.browsers.sessions[1] = &fakeBrowser{url: leverPosting + "/apply"}
	a.browsers.mu.Unlock()

	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Pronouns: "He/him", Approved: true}); err != nil {
		t.Fatal(err)
	}
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]preparedQuestion{}
	for _, q := range prep.Questions {
		byID[q.Field.ID] = q
	}
	// Extended collection: manual review, not editable, options preserved.
	ext, ok := byID["browser:pronouns-extended"]
	if !ok {
		t.Fatal("extended pronouns missing from preparation")
	}
	if writingCategory(ext.Field, true, ext.Editable) != "review" {
		t.Fatalf("extended pronouns triaged as %q, want review", writingCategory(ext.Field, true, ext.Editable))
	}
	if ext.Editable {
		t.Fatal("extended pronouns gained edit authority (would render as an answer control)")
	}
	if len(ext.Field.Options) != len(extended) {
		t.Fatalf("extended pronouns lost options: %v", ext.Field.Options)
	}
	if got := classifyApplicationField(ext.Field, true).Role; got != fieldUnsupported {
		t.Fatalf("extended pronouns role=%v, want unsupported", got)
	}
	if reusableAnswerKey(job.URL, ext.Field) != "" {
		t.Fatal("extended pronouns minted a reusable identity")
	}
	if ext.HelpEligible() {
		t.Fatal("extended pronouns gained writing assistance")
	}
	// Single checkbox pronouns: never a Yes/No answer task.
	single, ok := byID["browser:pronouns-checkbox"]
	if !ok {
		t.Fatal("checkbox pronouns missing from preparation")
	}
	if writingCategory(single.Field, true, single.Editable) != "review" {
		t.Fatalf("checkbox pronouns triaged as %q, want review", writingCategory(single.Field, true, single.Editable))
	}
	if single.Editable {
		t.Fatal("checkbox pronouns editable (would render as Yes/No)")
	}
	if editableQuestion(single.Field, true) {
		t.Fatal("checkbox pronouns qualified as an editable question")
	}
	// Qualified pronouns stay on the existing profile path.
	qualified, ok := byID["browser:pronouns-qualified"]
	if !ok {
		t.Fatal("qualified pronouns missing from preparation")
	}
	if got := classifyApplicationField(qualified.Field, true).Role; got != fieldCandidateProfile {
		t.Fatalf("qualified pronouns role=%v, want profile", got)
	}
	if !isProfileField(qualified.Field) {
		t.Fatal("qualified pronouns left the profile path")
	}
	// Genuine question still actionable and Fillable through the normal path.
	if len(prep.ActionableQuestions()) != 1 || prep.ActionableQuestions()[0].Field.Label != "How did you hear about this role?" {
		t.Fatalf("genuine Lever question lost: %+v", prep.ActionableQuestions())
	}
	for _, q := range prep.ActionableQuestions() {
		if strings.Contains(strings.ToLower(q.Field.Label), "pronoun") {
			t.Fatalf("pronouns control entered answer tasks: %+v", q.Field)
		}
	}
	for _, q := range prep.ReadyAnswers() {
		if strings.Contains(strings.ToLower(q.Field.Label), "pronoun") {
			t.Fatalf("pronouns control entered Fill scope: %+v", q.Field)
		}
	}
	if len(pendingAnswers(prep.ReadyAnswers(), nil, false)) != 0 {
		t.Fatal("unanswered pronouns entered continuation scope")
	}
	// No silent profile mapping for the extended collection, even with an
	// approved He/him preference that matches one of its options.
	if got, _ := resolveApplicationAnswer(ext.Field, "", false, profile, "", false); got != "" {
		t.Fatalf("extended pronouns silently mapped profile value %q", got)
	}
	// Qualified select keeps the existing qualified mapping.
	if got, _ := resolveApplicationAnswer(qualified.Field, "", false, profile, "", false); got != "He/him" {
		t.Fatalf("qualified pronouns mapping lost: %q", got)
	}

	// Unified Questions projection: one Pronouns row with the observed options
	// and manual guidance. The degenerate option-less control describing the
	// same requirement folds into the choice collection for display; both
	// underlying records, saved writing, and Fill scope are unchanged. No
	// Yes/No answer row for pronouns.
	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	var pronouns []map[string]any
	for _, item := range unified {
		q := item.(map[string]any)
		if q["label"] != "Pronouns" {
			continue
		}
		if q["triageCategory"] == "answer" {
			t.Fatalf("pronouns presented as an answer task (Yes/No risk): %v", q)
		}
		pronouns = append(pronouns, q)
	}
	if len(pronouns) != 1 {
		t.Fatalf("Pronouns requirement shown %d times, want once: %v", len(pronouns), pronouns)
	}
	shown := pronouns[0]
	if shown["triageCategory"] != "review" || shown["editable"] != false {
		t.Fatalf("merged Pronouns row misrepresented: %v", shown)
	}
	options, _ := shown["options"].([]any)
	if len(options) != len(extended) {
		t.Fatalf("merged Pronouns row lost observed options: %v", shown["options"])
	}
	for i, option := range extended {
		if options[i] != option {
			t.Fatalf("merged options=%v, want %v", options, extended)
		}
	}
	if reason, _ := shown["manualReason"].(string); reason == "" {
		t.Fatal("merged Pronouns row lacks manual-completion guidance")
	}
}
