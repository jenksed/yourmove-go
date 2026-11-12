//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// Restores the missing applicant-facing behavior: supported, approved
// Candidate Profile values for LinkedIn, Location, and Pronouns transfer
// through the React Apply → Fill → Go API → managed Chromium path.
//
// Synthetic Ashby form contains: LinkedIn URL input, Location combobox with
// autocomplete, supported Pronouns select, one ordinary saved answer,
// résumé + cover-letter file inputs, one employer-only control (manual), and
// a pre-filled contact value that must be preserved. Zero submissions.
func TestControlledProfileFactFill(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("controlled Chromium only")
	}
	a, _ := testWorkspace(t)
	requireQualificationStorage(t, a)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic profile-fact role", Description: "Resolve production incidents with clear customer communication"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Managed production incidents"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("role.docx", strings.NewReader(string(sampleDOCXForPreview(t, "Technical support experience"))))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	// Approved profile carries contacts plus the three restored facts.
	profile := CandidateProfile{
		FirstName: "Alex", LastName: "Fixture", Email: "alex@example.test", Phone: "202-555-0104",
		City: "Cadillac", Region: "Michigan", Country: "United States",
		LinkedIn: "https://www.linkedin.com/in/fixture", Pronouns: "He/him", Approved: true,
	}
	if err := a.saveCandidateProfile(profile); err != nil {
		t.Fatal(err)
	}
	savedProfile, err := a.candidateProfile()
	if err != nil {
		t.Fatal(err)
	}
	if !savedProfile.Approved {
		t.Fatal("profile not approved")
	}

	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><div class="ashby-application-form-container"><form class="ashby-application-form">
  <label>First name<input name="first_name"></label><label>Last name<input name="last_name" value="Employer entry"></label><label>Email<input name="email" type="email"></label><label>Phone<input name="phone" type="tel"></label>
  <label>LinkedIn URL<input name="linkedin" type="url"></label>
  <label>Location<input name="location" role="combobox" aria-controls="places" required></label>
  <div id="places" role="listbox" hidden><div role="option">Cadillac, Michigan, United States</div></div>
  <label>Pronouns<select name="pronouns"><option value="">Select…</option><option>He/him</option><option>She/her</option><option>They/them</option></select></label>
  <label>Why this team?<textarea name="motivation"></textarea></label>
  <label>Work authorization<input name="sponsorship"></label>
  <section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".pdf"></section>
  <section data-field-entry-id="cover"><div class="ashby-application-form-question-title">Cover Letter</div><input type="file" name="cover" accept=".pdf"></section>
  <button type="submit">Submit application</button></form></div><script>
  window.submits=0;document.querySelector('form').onsubmit=e=>{e.preventDefault();window.submits++};
  const locationInput=document.querySelector('[name=location]');
  locationInput.oninput=()=>{document.getElementById('places').hidden=false;locationInput.setCustomValidity('Select a location')};
  document.querySelector('[role=option]').onclick=e=>{const o=e.currentTarget;o.setAttribute('aria-selected','true');locationInput.value='Cadillac, Michigan, United States';locationInput.setCustomValidity('');document.getElementById('places').hidden=true};
  </script>`)
	}))
	defer fixture.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	// React path: open the managed browser (no saved material sent).
	rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/browser", `{"action":"start"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("browser start: %d %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(10 * time.Second)
	for a.browsers.state(1).LatestID == 0 {
		if time.Now().After(deadline) {
			t.Fatal("capture timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	job, _ := a.jobByID(1)
	liveProfile, _ := a.candidateProfile()
	prep, err := a.prepareQuestions(job, liveProfile)
	if err != nil {
		t.Fatal(err)
	}
	var motivationKey string
	for _, q := range prep.Questions {
		if q.Field.Label == "Why this team?" {
			motivationKey = q.Key
		}
	}
	if motivationKey == "" {
		t.Fatalf("ordinary question not captured: %+v", prep.Questions)
	}
	// One ordinary saved application answer.
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+motivationKey+`","revision":0,"destination":"`+job.URL+`","answer":"I enjoy resolving support incidents with clear follow-up."}`)
	if rec.Code != 200 {
		t.Fatalf("save answer: %d %s", rec.Code, rec.Body.String())
	}
	// Cover-letter draft for the file handoff.
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		return "Dear Hiring Manager,\n\nI resolve production incidents with care and clear customer communication.\n\nSincerely,\nAlex Fixture", nil
	}
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/letters", `{"provider":"ollama","model":"","letter_resume_id":0,"career_evidence":[],"confirm_local":"yes"}`)
	if rec.Code != 200 {
		t.Fatalf("letter generate: %d %s", rec.Code, rec.Body.String())
	}
	draft, err := a.latestCoverLetter(1)
	if err != nil {
		t.Fatal(err)
	}

	// Exact approved scope before transfer (React continuation API).
	scopeRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(scopeRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/jobs/1/continuation", nil))
	if scopeRec.Code != http.StatusOK {
		t.Fatalf("continuation: %d %s", scopeRec.Code, scopeRec.Body.String())
	}
	var scope map[string]any
	if err := json.Unmarshal(scopeRec.Body.Bytes(), &scope); err != nil {
		t.Fatal(err)
	}
	pending, _ := scope["pendingAnswers"].([]any)
	byLabel := map[string]map[string]any{}
	for _, item := range pending {
		m := item.(map[string]any)
		byLabel[m["label"].(string)] = m
	}
	// The three restored facts must be visible with approved values.
	for _, tc := range []struct{ label, value string }{
		{"LinkedIn URL", "https://www.linkedin.com/in/fixture"},
		{"Location", "Cadillac, Michigan, United States"},
		{"Pronouns", "He/him"},
		{"Why this team?", "I enjoy resolving support incidents with clear follow-up."},
	} {
		got, ok := byLabel[tc.label]
		if !ok {
			t.Fatalf("approved scope missing %q: %v", tc.label, byLabel)
		}
		if got["value"] != tc.value {
			t.Fatalf("scope %q value=%q want %q", tc.label, got["value"], tc.value)
		}
		if tc.label != "Why this team?" && got["source"] != "Approved candidate information" {
			t.Fatalf("scope %q source=%q want approved profile", tc.label, got["source"])
		}
	}
	prepMap := scope["preparation"].(map[string]any)
	if prepMap["includeAnswers"] != true {
		t.Fatalf("Fill not offered for profile facts: %v", prepMap)
	}
	reviewHash := prepMap["answerReviewHash"].(string)

	// Explicitly authorize the displayed scope (résumé + contacts + letter + answers/facts).
	savedResume, _ := a.resumeByID(resume.ID)
	payload, _ := json.Marshal(continueInput{
		Destination: job.URL, IncludeResume: true, ResumeID: strconv.FormatInt(savedResume.ID, 10), ResumeHash: savedResume.SHA256,
		TransferContacts: true, ProfileRevision: strconv.FormatInt(savedProfile.Revision, 10),
		IncludeLetter: true, DraftID: strconv.FormatInt(draft.ID, 10), BodyHash: draft.ContentHash(), Mode: "file",
		IncludeAnswers: true, ReviewHash: reviewHash,
	})
	rec = postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/continue", string(payload))
	if rec.Code != 200 {
		t.Fatalf("continue: %d %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Items []continueItemResult `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	byItem := map[string]continueItemResult{}
	for _, item := range result.Items {
		byItem[item.Item] = item
	}
	for _, name := range []string{"resume", "letter", "answers"} {
		got, ok := byItem[name]
		if !ok || !got.OK {
			t.Fatalf("item %q not authorized: %+v", name, result.Items)
		}
	}
	// Contacts travel with the résumé handoff (no separate item when combined).

	// Actual employer form values after transfer.
	var observed struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	expression := `document.querySelector('[name=first_name]').value==='Alex'&&document.querySelector('[name=last_name]').value==='Employer entry'&&document.querySelector('[name=email]').value==='alex@example.test'&&document.querySelector('[name=phone]').value==='202-555-0104'&&document.querySelector('[name=linkedin]').value==='https://www.linkedin.com/in/fixture'&&document.querySelector('[name=location]').value==='Cadillac, Michigan, United States'&&document.querySelector('[name=location]').validity.valid&&document.querySelector('[name=pronouns]').value==='He/him'&&document.querySelector('[name=motivation]').value==='I enjoy resolving support incidents with clear follow-up.'&&document.querySelector('[name=sponsorship]').value===''&&submits===0`
	hit := false
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		if err := chrome.FixtureCall(ctx, base+"/application", "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &observed); err == nil && observed.Result.Value {
			hit = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !hit {
		t.Fatalf("observed Fill mismatch state=%+v body=%s", a.browsers.state(1), rec.Body.String())
	}
	// Per-item outcomes: contacts + answers/facts must report filled/correct,
	// never a silent overwrite, and the employer-only control stays manual.
	state := a.browsers.state(1)
	found := map[string]string{}
	for _, o := range state.ContactOutcomes {
		found[o.Key] = o.Status
	}
	// Pre-filled last name must be preserved (manual/correct, never overwritten).
	if status := found["last_name"]; status != "manual" && status != "correct" {
		t.Fatalf("last_name outcome=%q want manual/correct (preserved Employer entry)", status)
	}
	answerStatus := map[string]string{}
	for _, o := range state.AnswerOutcomes {
		answerStatus[o.Label] = o.Status
	}
	for _, label := range []string{"LinkedIn URL", "Location", "Pronouns", "Why this team?"} {
		if status := answerStatus[label]; status != "filled" && status != "correct" {
			t.Fatalf("answer %q outcome=%q want filled/correct outcomes=%+v", label, status, state.AnswerOutcomes)
		}
	}
	// Unsupported employer-only control never enters Fill scope.
	for _, o := range state.AnswerOutcomes {
		if o.Label == "Work authorization" {
			t.Fatalf("employer-only control entered Fill: %+v", o)
		}
	}
	// No employer submission and saved approval unchanged.
	var submits struct {
		Result struct {
			Value int `json:"value"`
		} `json:"result"`
	}
	if err := chrome.FixtureCall(ctx, base+"/application", "Runtime.evaluate", map[string]any{"expression": "window.submits", "returnByValue": true}, &submits); err != nil || submits.Result.Value != 0 {
		t.Fatalf("employer submission occurred: %+v %v", submits, err)
	}
	if stored, _ := a.jobByID(1); stored.Stage == StageApplied || stored.SubmissionCount != 0 {
		t.Fatalf("transfer recorded a submission: %+v", stored)
	}
	after, _ := a.candidateProfile()
	if !after.Approved || after.Revision != savedProfile.Revision {
		t.Fatalf("Fill changed saved approval: %+v", after)
	}
	t.Log("React Apply → Fill → Go API → managed Chromium: LinkedIn, Location (Ashby autocomplete selected), and Pronouns filled from approved profile; ordinary answer filled; résumé/cover attached; existing last name preserved; Work authorization left manual; zero submissions.")
}
