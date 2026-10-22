package app

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestEarlyQuestionTriageUsesApprovedProfileAndKeepsDrafts(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	profile := CandidateProfile{Approved: true, FirstName: "Alex", LastName: "Example", Email: "alex@example.test", Phone: "5551231234", City: "Detroit", Region: "MI", Country: "US", LinkedIn: "https://linkedin.example.test"}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	const ns = "preview:ff5bb721-1230-4753-bb72-2f8324c0bf26_"
	result := schema.Result{Platform: "ashby", Source: "preview", Completeness: "partial", ApplicationURL: testAshbyPosting, Fields: []schema.Field{
		{ID: ns + "_systemfield_name", Label: "Full Name", Kind: "text", Required: true},
		{ID: ns + "_systemfield_email", Label: "Email", Kind: "email", Required: true},
		{ID: "preview:why-company", Label: "Why are you interested in this company?", Kind: "textarea", Required: true},
		{ID: "preview:incident", Label: "Describe a challenging support incident", Kind: "textarea"},
		{ID: "preview:work-style", Label: "Preferred work style", Kind: "select", Options: []string{"Remote", "Hybrid"}},
		{ID: "preview:demographic", Label: "Race and ethnicity", Kind: "checkbox_group", Required: false, Sensitive: true},
		{ID: "preview:attestation", Label: "I certify that the information is accurate", Kind: "checkbox", Required: true},
		{ID: "preview:custom", Label: "Choose a location on the map", Kind: "custom_widget"},
		{ID: "preview:group-a", Label: "Which customer segment?", Kind: "unsupported_ambiguous"},
		{ID: "preview:group-b", Label: "Which customer segment?", Kind: "unsupported_ambiguous"},
		{ID: "preview:unidentified-0", Label: "Unidentified control", Kind: "text"},
	}}
	if _, err := a.saveInspection(1, result); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(prep.Questions) != 11 || len(prep.ActionableQuestions()) != 3 || len(prep.ProfileQuestions()) != 2 || len(prep.EmployerSiteQuestions()) != 2 || len(prep.ManualReviewQuestions()) != 4 {
		t.Fatalf("triage counts total=%d action=%d profile=%d employer=%d review=%d", len(prep.Questions), len(prep.ActionableQuestions()), len(prep.ProfileQuestions()), len(prep.EmployerSiteQuestions()), len(prep.ManualReviewQuestions()))
	}
	if prep.SelectedKey != prep.ActionableQuestions()[0].Key || prep.ActionableQuestions()[0].Field.Label != "Why are you interested in this company?" {
		t.Fatalf("initial selection is not first unanswered actionable question: %+v", prep)
	}
	if prep.ProfileQuestions()[0].ProfileValue != "Alex Example" || prep.ProfileQuestions()[1].ProfileValue != profile.Email {
		t.Fatalf("Ashby preview namespace failed exact profile resolution: %+v", prep.ProfileQuestions())
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	if page.Code != 200 || strings.Count(body, `data-question-panel `) != 3 || !strings.Contains(body, "From your candidate profile") || !strings.Contains(body, "Complete on employer site") || !strings.Contains(body, "Needs manual review") || !strings.Contains(body, "Race and ethnicity") || !strings.Contains(body, "I certify that the information is accurate") || !strings.Contains(body, "Optional") || !strings.Contains(body, "Required") {
		t.Fatalf("read-only preview did not render useful categories and observed metadata: code=%d panels=%d", page.Code, strings.Count(body, `data-question-panel `))
	}
	for _, q := range prep.EmployerSiteQuestions() {
		if q.Field.Label == "Race and ethnicity" && !q.Field.Required && q.Revision != 0 && q.Value != "" {
			t.Fatal("sensitive employer field acquired an answer")
		}
	}
	// Profile availability does not create early-answer or live-answer rows.
	var early, live int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&early); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if early != 0 || live != 0 {
		t.Fatalf("presentation created answer records: early=%d live=%d", early, live)
	}

	// Saving an explicit draft on a profile-resolvable field keeps the draft and
	// puts it back in the action queue for review instead of replacing it.
	name := prep.ProfileQuestions()[0]
	if ok, reason := earlyQuestionWritable(name.Field, true); !ok {
		t.Fatalf("full name should remain explicitly editable in the early flow: %s", reason)
	}
	name.Editable = true
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, name, "0", "Alex E."), "", ""); rec.Code != 303 {
		t.Fatalf("save profile override draft: %d %s", rec.Code, rec.Body.String())
	}
	prep, err = a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	var saved *earlyApplicationQuestion
	for i := range prep.Questions {
		if prep.Questions[i].SourceQuestionID == name.SourceQuestionID {
			saved = &prep.Questions[i]
		}
	}
	if saved == nil || saved.Value != "Alex E." || saved.ProfileValue != "Alex Example" || !saved.NeedsAnswer() {
		t.Fatalf("saved draft was replaced or hidden: %+v", saved)
	}
	if len(prep.ActionableQuestions()) != 4 || len(prep.ProfileQuestions()) != 1 || prep.PreparedCount() != 1 {
		t.Fatalf("saved early draft did not remain available for review: action=%d profile=%d prepared=%d", len(prep.ActionableQuestions()), len(prep.ProfileQuestions()), prep.PreparedCount())
	}
}

func TestQuestionTriageLeavesMissingAndUnapprovedFactsUnresolved(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	result := schema.Result{Platform: "ashby", Source: "preview", ApplicationURL: testAshbyPosting, Fields: []schema.Field{
		{ID: "preview:name", Label: "Full Name", Kind: "text"},
		{ID: "preview:email", Label: "Email", Kind: "email"},
		{ID: "preview:contact-phone", Label: "Phone", Kind: "tel"},
		{ID: "preview:story-a", Label: "Describe a customer challenge", Kind: "textarea"},
		{ID: "preview:story-b", Label: "Describe a customer challenge", Kind: "textarea"},
	}}
	if _, err := a.saveInspection(1, result); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	for _, tc := range []struct {
		profile      CandidateProfile
		profileCount int
		actionCount  int
	}{{CandidateProfile{}, 0, 3}, {CandidateProfile{FirstName: "Alex", LastName: "Example", Email: "alex@example.test"}, 0, 3}, {CandidateProfile{Approved: true, FirstName: "Alex", LastName: "Example", Email: "alex@example.test"}, 2, 2}} {
		profile := tc.profile
		if err := a.saveCandidateProfile(profile); err != nil {
			t.Fatal(err)
		}
		prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
		if err != nil {
			t.Fatal(err)
		}
		if len(prep.ProfileQuestions()) != tc.profileCount || len(prep.ActionableQuestions()) != tc.actionCount {
			t.Fatalf("unapproved/missing profile was claimed or repeated labels collapsed: %+v", prep)
		}
		stories := 0
		for _, q := range prep.ActionableQuestions() {
			if q.Field.Label == "Describe a customer challenge" {
				stories++
			}
		}
		if stories != 2 {
			t.Fatalf("repeated labels with different identities collapsed: %+v", prep.ActionableQuestions())
		}
	}
}

func TestLiveQuestionTriageSeparatesActionEmployerAndReview(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	profile := CandidateProfile{Approved: true, FirstName: "Alex", LastName: "Example", Email: "alex@example.test"}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:name", Label: "Full Name", Kind: "text", Required: true},
		{ID: "browser:story-a", Label: "Describe a customer challenge", Kind: "textarea"},
		{ID: "browser:story-b", Label: "Describe a customer challenge", Kind: "textarea"},
		{ID: "browser:race", Label: "Race and ethnicity", Kind: "checkbox_group", Sensitive: true},
		{ID: "browser:attest", Label: "I agree to the terms", Kind: "checkbox"},
		{ID: "browser:widget", Label: "Choose a location on the map", Kind: "unsupported_custom"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	prep.selectQuestion(httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if len(prep.Questions) != len(fields) || len(prep.ActionableQuestions()) != 2 || len(prep.PreparedQuestions()) != 0 || len(prep.ProfileQuestions()) != 1 || len(prep.EmployerSiteQuestions()) != 2 || len(prep.ManualReviewQuestions()) != 1 {
		t.Fatalf("live triage counts total=%d action=%d profile=%d employer=%d review=%d", len(prep.Questions), len(prep.ActionableQuestions()), len(prep.ProfileQuestions()), len(prep.EmployerSiteQuestions()), len(prep.ManualReviewQuestions()))
	}
	if prep.SelectedKey != prep.ActionableQuestions()[0].Key {
		t.Fatal("initial live selection did not choose first actionable question")
	}
	if len(prep.ActionableQuestions()) != 2 || prep.ActionableQuestions()[0].Key == prep.ActionableQuestions()[1].Key {
		t.Fatal("repeated labels with distinct live identities were collapsed")
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	if page.Code != 200 || strings.Count(body, `data-question-panel `) != 2 || !strings.Contains(body, "Complete on employer site") || !strings.Contains(body, "Needs manual review") || !strings.Contains(body, "From your candidate profile") {
		t.Fatalf("live question presentation did not expose grouped triage: code=%d", page.Code)
	}
}
