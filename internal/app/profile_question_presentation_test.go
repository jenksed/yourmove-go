package app

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestProfileQuestionsLeaveNavigatorWithoutChangingTransferScope(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		name := "simple"
		if namespaced {
			name = "observed-ashby"
		}
		t.Run(name, func(t *testing.T) { testProfileQuestionPresentation(t, namespaced) })
	}
}

func testProfileQuestionPresentation(t *testing.T, namespaced bool) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	profile := CandidateProfile{Approved: true, FirstName: "Alex", LastName: "Example", Email: "alex@example.test", Phone: "5551231234", LinkedIn: "https://linkedin.example.test"}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:name", Label: "Name", Kind: "text"},
		{ID: "browser:email", Label: "Email", Kind: "email"},
		{ID: "browser:phone", Label: "Phone", Kind: "tel"},
		{ID: "browser:linkedin", Label: "LinkedIn", Kind: "url"},
		{ID: "browser:motivation", Label: "Why this team?", Kind: "textarea"},
	}
	if namespaced {
		const prefix = "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26_"
		fields[0].ID = prefix + "_systemfield_name"
		fields[1].ID = prefix + "_systemfield_email"
		fields[2].ID = prefix + "106ba651-5f51-4048-af0a-ef3195a66c55"
		fields[3].ID = prefix + "d06b8fc6-34c2-4815-bb88-e4d9fd035a61"
		fields[3].Label, fields[3].Kind = "LinkedIn Profile Link", "text"
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	p, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	hash := p.ReviewHash()
	if len(p.ProfileQuestions()) != 4 || len(p.EmployerQuestions()) != 1 {
		t.Fatalf("profile=%d employer=%d", len(p.ProfileQuestions()), len(p.EmployerQuestions()))
	}
	p.selectQuestion(httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if p.SelectedKey != p.Questions[4].Key || hash != p.ReviewHash() || len(p.Questions) != 5 {
		t.Fatal("presentation changed authority or selected a profile fact")
	}
	mux := newMux(a)
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	body := page.Body.String()
	if page.Code != 200 || !strings.Contains(body, "From your candidate profile") || strings.Count(body, "data-question-panel ") != 1 {
		t.Fatal("profile fields remain in question navigator", page.Code)
	}
	// A profile-only form still exposes review and overrides without question panels.
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields[:4]}); err != nil {
		t.Fatal(err)
	}
	only := httptest.NewRecorder()
	mux.ServeHTTP(only, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if only.Code != 200 || strings.Contains(only.Body.String(), "data-question-panel ") || !strings.Contains(only.Body.String(), "No additional answers to prepare here") || !strings.Contains(only.Body.String(), "From your candidate profile") {
		t.Fatal("profile-only form lost review or retained question panels")
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	// The disclosure retains the real POST override; saving moves that answer
	// back into the navigator and does not alter the candidate profile.
	q := p.Questions[1]
	r := postApp(mux, "/jobs/1/answers/save", url.Values{"question_key": {q.Key}, "revision": {"0"}, "destination": {job.URL}, "answer": {"other@example.test"}}, "", "")
	if r.Code != 303 {
		t.Fatal(r.Code, r.Body.String())
	}
	p, _ = a.prepareQuestions(job, profile)
	if len(p.ProfileQuestions()) != 3 || len(p.EmployerQuestions()) != 2 || p.Questions[1].Value != "other@example.test" {
		t.Fatal("override lost")
	}
	saved, _ := a.candidateProfile()
	if saved.Email != profile.Email {
		t.Fatal("override changed profile")
	}
	// Missing/unapproved details and manual controls must remain visible.
	for _, draft := range []CandidateProfile{{}, {Approved: true}} {
		pending, _ := a.prepareQuestions(job, draft)
		if len(pending.ProfileQuestions()) != 0 || len(pending.EmployerQuestions()) != 5 {
			t.Fatal("missing or unapproved profile facts concealed")
		}
	}
	manual := questionPreparation{Questions: []preparedQuestion{{Key: "manual", Value: "value", Source: "Approved candidate information"}}}
	if len(manual.ProfileQuestions()) != 0 || len(manual.EmployerQuestions()) != 0 || len(manual.ManualReviewQuestions()) != 1 {
		t.Fatal("manual control concealed")
	}
}

func TestNamespacedProfileMatchingRejectsUnrelatedFields(t *testing.T) {
	p := CandidateProfile{Approved: true, FirstName: "Alex", Email: "alex@example.test", Phone: "5551231234", LinkedIn: "https://linkedin.example.test"}
	for _, f := range []schema.Field{
		{ID: "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26__systemfield_name", Label: "Manager name", Kind: "text"},
		{ID: "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26_106ba651-5f51-4048-af0a-ef3195a66c55", Label: "Reference phone", Kind: "tel"},
		{ID: "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26_d06b8fc6-34c2-4815-bb88-e4d9fd035a61", Label: "Manager LinkedIn Profile Link", Kind: "text"},
		{ID: "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26_d06b8fc6-34c2-4815-bb88-e4d9fd035a61", Label: "LinkedIn Profile Link", Kind: "text", Sensitive: true},
		{ID: "browser:unidentified-0", Label: "LinkedIn Profile Link", Kind: "text"},
	} {
		if value, _ := resolveApplicationAnswer(f, "", false, p, "", false); value != "" {
			t.Fatalf("unrelated field populated: %+v", f)
		}
	}
}
