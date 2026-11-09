package app

import (
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Structured ARIA controls (comboboxes, custom dropdowns, listboxes) carry
// recognizable employer questions that can never be operated automatically.
// They must be represented with their label and reliably observed choices,
// stay manual-only, and never enter save or Fill scope.
func TestStructuredCustomKindsStayManualOnly(t *testing.T) {
	for _, kind := range []string{"combobox", "custom_combobox", "listbox", "custom_radio_group", "custom_checkbox_group"} {
		f := schema.Field{ID: "browser:aria_group_hear", Label: "How did you hear about us?", Kind: kind, Options: []string{"Search", "Friend"}}
		if got := classifyApplicationField(f, true).Role; got != fieldUnsupported {
			t.Fatalf("kind %q classified as %v, want unsupported", kind, got)
		}
		if editableQuestion(f, true) || editableQuestionForPlatform(f, true, "ashby", CandidateProfile{Approved: true, City: "Detroit"}) {
			t.Fatalf("kind %q reported editable", kind)
		}
		if writingCategory(f, true, false) != "review" {
			t.Fatalf("kind %q projected as %q, want review", kind, writingCategory(f, true, false))
		}
		if got := safePublishedOptions(kind, f.Options); len(got) != 2 || got[0] != "Search" {
			t.Fatalf("kind %q lost observed choices: %v", kind, got)
		}
	}
	if got := safePublishedOptions("text", []string{"A"}); got != nil {
		t.Fatalf("free-text options leaked into display: %v", got)
	}
}

// Relationship-scoped display identities must never merge with genuine
// control identities, hide behind the unidentified bucket, or authorize
// transfer. They exist only so a recognizable question renders once.
func TestCustomDisplayIdentitiesNeverAuthorize(t *testing.T) {
	for _, id := range []string{"browser:aria_group_hear", "browser:aria_labelled_hear", "preview:aria_group_hear"} {
		if displayUnidentifiedControl(id) {
			t.Fatalf("display identity %q collapsed into the unidentified bucket", id)
		}
		if got := displayQuestionIdentity(id); got == "" {
			t.Fatalf("display identity %q lost its merge key", id)
		}
		if !stableQuestionIdentity(schema.Field{ID: id, Label: "How did you hear about us?", Kind: "combobox"}, true) {
			t.Fatalf("display identity %q unexpectedly unstable", id)
		}
	}
	if displayQuestionIdentity("browser:hear") == displayQuestionIdentity("browser:aria_group_hear") {
		t.Fatal("display identity merged with a genuine control identity")
	}
}

func TestStructuredQuestionsProjectedManualWithChoices(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:pattern", Label: "Preferred working pattern", Kind: "select", Options: []string{"Remote", "Office", "Hybrid"}},
		{ID: "browser:shift", Label: "Preferred shift", Kind: "radio", Options: []string{"Day", "Night"}},
		{ID: "browser:referral-source", Label: "How did you find us?", Kind: "combobox", Options: []string{"Search engine", "Friend", "Other"}},
		{ID: "browser:aria_group_hear-popup", Label: "How did you hear about Render?", Kind: "custom_combobox", Options: []string{"Search", "Friend", "Other"}},
		{ID: "browser:unidentified-controls", Label: "Controls without a stable field identity", Kind: "unsupported_ambiguous"},
		{ID: "browser:email", Label: "Email", Kind: "email"},
		{ID: "browser:resume", Label: "Resume", Kind: "file"},
	}
	liveRowID, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields})
	if err != nil {
		t.Fatal(err)
	}
	// The live inventory is current while the managed session is active at
	// that exact observation. Active derives from an attached session.
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	if a.browsers.sessions == nil {
		a.browsers.sessions = map[int64]readOnlySession{}
	}
	a.browsers.states[1] = browserRunState{Active: true, Destination: testAshbyPosting, LatestID: liveRowID}
	a.browsers.sessions[1] = &fakeBrowser{url: testAshbyPosting + "/application"}
	a.browsers.mu.Unlock()
	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	byLabel := map[string]preparedQuestion{}
	for _, q := range prep.Questions {
		byLabel[q.Field.Label] = q
	}
	for _, tc := range []struct {
		label       string
		editable    bool
		options     int
		inReady     bool
		inAction    bool
		inReview    bool
	}{
		{"Preferred working pattern", true, 3, false, true, false},
		{"Preferred shift", true, 2, false, true, false},
		{"How did you find us?", false, 3, false, false, true},
		{"How did you hear about Render?", false, 3, false, false, true},
	} {
		q, ok := byLabel[tc.label]
		if !ok {
			t.Fatalf("question %q missing from preparation", tc.label)
		}
		if q.Editable != tc.editable {
			t.Fatalf("question %q editable=%v, want %v", tc.label, q.Editable, tc.editable)
		}
	}
	for _, q := range prep.ReadyAnswers() {
		if strings.HasPrefix(q.Field.Label, "How did you") {
			t.Fatalf("manual-only question entered Fill scope: %+v", q.Field)
		}
	}
	for _, q := range prep.ActionableQuestions() {
		if strings.HasPrefix(q.Field.Label, "How did you") {
			t.Fatalf("manual-only question entered answer tasks: %+v", q.Field)
		}
	}
	found := map[string]bool{}
	for _, q := range prep.ManualReviewQuestions() {
		found[q.Field.Label] = true
	}
	if !found["How did you find us?"] || !found["How did you hear about Render?"] {
		t.Fatalf("manual review lost structured questions: %v", found)
	}

	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	rows := map[string]map[string]any{}
	for _, item := range unified {
		q := item.(map[string]any)
		rows[q["label"].(string)] = q
	}
	hear, ok := rows["How did you hear about Render?"]
	if !ok {
		t.Fatalf("Render-like question missing from unified projection: %v", rows)
	}
	if hear["editable"] != false || hear["triageCategory"] != "review" || hear["manualReason"] == "" {
		t.Fatalf("Render-like question misrepresented: %v", hear)
	}
	options, _ := hear["options"].([]any)
	if len(options) != 3 || options[0] != "Search" {
		t.Fatalf("Render-like question lost observed choices: %v", hear["options"])
	}
	if hear["kind"] == "select" || hear["kind"] == "radio" || hear["kind"] == "text" {
		t.Fatalf("Render-like question claims a fillable control type: %v", hear["kind"])
	}
	if count, _ := payload["unidentifiedCount"].(float64); count != 1 {
		t.Fatalf("unidentifiedCount=%v, want 1 (only the truly unidentified control)", payload["unidentifiedCount"])
	}

	// Saving against a manual-only question is refused; the observation that
	// produced it is untouched.
	rec := postWritingJSON(t, a, "POST", "/api/jobs/1/answers", `{"question_key":"`+rows["How did you hear about Render?"]["key"].(string)+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Search"}`)
	if rec.Code != 409 {
		t.Fatalf("manual-only save status=%d body=%s", rec.Code, rec.Body.String())
	}
	scope := continuationScope(t, a, 1)
	for _, key := range []string{"pendingAnswers", "readyAnswers"} {
		items, _ := scope[key].([]any)
		for _, item := range items {
			if strings.Contains(item.(map[string]any)["label"].(string), "How did you hear") {
				t.Fatalf("manual-only question in continuation %s", key)
			}
		}
	}
}

func TestListboxObservationPassesThrough(t *testing.T) {
	o := browser.Observation{URL: testAshbyPosting + "/application", Controls: []browser.Control{
		{ID: "aria_group_hear-popup", Label: "How did you hear about us?", Kind: "listbox", Options: []string{"Search", "Friend"}},
	}}
	result, err := browser.AsResult(testAshbyPosting, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fields) != 1 || result.Fields[0].Kind != "listbox" || len(result.Fields[0].Options) != 2 {
		t.Fatalf("listbox observation not preserved: %+v", result.Fields)
	}
	if got := classifyApplicationField(result.Fields[0], true).Role; got != fieldUnsupported {
		t.Fatalf("listbox classified as %v, want unsupported", got)
	}
}
