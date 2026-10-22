package main

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestApplicationAnswerPersistenceAndQuestionIdentity(t *testing.T) {
	for _, tc := range []struct{ posting, platform, suffix string }{
		{testAshbyPosting, "ashby", "/application"},
		{"https://job-boards.greenhouse.io/cordance/jobs/5423762008", "greenhouse", "#application"},
		{"https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "lever", "/apply"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			posting, platform := tc.posting, tc.platform
			application := posting + tc.suffix
			a, _ := testWorkspace(t)
			if err := a.saveJob(Job{URL: posting, Host: "jobs.ashbyhq.com"}); err != nil {
				t.Fatal(err)
			}
			field := schema.Field{ID: "browser:motivation-uuid", Label: "Why would you like to join this team?", Kind: "textarea", Required: true}
			observation := schema.Result{Platform: platform, Source: "browser", Completeness: "rendered_partial", ApplicationURL: application, Fields: []schema.Field{field}}
			if _, err := a.saveInspection(1, observation); err != nil {
				t.Fatal(err)
			}
			job, _ := a.jobByID(1)
			prepared, err := a.prepareQuestions(job, CandidateProfile{})
			if err != nil {
				t.Fatal(err)
			}
			q := prepared.Questions[0]
			answer := "I enjoy helping people resolve difficult technical problems."
			values := url.Values{"question_key": {q.Key}, "revision": {"0"}, "answer": {answer}, "destination": {job.URL}}
			mux := newMux(a)
			if rec := postApp(mux, "/jobs/1/answers/save", values, "", ""); rec.Code != 303 {
				t.Fatal(rec.Code, rec.Body.String())
			}
			// Leaving and returning reads SQLite; there is no in-memory answer cache.
			page := httptest.NewRecorder()
			mux.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
			if !strings.Contains(page.Body.String(), answer) {
				t.Fatal("saved answer missing on return")
			}
			if rec := postApp(mux, "/jobs/1/answers/save", values, "", ""); rec.Code != 409 {
				t.Fatal("stale editor overwrote saved work")
			}
			// An unrelated form change preserves the question and its answer.
			observation.Fields = append(observation.Fields, schema.Field{ID: "browser:site", Label: "Website", Kind: "url"})
			if _, err := a.saveInspection(1, observation); err != nil {
				t.Fatal(err)
			}
			prepared, err = a.prepareQuestions(job, CandidateProfile{})
			if err != nil {
				t.Fatal(err)
			}
			if prepared.Questions[0].Value != answer || prepared.Questions[0].Revision != 1 {
				t.Fatal("reinspection lost saved answer")
			}
			// Reusing an employer ID for a different question never inherits the answer.
			observation.Fields[0].Label = "Why did you leave your previous employer?"
			if _, err := a.saveInspection(1, observation); err != nil {
				t.Fatal(err)
			}
			prepared, err = a.prepareQuestions(job, CandidateProfile{})
			if err != nil {
				t.Fatal(err)
			}
			if prepared.Questions[0].Value != "" || len(prepared.Previous) != 1 || prepared.Previous[0].Value != answer {
				t.Fatal("answer silently moved or was destroyed")
			}
			if rec := postApp(mux, "/jobs/1/answers/save", values, "", ""); rec.Code != 409 {
				t.Fatal("old field still writable")
			}
			if _, err := a.deleteJobs([]int64{1}, false); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := a.db.QueryRow("SELECT count(*) FROM application_answers").Scan(&count); err != nil || count != 0 {
				t.Fatal("job deletion retained answers", err)
			}
		})
	}
}

func TestAnswerResolutionUsesQualifiedFactsAndExplicitOverrides(t *testing.T) {
	p := CandidateProfile{Approved: true, Email: "candidate@example.test"}
	for _, tc := range []struct {
		name     string
		f        schema.Field
		explicit string
		has      bool
		want     string
	}{
		{"qualified fact", schema.Field{ID: "browser:email", Kind: "email", Label: "Email"}, "", false, p.Email},
		{"different meaning", schema.Field{ID: "browser:email", Kind: "email", Label: "Manager email"}, "", false, ""},
		{"label alone", schema.Field{ID: "browser:other", Kind: "email", Label: "Email"}, "", false, ""},
		{"sensitive", schema.Field{ID: "browser:email", Kind: "email", Label: "Email", Sensitive: true}, "", false, ""},
		{"explicit override", schema.Field{ID: "browser:email", Kind: "email", Label: "Email"}, "application@example.test", true, "application@example.test"},
		{"explicit blank", schema.Field{ID: "browser:email", Kind: "email", Label: "Email"}, "", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := resolveApplicationAnswer(tc.f, tc.explicit, tc.has, p, "", false)
			if got != tc.want {
				t.Fatal("incorrect resolution")
			}
		})
	}
}
func TestUnstableSensitiveAndAmbiguousQuestionsStayManual(t *testing.T) {
	for _, f := range []schema.Field{
		{ID: "browser:unidentified-1", Kind: "text", Label: "Narrative"},
		{ID: "browser:legal", Kind: "text", Label: "Work authorization", Sensitive: true},
		{ID: "browser:cover", Kind: "textarea", Label: "Cover letter"},
		{ID: "browser:custom", Kind: "combobox", Label: "Preference"},
	} {
		if editableQuestion(f, true) {
			t.Fatal("unsafe question editor", f)
		}
	}
	if editableQuestion(schema.Field{ID: "browser:shared", Kind: "text"}, false) {
		t.Fatal("ambiguous identity accepted")
	}
	// Options, sensitivity, and required semantics are part of question identity.
	f := schema.Field{ID: "browser:question", Kind: "select", Label: "Preference", Options: []string{"One", "Two"}}
	original := questionKey(testAshbyPosting, "ashby", f)
	for _, change := range []int{0, 1, 2} {
		copy := f
		switch change {
		case 0:
			copy.Options = []string{"One", "Other"}
		case 1:
			copy.Sensitive = true
		case 2:
			copy.Required = true
		}
		if questionKey(testAshbyPosting, "ashby", copy) == original {
			t.Fatal(fmt.Sprint("identity omitted semantics ", change))
		}
	}
}
