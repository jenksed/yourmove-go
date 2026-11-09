package app

import (
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"net/http/httptest"
	"testing"
)

func TestProfileRoleStableAcrossValuesApprovalAndHistory(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{{ID: "browser:email", Label: "Email", Kind: "email"}, {ID: "browser:first_name", Label: "First name", Kind: "textarea"}, {ID: "browser:story", Label: "Describe a production incident", Kind: "textarea"}, {ID: "browser:supervisor_email", Label: "Email", Kind: "email"}}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	for _, f := range fields[:2] {
		if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(?,?,?,?,?,?,1)`, job.ID, job.URL, "ashby", questionKey(job.URL, "ashby", f), f.Label, "Historical user answer"); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []CandidateProfile{{}, {Approved: true}, {FirstName: "Alex", Email: "alex@example.test"}, {Approved: true, FirstName: "Alex", Email: "alex@example.test"}} {
		stored := p
		stored.Approved = false
		if err := a.saveCandidateProfile(stored); err != nil {
			t.Fatal(err)
		}
		prep, err := a.prepareQuestions(job, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(prep.ProfileQuestions()) != 2 || len(prep.ActionableQuestions()) != 2 || len(prep.ReadyAnswers()) != 0 || len(prep.Previous) != 2 {
			t.Fatalf("role changed: %+v", prep)
		}
		rows, _ := unifyQuestionInventories(prep.Questions, nil, true)
		if len(rows) != 2 {
			t.Fatalf("profile leaked into Questions: %+v", rows)
		}
		for _, q := range prep.ProfileQuestions() {
			if q.LibraryEligible || answerLibraryEligible(q.Field, true) || q.Editable {
				t.Fatal("profile gained answer authority")
			}
			for _, source := range []string{"", "Approved candidate information", "Saved for this application", libraryAutoSource} {
				q.Source = source
				q.Value = "different"
				q.Editable = true
				forced := questionPreparation{Questions: []preparedQuestion{q}}
				if len(forced.ReadyAnswers()) != 0 || len(forced.ActionableQuestions()) != 0 || len(pendingAnswers(forced.Questions, nil, false)) != 0 {
					t.Fatal("source changed ownership")
				}
			}
		}
	}
	early := fields[:2]
	for i := range early {
		early[i].ID = "preview:" + profileQuestionIdentity(early[i].ID)
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "preview", ApplicationURL: job.URL, Fields: early}); err != nil {
		t.Fatal(err)
	}
	prep, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "/jobs/1", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(prep.ProfileQuestions()) != 2 || len(prep.ActionableQuestions()) != 0 {
		t.Fatalf("early profile routing: %+v", prep)
	}
}
