package app

import (
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// ATS question discovery corrections: Greenhouse public definitions and
// Lever-style browser observations must route profile, document,
// employer-only, and ambiguous controls away from applicant-written tasks
// while preserving genuine employer questions and saved writing.
//
// The fixtures below replicate the reported failure shapes: candidate name
// and contact controls, résumé and résumé-text controls, a placeholder-only
// free-text control, select/checkbox/radio questions, sensitive controls,
// machine-identified radio groups, and controls with no recoverable label.
func TestGreenhouseDiscoveryRoutesProfileDocumentAndGenuineQuestions(t *testing.T) {
	a, _ := testWorkspace(t)
	job := Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io", Title: "Synthetic Engineer", Company: "Synthetic Co"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	public := schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL, Fields: []schema.Field{
		{ID: "application:first_name", Label: "First Name", Kind: "input_text", Required: true, Group: "application"},
		{ID: "application:last_name", Label: "Last Name", Kind: "input_text", Required: true, Group: "application"},
		{ID: "application:email", Label: "Email", Kind: "input_text", Required: true, Group: "application"},
		{ID: "application:phone", Label: "Phone", Kind: "input_text", Group: "application"},
		{ID: "application:resume", Label: "Resume", Kind: "input_file", Required: true, Group: "application"},
		{ID: "application:resume_text", Label: "Resume", Kind: "textarea", Group: "application"},
		{ID: "application:question_123", Label: "Why do you want to join us?", Kind: "textarea", Group: "application"},
		{ID: "application:question_124", Label: "Describe a challenging support incident you resolved", Kind: "textarea", Required: true, Group: "application"},
		{ID: "application:source", Label: "How did you hear about us?", Kind: "multi_value_single_select", Options: []string{"Search", "Friend", "Other"}, Group: "application"},
		{ID: "compliance:gender", Label: "Gender", Kind: "multi_value_single_select", Sensitive: true, Group: "compliance"},
	}}
	if _, err := a.saveInspection(1, public); err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]earlyApplicationQuestion{}
	for _, q := range prep.Questions {
		byID[q.SourceQuestionID] = q
	}
	for _, id := range []string{"application:first_name", "application:last_name", "application:email", "application:phone"} {
		q, ok := byID[id]
		if !ok {
			t.Fatalf("profile field %q missing from preparation", id)
		}
		if !q.FromProfile() || q.Editable {
			t.Fatalf("profile field %q triaged as %q editable=%v", id, q.TriageCategory, q.Editable)
		}
	}
	// Résumé controls are documents, never applicant-written tasks, whatever
	// their control kind is.
	for _, id := range []string{"application:resume", "application:resume_text"} {
		q, ok := byID[id]
		if !ok {
			t.Fatalf("document field %q missing from preparation", id)
		}
		if q.Editable {
			t.Fatalf("document field %q exposed as an editable writing task", id)
		}
	}
	// Genuine custom questions keep their provider identity: Greenhouse names
	// custom questions question_N, which is authoritative here.
	for _, id := range []string{"application:question_123", "application:question_124", "application:source"} {
		q, ok := byID[id]
		if !ok {
			t.Fatalf("genuine question %q missing from preparation", id)
		}
		if !q.NeedsAnswer() || !q.Editable {
			t.Fatalf("genuine question %q triaged as %q editable=%v", id, q.TriageCategory, q.Editable)
		}
	}
	if q := byID["compliance:gender"]; !q.EmployerOnly() || q.Editable {
		t.Fatalf("sensitive control triaged as %q editable=%v", q.TriageCategory, q.Editable)
	}
	if len(prep.ActionableQuestions()) != 3 || len(prep.ProfileQuestions()) != 4 || len(prep.EmployerSiteQuestions()) != 1 {
		t.Fatalf("triage counts action=%d profile=%d employer=%d review=%d",
			len(prep.ActionableQuestions()), len(prep.ProfileQuestions()), len(prep.EmployerSiteQuestions()), len(prep.ManualReviewQuestions()))
	}

	// The unified Questions interface hides profile and document controls,
	// shows genuine questions as writing tasks, and routes the rest.
	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	labels := map[string]map[string]any{}
	for _, item := range unified {
		q := item.(map[string]any)
		labels[q["label"].(string)] = q
	}
	for _, unwanted := range []string{"First Name", "Last Name", "Email", "Phone", "Resume"} {
		if _, found := labels[unwanted]; found {
			t.Fatalf("non-question %q leaked into the Questions navigator", unwanted)
		}
	}
	for _, want := range []string{"Why do you want to join us?", "Describe a challenging support incident you resolved", "How did you hear about us?"} {
		q, found := labels[want]
		if !found {
			t.Fatalf("genuine question %q missing from the Questions navigator", want)
		}
		if q["triageCategory"] != "answer" || q["editable"] != true {
			t.Fatalf("genuine question %q misrepresented: %v", want, q)
		}
	}
	if count, _ := payload["profileCount"].(float64); count != 4 {
		t.Fatalf("profileCount=%v, want 4", payload["profileCount"])
	}
	if count, _ := payload["manualReviewCount"].(float64); count != 0 {
		t.Fatalf("manualReviewCount=%v, want 0", payload["manualReviewCount"])
	}

	// Placeholder and observer fallback wording never becomes a reusable
	// identity or a library entry.
	for _, field := range []schema.Field{
		{ID: "browser:notes", Label: "Unlabeled control", Kind: "text"},
		{ID: "browser:cards", Label: "Radio group: cards[5812fdbe-0000-4000-8000-000000000000][field4]", Kind: "radio"},
	} {
		if stableQuestionIdentity(field, true) || editableQuestion(field, true) || answerLibraryEligible(field, true) {
			t.Fatalf("fallback wording authorized as a question: %+v", field)
		}
		if reusableAnswerKey(job.URL, field) != "" {
			t.Fatalf("fallback wording minted a reusable identity: %+v", field)
		}
	}
	if answerLibraryEligible(schema.Field{ID: "browser:notes", Label: "Type your reply...", Kind: "textarea"}, true) {
		t.Fatal("placeholder wording qualified for the Answer Library")
	}

	// Previously saved writing on a genuine question survives rediscovery.
	story := byID["application:question_124"]
	rec := postWritingJSON(t, a, "POST", "/api/jobs/1/answers", `{"question_key":"`+story.Key+`","revision":0,"destination":"`+job.URL+`","discovery_source":"public_api","discovery_fingerprint":"`+story.DiscoveryFingerprint+`","source_question_id":"application:question_124","early":true,"answer":"I compared request logs with deployment times."}`)
	if rec.Code != 200 {
		t.Fatalf("early save status=%d body=%s", rec.Code, rec.Body.String())
	}
	again, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		t.Fatal(err)
	}
	kept := false
	for _, q := range again.Questions {
		if q.SourceQuestionID == "application:question_124" && q.Value == "I compared request logs with deployment times." && q.Revision == 1 {
			kept = true
		}
	}
	if !kept {
		t.Fatal("saved early writing did not survive rediscovery")
	}
}

func TestLeverBrowserObservationRoutesMachineLabelsToManualReview(t *testing.T) {
	a, _ := testWorkspace(t)
	leverPosting := "https://jobs.lever.co/synthetic/3a6264ff-2a29-41d4-95fa-fa52b86e9c69"
	if err := a.saveJob(Job{URL: leverPosting, Host: "jobs.lever.co", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	machineName := "cards[5812fdbe-0000-4000-8000-000000000000][field4]"
	live := schema.Result{Platform: "lever", Source: "browser", Completeness: "rendered_partial", ApplicationURL: leverPosting + "/apply", Fields: []schema.Field{
		{ID: "browser:name", Label: "Full name", Kind: "text", Required: true, Group: "rendered form"},
		{ID: "browser:email", Label: "Email", Kind: "email", Required: true, Group: "rendered form"},
		{ID: "browser:resume-upload-input", Label: "Resume", Kind: "file", Required: true, Group: "rendered form"},
		{ID: "browser:" + machineName, Label: "Radio group: " + machineName, Kind: "radio", Required: true, Options: []string{"Spreedly Careers Page", "LinkedIn", "Other (Please explain below)"}, Group: "rendered form"},
		{ID: "browser:hear-role", Label: "How did you hear about this role?", Kind: "radio", Required: true, Options: []string{"Search", "Friend", "Other"}, Group: "rendered form"},
		{ID: "browser:other-note", Label: "Other", Kind: "text", Group: "rendered form"},
		{ID: "browser:sponsorship", Label: "Will you require sponsorship?", Kind: "radio", Sensitive: true, Group: "rendered form"},
		{ID: "browser:custom_pronouns", Label: "Unlabeled control", Kind: "text", Group: "rendered form"},
		{ID: "browser:unidentified-controls", Label: "Controls without a stable field identity", Kind: "unsupported_ambiguous", Group: "rendered form"},
	}}
	liveRowID, err := a.saveInspection(1, live)
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
	a.browsers.states[1] = browserRunState{Active: true, Destination: leverPosting, LatestID: liveRowID}
	a.browsers.sessions[1] = &fakeBrowser{url: leverPosting + "/apply"}
	a.browsers.mu.Unlock()
	profile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	// The machine-labeled group, the unlabeled control, and the
	// unidentified-control bucket all need manual review; only the two
	// genuine questions are answer tasks.
	if len(prep.ActionableQuestions()) != 2 || len(prep.ProfileQuestions()) != 2 || len(prep.EmployerSiteQuestions()) != 1 || len(prep.ManualReviewQuestions()) != 3 {
		t.Fatalf("live triage counts action=%d profile=%d employer=%d review=%d",
			len(prep.ActionableQuestions()), len(prep.ProfileQuestions()), len(prep.EmployerSiteQuestions()), len(prep.ManualReviewQuestions()))
	}
	for _, q := range prep.ActionableQuestions() {
		if strings.HasPrefix(q.Field.Label, "Radio group: ") || q.Field.Label == "Unlabeled control" {
			t.Fatalf("ambiguous control entered answer tasks: %+v", q.Field)
		}
	}
	for _, q := range prep.ReadyAnswers() {
		if strings.HasPrefix(q.Field.Label, "Radio group: ") || q.Field.Label == "Unlabeled control" {
			t.Fatalf("ambiguous control entered Fill scope: %+v", q.Field)
		}
	}
	if len(pendingAnswers(prep.ReadyAnswers(), nil, false)) != 0 {
		t.Fatal("unanswered ambiguous controls entered continuation scope")
	}

	// The machine-identified radio group keeps its observed options for
	// manual completion, stays out of Fill scope, and mints no reuse.
	for _, q := range prep.ManualReviewQuestions() {
		if q.Field.ID != "browser:"+machineName {
			continue
		}
		if len(q.Field.Options) != 3 || q.Editable {
			t.Fatalf("machine-labeled group lost options or gained edit authority: %+v", q.Field)
		}
		if reusableAnswerKey(job.URL, q.Field) != "" {
			t.Fatal("machine-labeled group minted a reusable identity")
		}
		if q.HelpEligible() {
			t.Fatal("machine-labeled group gained writing assistance")
		}
	}

	// Saved writing on the genuine radio question is preserved and Fillable
	// only through the normal explicit authorization path.
	var hear preparedQuestion
	for _, q := range prep.ActionableQuestions() {
		if q.Field.Label == "How did you hear about this role?" {
			hear = q
		}
	}
	if hear.Key == "" {
		t.Fatal("genuine Lever radio question missing from answer tasks")
	}
	if err := func() error {
		tx, err := a.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := persistApplicationAnswer(tx, job, hear, 0, "Friend", false, 0); err != nil {
			return err
		}
		return tx.Commit()
	}(); err != nil {
		t.Fatal(err)
	}
	again, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.ReadyAnswers()) != 1 || again.ReadyAnswers()[0].Field.Label != "How did you hear about this role?" || again.ReadyAnswers()[0].Value != "Friend" {
		t.Fatalf("saved genuine answer not preserved: %+v", again.ReadyAnswers())
	}

	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	seen := map[string]bool{}
	for _, item := range unified {
		q := item.(map[string]any)
		label := q["label"].(string)
		seen[label] = true
		if strings.HasPrefix(label, "Radio group: ") && q["triageCategory"] == "answer" {
			t.Fatalf("fabricated radio label presented as an answer task: %q", label)
		}
		if label == "Unlabeled control" && q["triageCategory"] == "answer" {
			t.Fatalf("unlabeled control presented as an answer task: %v", q)
		}
	}
	for _, want := range []string{"How did you hear about this role?", "Other"} {
		if !seen[want] {
			t.Fatalf("genuine Lever question %q missing from the Questions navigator", want)
		}
	}
	// The ambiguous control is neither discarded nor promoted: it stays
	// visible as manual review with its observed options.
	var ambiguous map[string]any
	found := false
	for _, item := range unified {
		q := item.(map[string]any)
		if strings.HasPrefix(q["label"].(string), "Radio group: ") {
			ambiguous = q
			found = true
		}
	}
	if !found {
		t.Fatal("ambiguous Lever control silently discarded from the Questions navigator")
	}
	if ambiguous["triageCategory"] != "review" || ambiguous["editable"] != false {
		t.Fatalf("ambiguous Lever control misrepresented: %v", ambiguous)
	}
	if options, _ := ambiguous["options"].([]any); len(options) != 3 {
		t.Fatalf("ambiguous Lever control lost observed options: %v", ambiguous["options"])
	}
	for _, unwanted := range []string{"Full name", "Email", "Resume"} {
		if seen[unwanted] {
			t.Fatalf("non-question %q leaked into the Questions navigator", unwanted)
		}
	}
	if count, _ := payload["unidentifiedCount"].(float64); count != 1 {
		t.Fatalf("unidentifiedCount=%v, want 1", payload["unidentifiedCount"])
	}
	if count, _ := payload["manualReviewCount"].(float64); count != 3 {
		t.Fatalf("manualReviewCount=%v, want 3", payload["manualReviewCount"])
	}
	if count, _ := payload["profileCount"].(float64); count != 2 {
		t.Fatalf("profileCount=%v, want 2", payload["profileCount"])
	}
}
