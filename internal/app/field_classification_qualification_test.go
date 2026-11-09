//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestControlledFieldClassificationFill(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("controlled Chromium only")
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("fixture.pdf", strings.NewReader("%PDF-1.4\nsynthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	if err = a.saveCandidateProfile(CandidateProfile{FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test"}); err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><form class="ashby-application-form">
 <label>First name<input name="first_name"></label><label>Last name<input name="last_name" value="Employer entry"></label><label>Email<input name="email" type="email"></label><label>Phone<input name="phone" type="tel"></label>
 <label>Describe a production incident<textarea name="story"></textarea></label><label>Email<input name="supervisor_email" type="email" autocomplete="email"></label>
 <label>First name<textarea name="given_name"></textarea></label><label>Custom control<input name="custom" role="combobox"></label><label>Resume<input name="resume" type="file"></label><button type="submit">Submit</button></form><script>window.submits=0;document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	base := fixture.URL + "/fixture/job"
	chrome, err := chromium.LaunchFixture(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	session := &contactFixtureSession{Session: chrome, fixture: base, saved: testAshbyPosting}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return session, nil }
	a.browsers.pollEvery = 50 * time.Millisecond
	defer a.browsers.closeAll()
	if rec := postApp(newMux(a), "/jobs/1/browser/start", mapValues(), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	for deadline := time.Now().Add(8 * time.Second); a.browsers.state(1).LatestID == 0; {
		if time.Now().After(deadline) {
			t.Fatal("capture timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	var story preparedQuestion
	for _, q := range prep.Questions {
		if q.Field.ID == "browser:story" {
			story = q
		}
	}
	if story.Key == "" || len(prep.ProfileQuestions()) != 5 {
		t.Fatalf("capture/classification: %+v", prep)
	}
	rows, _ := unifyQuestionInventories(prep.Questions, nil, true)
	for _, row := range rows {
		if row.TriageCategory == "profile" {
			t.Fatal("profile in Questions")
		}
	}
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+story.Key+`","revision":0,"destination":"`+job.URL+`","answer":"I diagnosed the incident and restored service.","save_to_library":true}`)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("library: %+v %v", entries, err)
	}
	scope := continuationScope(t, a, 1)
	if len(scope["pendingAnswers"].([]any)) != 1 {
		t.Fatalf("overlapping scope: %+v", scope)
	}
	input := continueInput{Destination: job.URL, IncludeResume: true, ResumeID: strconv.FormatInt(resume.ID, 10), ResumeHash: resume.SHA256, TransferContacts: true, ProfileRevision: strconv.FormatInt(profile.Revision, 10), IncludeAnswers: true, ReviewHash: scope["preparation"].(map[string]any)["answerReviewHash"].(string)}
	raw, _ := json.Marshal(input)
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/continue", string(raw))
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var result struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	expression := `document.querySelector('[name=first_name]').value==='Alex'&&document.querySelector('[name=last_name]').value==='Employer entry'&&document.querySelector('[name=email]').value==='alex@example.test'&&document.querySelector('[name=phone]').value===''&&document.querySelector('[name=story]').value==='I diagnosed the incident and restored service.'&&document.querySelector('[name=supervisor_email]').value===''&&document.querySelector('[name=given_name]').value===''&&document.querySelector('[name=custom]').value===''&&submits===0`
	for deadline := time.Now().Add(12 * time.Second); ; {
		err = chrome.FixtureCall(ctx, base+"/application", "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &result)
		if err == nil && result.Result.Value {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("observed Fill mismatch: %v state=%+v response=%s", err, a.browsers.state(1), rec.Body.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	saved, _ := a.candidateProfile()
	if saved.Approved {
		t.Fatal("Fill changed saved approval")
	}
	t.Log("Managed capture → Go Questions projection → prepare/save to library → Apply API explicit Fill: unapproved snapshot filled first name/email; existing last name protected; missing phone, contact textarea, misleading email and unsupported control untouched; narrative filled; zero submissions; saved approval unchanged.")
}
