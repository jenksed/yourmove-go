package main

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"example.com/yourmove-lite/internal/application/schema"
)

func TestExactEmployerAnswerReuseAndLocalOverride(t *testing.T) {
	a, _ := testWorkspace(t)
	urls := []string{testAshbyPosting, strings.Replace(testAshbyPosting, "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 1), strings.Replace(testAshbyPosting, "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 1), strings.Replace(testAshbyPosting, "/render/", "/other-employer/", 1)}
	field := schema.Field{ID: "browser:question-stable-id", Label: "Which support tools have you used?", Kind: "textarea"}
	for _, u := range urls {
		if err := a.saveJob(Job{URL: u, Host: "jobs.ashbyhq.com"}); err != nil {
			t.Fatal(err)
		}
	}
	for id, u := range urls {
		if _, err := a.saveInspection(int64(id+1), schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: u + "/application", Fields: []schema.Field{field}}); err != nil {
			t.Fatal(err)
		}
	}
	save := func(id int64, value string, revision int64, reuse bool) int {
		t.Helper()
		job, _ := a.jobByID(id)
		p, err := a.prepareQuestions(job, CandidateProfile{})
		if err != nil {
			t.Fatal(err)
		}
		q := p.Questions[0]
		values := url.Values{"destination": {job.URL}, "question_key": {q.Key}, "revision": {fmt.Sprint(revision)}, "answer": {value}, "reusable_revision": {fmt.Sprint(q.ReusableRevision)}}
		if reuse {
			values.Set("reusable", "yes")
		}
		return postApp(newMux(a), fmt.Sprintf("/jobs/%d/answers/save", id), values, "", "").Code
	}
	const original = "SSH, incident notes, and Linux troubleshooting tools."
	if code := save(1, original, 0, true); code != 303 {
		t.Fatal(code)
	}
	job2, _ := a.jobByID(2)
	p2, err := a.prepareQuestions(job2, CandidateProfile{})
	if err != nil {
		t.Fatal(err)
	}
	if p2.Questions[0].Value != original || !strings.Contains(p2.Questions[0].Source, "Reusable") || p2.Questions[0].ReusableRevision != 1 {
		t.Fatal("qualified question did not reuse the approved answer")
	}
	if code := save(2, "Specific experience for this application only.", 0, false); code != 303 {
		t.Fatal(code)
	}
	job3, _ := a.jobByID(3)
	p3, _ := a.prepareQuestions(job3, CandidateProfile{})
	if p3.Questions[0].Value != original {
		t.Fatal("application-specific override modified reusable original")
	}
	job4, _ := a.jobByID(4)
	p4, _ := a.prepareQuestions(job4, CandidateProfile{})
	if p4.Questions[0].Value != "" {
		t.Fatal("answer crossed employer scope")
	}
	if code := save(1, "An explicitly updated reusable answer for this employer.", 1, true); code != 303 {
		t.Fatal(code)
	}
	p3, _ = a.prepareQuestions(job3, CandidateProfile{})
	if p3.Questions[0].ReusableRevision != 2 || p3.Questions[0].Value == original {
		t.Fatal("explicit reusable update was not revisioned")
	}
	p2, _ = a.prepareQuestions(job2, CandidateProfile{})
	if p2.Questions[0].Value != "Specific experience for this application only." {
		t.Fatal("reusable revision overrode application-specific work")
	}
	// Matching labels with different provider identities never inherit memory.
	different := field
	different.ID = "browser:another-question"
	if reusableAnswerKey(urls[2], different) == reusableAnswerKey(urls[2], field) {
		t.Fatal("label-only memory match")
	}
	different = field
	different.Sensitive = true
	if reusableAnswerKey(urls[0], different) != "" {
		t.Fatal("sensitive answer became reusable")
	}
}
