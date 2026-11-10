package app

import (
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

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

	// Unified Questions projection: real labels with observed options as
	// manual work; no Yes/No answer row for pronouns.
	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	seenExtended, seenCheckbox := false, false
	for _, item := range unified {
		q := item.(map[string]any)
		label, _ := q["label"].(string)
		kind, _ := q["kind"].(string)
		triage, _ := q["triageCategory"].(string)
		editable, _ := q["editable"].(bool)
		if label != "Pronouns" {
			continue
		}
		if triage == "answer" {
			t.Fatalf("pronouns presented as an answer task (Yes/No risk): %v", q)
		}
		if kind == "checkbox" && triage == "answer" && editable {
			t.Fatalf("pronouns checkbox would render Yes/No: %v", q)
		}
		if options, _ := q["options"].([]any); len(options) == len(extended) && triage == "review" && !editable {
			seenExtended = true
		}
		if kind == "checkbox" && triage == "review" && !editable {
			seenCheckbox = true
		}
	}
	if !seenExtended {
		t.Fatal("extended pronouns not visible as manual review with observed options")
	}
	if !seenCheckbox {
		t.Fatal("checkbox pronouns not visible as manual review (Yes/No must be gone)")
	}
}
