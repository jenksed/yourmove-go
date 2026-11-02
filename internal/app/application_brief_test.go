package app

import (
	"context"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestFormatApplicationBriefExactOutputAndOptions(t *testing.T) {
	job := Job{
		Company: "Example Company", Title: "Technical Account Manager",
		SalaryText: "$90,000–$120,000", SalaryOrigin: "employer",
		URL: "https://example.test/careers/123", Description: "Own customer outcomes.",
	}
	questions := applicationQuestionSnapshot{
		Available: true, Source: "Current browser-observed form", Completeness: "rendered_complete",
		Questions: []briefQuestion{
			{Label: "Why are you interested in joining our team?", Required: true},
			{Label: "What is your preferred work mode?", Options: []string{"Remote", "Hybrid"}},
		},
	}
	finishQuestionSnapshot(&questions)
	got := formatApplicationBrief(job, questions, briefFormatApplication, briefScopeUnanswered)
	want := "JOB APPLICATION BRIEF\n\n" +
		"Company: Example Company\nPosition: Technical Account Manager\nCompensation: $90,000–$120,000\nCompensation source: Employer posting\nPosting: https://example.test/careers/123\n\n" +
		"JOB DESCRIPTION\n\nOwn customer outcomes.\n\n" +
		"QUESTIONS NEEDING MY INPUT\n\nSource: Current browser-observed form.\n\n" +
		"1. Why are you interested in joining our team?\n   Required\n\n" +
		"2. What is your preferred work mode?\n   Required state not verified\n   Published options:\n   - Remote\n   - Hybrid\n\nEND OF BRIEF\n"
	if got != want {
		t.Fatalf("application brief mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRequiredLabelIsHonestAndDoesNotChangeQuestionIdentity(t *testing.T) {
	if got := requiredLabel(true); got != "Required" {
		t.Fatalf("true required state label = %q", got)
	}
	if got := requiredLabel(false); got != "Required state not verified" {
		t.Fatalf("unverified required state label = %q", got)
	}

	jobURL := testAshbyPosting
	field := schema.Field{ID: "browser:motivation", Label: "Why this team?", Kind: "textarea"}
	result := schema.Result{
		Platform: "ashby", Source: "browser", Completeness: "rendered_partial",
		ApplicationURL: jobURL + "/application", Fields: []schema.Field{field},
	}
	fieldBefore := result.Fields[0]
	fingerprintBefore := result.Fingerprint()
	keyBefore := questionKey(jobURL, result.Platform, fieldBefore)
	_ = requiredLabel(field.Required)

	if result.Fingerprint() != fingerprintBefore {
		t.Fatal("required-state presentation changed the schema fingerprint")
	}
	if got := questionKey(jobURL, result.Platform, result.Fields[0]); got != keyBefore {
		t.Fatal("required-state presentation changed the question key")
	}
	if !reflect.DeepEqual(result.Fields[0], fieldBefore) || result.Fields[0].Required {
		t.Fatal("required-state presentation changed the stored field evidence")
	}
}

func TestApplicationBriefDoesNotTruncateFullDescription(t *testing.T) {
	description := strings.Repeat("Saved description detail — ", 12000) + "FULL_DESCRIPTION_END_MARKER"
	got := formatApplicationBrief(Job{Description: description}, applicationQuestionSnapshot{}, briefFormatApplication, briefScopeUnanswered)
	if !strings.Contains(got, description) || !strings.HasSuffix(got, "END OF BRIEF\n") {
		t.Fatal("full saved description was truncated or lost")
	}
}

func TestApplicationBriefQuestionSelectionUsesOneAuthoritativeInspection(t *testing.T) {
	a, _ := testWorkspace(t)
	job := Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io", Title: "Synthetic Engineer", Company: "Synthetic Co"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	earlyID, err := a.saveInspection(1, schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL, Fields: []schema.Field{
		{ID: "application:shared", Label: "What is your greatest strength?", Kind: "textarea", Required: true},
		{ID: "application:early-only", Label: "Why this role?", Kind: "textarea"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	liveID, err := a.saveInspection(1, schema.Result{Platform: "greenhouse", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL, Fields: []schema.Field{
		{ID: "browser:shared", Label: "What is your greatest strength?", Kind: "textarea", Required: true},
		{ID: "browser:live-only", Label: "Describe a customer challenge.", Kind: "textarea"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	job, _ = a.jobByID(1)
	profile, _ := a.candidateProfile()
	live, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		t.Fatal(err)
	}
	if early.Item == nil || early.Item.ID != earlyID || live.InspectionID != liveID {
		t.Fatalf("fixture inspections were not selected: early=%+v live=%+v", early.Item, live)
	}

	inactive := chooseApplicationQuestionSnapshot(job, browserRunState{}, live, early)
	if inactive.Source != "Published question definitions" || len(inactive.Questions) != 2 || inactive.Questions[1].Label != "Why this role?" {
		t.Fatalf("inactive session did not select only published questions: %+v", inactive)
	}
	state := browserRunState{Active: true, Destination: job.URL, LatestID: liveID}
	active := chooseApplicationQuestionSnapshot(job, state, live, early)
	if active.Source != "Current browser-observed form" || len(active.Questions) != 2 || active.Questions[1].Label != "Describe a customer challenge." {
		t.Fatalf("active session did not select only current browser questions: %+v", active)
	}
	if strings.Count(formatApplicationBrief(job, active, briefFormatQuestions, briefScopeAll), "What is your greatest strength?") != 1 {
		t.Fatal("same-labeled early/live question was duplicated")
	}
	if !reflect.DeepEqual(active, chooseApplicationQuestionSnapshot(job, state, live, early)) {
		t.Fatal("question-source selection was not deterministic")
	}
	state.LatestID++
	if got := chooseApplicationQuestionSnapshot(job, state, live, early); got.Source != "Published question definitions" {
		t.Fatalf("stale live observation was selected: %+v", got)
	}
}

func TestApplicationBriefUsesLatestValidEarlyDiscoveryAndGreenhouseSource(t *testing.T) {
	a, _ := testWorkspace(t)
	job := Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	older, err := a.saveInspection(1, schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL, Fields: []schema.Field{{ID: "application:valid", Label: "Why are you interested in this role?", Kind: "textarea"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: "https://job-boards.greenhouse.io/other/jobs/1", Fields: []schema.Field{{ID: "application:wrong", Label: "Wrong posting question", Kind: "textarea"}}}); err != nil {
		t.Fatal(err)
	}
	selected, err := a.latestValidInspectionForPosting(1, "public_api", job.URL)
	if err != nil || selected.ID != older {
		t.Fatalf("latest valid posting discovery = %d, err=%v; want %d", selected.ID, err, older)
	}
	job, _ = a.jobByID(1)
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil || early.Item == nil || early.Item.ID != older {
		t.Fatalf("preparation did not use the latest valid discovery: item=%+v err=%v", early.Item, err)
	}
	profile, _ := a.candidateProfile()
	live, _ := a.prepareQuestions(job, profile)
	questions := chooseApplicationQuestionSnapshot(job, browserRunState{}, live, early)
	text := formatApplicationBrief(job, questions, briefFormatQuestions, briefScopeUnanswered)
	if !strings.Contains(text, "Source: Published question definitions.") || !strings.Contains(text, "Inspection is partial; later or conditional questions may not be represented.") || strings.Contains(text, "Wrong posting question") {
		t.Fatalf("published Greenhouse source/partial note incorrect: %s", text)
	}
}

func makeAshbyBriefFixture(t *testing.T, a *App, description, salary string) Job {
	t.Helper()
	job := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Technical Account Manager", Company: "Synthetic Northstar", SalaryText: salary, SalaryOrigin: "employer", Description: description}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	profile := CandidateProfile{Approved: true, FirstName: "PRIVATE_FIRST_NAME", LastName: "PRIVATE_LAST_NAME", Email: "PRIVATE_EMAIL@example.test", Phone: "PRIVATE_PHONE", City: "PRIVATE_CITY", Region: "PRIVATE_REGION", Country: "PRIVATE_COUNTRY", Pronouns: "They/them"}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	namespace := "preview:ff5bb721-1230-4753-bb72-2f8324c0bf26_"
	result := schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{
		{ID: "preview:why-team", Label: "Why are you interested in joining our team?", Kind: "textarea", Required: true},
		{ID: "preview:enterprise", Label: "Describe your experience supporting enterprise customers.", Kind: "textarea", Required: true},
		{ID: "preview:product", Label: "What interests you about this product?", Kind: "textarea"},
		{ID: "preview:work-mode", Label: "Which work mode do you prefer?", Kind: "select", Options: []string{"Remote", "Hybrid"}},
		{ID: "preview:saved-story", Label: "Which project are you most proud of?", Kind: "textarea"},
		{ID: "preview:same-label-a", Label: "Describe a customer challenge you resolved.", Kind: "textarea"},
		{ID: "preview:same-label-b", Label: "Describe a customer challenge you resolved.", Kind: "textarea"},
		{ID: namespace + "_systemfield_name", Label: "Full Name", Kind: "text", Required: true},
		{ID: "preview:demographic", Label: "Race and ethnicity", Kind: "checkbox_group", Sensitive: true},
		{ID: "preview:consent", Label: "I agree to the terms", Kind: "checkbox", Required: true},
		{ID: "preview:unsupported", Label: "Choose a location on the map", Kind: "custom_widget"},
	}}
	if _, err := a.saveInspection(1, result); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, question := range prep.ActionableQuestions() {
		if question.SourceQuestionID == "preview:saved-story" {
			form := earlySaveValues(job, question, "0", "PRIVATE_SAVED_ANSWER_TEXT")
			if got := postApp(newMux(a), "/jobs/1/answers/early-save", form, "", ""); got.Code != 303 {
				t.Fatalf("save synthetic answer: %d %s", got.Code, got.Body.String())
			}
			return job
		}
	}
	t.Fatal("saved-answer question fixture was not actionable")
	return Job{}
}

func briefRoute(a *App, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+path, nil))
	return w
}

func TestApplicationBriefExcludesProfileSensitiveAndSavedAnswerContent(t *testing.T) {
	a, _ := testWorkspace(t)
	job := makeAshbyBriefFixture(t, a, "A complete synthetic posting description.", "$90,000–$120,000")
	w := briefRoute(a, "/jobs/1/brief")
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("brief response: %d %q %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	text := w.Body.String()
	for _, want := range []string{job.Company, job.Title, job.SalaryText, "Compensation source: Employer posting", job.URL, job.Description, "Read-only application preview", "Required", "Required state not verified", "Published options:", "- Remote", "- Hybrid"} {
		if !strings.Contains(text, want) {
			t.Errorf("brief missing %q", want)
		}
	}
	for _, private := range []string{
		"PRIVATE_FIRST_NAME", "PRIVATE_LAST_NAME", "PRIVATE_EMAIL", "PRIVATE_PHONE", "PRIVATE_CITY", "PRIVATE_REGION", "PRIVATE_COUNTRY", "They/them",
		"Full Name", "Race and ethnicity", "I agree to the terms", "Choose a location on the map", "Which project are you most proud of?", "PRIVATE_SAVED_ANSWER_TEXT", "UNSAVED_EDITOR_TEXT",
	} {
		if strings.Contains(text, private) {
			t.Errorf("brief contains excluded content %q", private)
		}
	}
	if count := strings.Count(text, "Describe a customer challenge you resolved."); count != 2 {
		t.Fatalf("same-label actionable identities were collapsed or repeated incorrectly: count=%d", count)
	}
	if strings.Index(text, "Why are you interested in joining our team?") > strings.Index(text, "Describe your experience supporting enterprise customers.") ||
		strings.Index(text, "Describe your experience supporting enterprise customers.") > strings.Index(text, "What interests you about this product?") {
		t.Fatal("question order did not match the saved inspection order")
	}
	if !strings.Contains(text, "Inspection is partial; later or conditional questions may not be represented.") {
		t.Fatal("partial Ashby preview was not described")
	}

	all := briefRoute(a, "/jobs/1/brief?scope=all")
	if all.Code != 200 || !strings.Contains(all.Body.String(), "ACTIONABLE APPLICATION QUESTIONS (WORDING ONLY)") || !strings.Contains(all.Body.String(), "Which project are you most proud of?") || !strings.Contains(all.Body.String(), "Question wording only; saved answers are not included.") || strings.Contains(all.Body.String(), "PRIVATE_SAVED_ANSWER_TEXT") {
		t.Fatalf("all-actionable export did not include wording only: %d %s", all.Code, all.Body.String())
	}
	questions := briefRoute(a, "/jobs/1/brief?format=questions")
	if questions.Code != 200 || strings.Contains(questions.Body.String(), job.Description) || strings.Contains(questions.Body.String(), job.Company) {
		t.Fatalf("Copy questions included job details: %d %s", questions.Code, questions.Body.String())
	}
	details := briefRoute(a, "/jobs/1/brief?format=details")
	if details.Code != 200 || strings.Contains(details.Body.String(), "QUESTIONS NEEDING MY INPUT") || !strings.Contains(details.Body.String(), job.Description) {
		t.Fatalf("Copy job details format incorrect: %d %s", details.Code, details.Body.String())
	}
}

func TestApplicationBriefMissingDetailsAndNoDiscovery(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/jobs/without-details", Host: "example.test", Title: "Unpriced role"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.test/jobs/other", Host: "example.test", SalaryText: "$250,000"}); err != nil {
		t.Fatal(err)
	}
	w := briefRoute(a, "/jobs/1/brief")
	if w.Code != 200 {
		t.Fatalf("missing-detail brief status: %d %s", w.Code, w.Body.String())
	}
	text := w.Body.String()
	for _, want := range []string{"Company: Not specified", "Position: Unpriced role", "Compensation: Not specified", "Posting: https://example.test/jobs/without-details", "Not available in the saved job record.", "Application questions have not been discovered yet."} {
		if !strings.Contains(text, want) {
			t.Errorf("missing-detail brief missing %q", want)
		}
	}
	if strings.Contains(text, "$250,000") {
		t.Fatal("compensation was guessed from another job")
	}
	if invalid := briefRoute(a, "/jobs/1/brief?scope=other"); invalid.Code != 400 {
		t.Fatalf("unknown export scope status=%d, want 400", invalid.Code)
	}
}

func TestApplicationBriefDoesNotWriteOrCallProviders(t *testing.T) {
	a, _ := testWorkspace(t)
	makeAshbyBriefFixture(t, a, "Synthetic description", "$100,000")
	var answerCalls, letterCalls int
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) { answerCalls++; return "provider output", nil }
	a.coverLetterGenerate = func(context.Context, string) (string, error) { letterCalls++; return "provider output", nil }
	tables := []string{"jobs", "application_inspections", "application_answers", "early_application_answers", "cover_letters", "submissions", "answer_library", "career_evidence"}
	counts := func() map[string]int {
		t.Helper()
		out := make(map[string]int, len(tables))
		for _, table := range tables {
			var count int
			if err := a.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
				t.Fatal(err)
			}
			out[table] = count
		}
		return out
	}
	before := counts()
	var beforeWrites int64
	if err := a.db.QueryRow(`SELECT total_changes()`).Scan(&beforeWrites); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/jobs/1/brief", "/jobs/1/brief?format=questions", "/jobs/1/brief?format=details&scope=all"} {
		if w := briefRoute(a, path); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if after := counts(); !reflect.DeepEqual(before, after) {
		t.Fatalf("copy wrote database rows: before=%v after=%v", before, after)
	}
	var afterWrites int64
	if err := a.db.QueryRow(`SELECT total_changes()`).Scan(&afterWrites); err != nil || afterWrites != beforeWrites {
		t.Fatalf("copy executed database writes: before=%d after=%d err=%v", beforeWrites, afterWrites, err)
	}
	if answerCalls != 0 || letterCalls != 0 {
		t.Fatalf("copy called a provider: answer=%d cover=%d", answerCalls, letterCalls)
	}
}
