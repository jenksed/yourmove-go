//go:build qualification

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"example.com/yourmove-lite/internal/application/browser/chromium"
)

func TestControlledIndependentCoverLetter(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("isolated rendered letter workflow")
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Independent letter fixture", Description: "Resolve production support incidents"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Verified support experience")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Verified troubleshooting experience")))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	if err = a.saveCandidateProfile(CandidateProfile{Approved: true, FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test"}); err != nil {
		t.Fatal(err)
	}
	const body = "Dear Hiring Team, my verified technical support experience is relevant to this role. I value careful customer communication."
	var generationCalls atomic.Int64
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		if generationCalls.Add(1) == 1 {
			return "", errCoverOutput
		}
		return body, nil
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><form class="ashby-application-form">
<label>Name<input name="_systemfield_name"></label><label>Email<input name="email" type="email"></label>
<section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".docx"></section>
<label>Why this team?<textarea name="why">Employer manual narrative</textarea></label>
<label>Cover Letter<textarea name="cover_letter"></textarea></label><button type="submit">Submit</button></form>
<script>window.uploads=0;window.submits=0;document.querySelector('[name=resume]').onchange=()=>uploads++;document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
	defer cancel()
	employer, err := chromium.LaunchFixture(ctx, fixture.URL+"/fixture/job")
	if err != nil {
		t.Fatal(err)
	}
	defer employer.Close()
	employerPage := fixture.URL + "/fixture/job/application"
	fs := &contactFixtureSession{Session: employer, fixture: fixture.URL + "/fixture/job", saved: testAshbyPosting}
	a.browsers.pollEvery = 80 * time.Millisecond
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fs, nil }
	defer a.browsers.closeAll()
	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1#prepare", 303) })
	router.Handle("/", newMux(a))
	app := httptest.NewServer(router)
	defer app.Close()
	ui, err := chromium.LaunchFixture(ctx, app.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Close()
	page := app.URL + "/jobs/1"
	eval := func(chrome *chromium.Session, target, script string, out any) error {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, target, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &reply); err != nil {
			return err
		}
		if len(reply.Exception) > 0 {
			return fmt.Errorf("JS: %s", reply.Exception)
		}
		if out != nil {
			return json.Unmarshal(reply.Result.Value, out)
		}
		return nil
	}
	run := func(script string) {
		t.Helper()
		if err := eval(ui, page, script, nil); err != nil {
			t.Fatal(err)
		}
	}
	checkEmployer := func(script string) {
		t.Helper()
		var ok bool
		if err := eval(employer, employerPage, script, &ok); err != nil || !ok {
			t.Fatalf("employer assertion: %s: %v", script, err)
		}
	}
	wait := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; {
			var ok bool
			if eval(ui, page, script, &ok) == nil && ok {
				return
			}
			if time.Now().After(deadline) {
				var text string
				_ = eval(ui, page, `document.body.innerText`, &text)
				t.Fatalf("timeout %s: %s", script, text)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	wait(`document.readyState==='complete'&&!!document.querySelector('form[action$="/continue"]')`)
	run(`document.querySelector('form[action$="/continue"] button').click()`)
	for deadline := time.Now().Add(10 * time.Second); ; {
		s := a.browsers.state(1)
		if s.ResumeStatus == "selected" && s.ContactStatus == "complete" && s.LatestID > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("initial fill: %+v", s)
		}
		time.Sleep(50 * time.Millisecond)
	}
	wait(`document.querySelector('.profile-question-summary')?.textContent.includes('Casey Fixture')&&document.querySelector('.profile-question-summary')?.textContent.includes('casey@example.test')`)
	checkEmployer(`document.querySelector('[name=_systemfield_name]').value==='Casey Fixture'&&document.querySelector('[name=email]').value==='casey@example.test'&&uploads===1&&submits===0`)
	wait(`document.querySelector('.profile-question-summary summary')?.textContent.includes('2 ready')`)
	// Reproduce the user's action with an unresolved question and a dirty editor.
	run(`(()=>{window.confirm=()=>{throw Error('Independent generation prompted about questions')};const q=document.querySelector('[data-question-label="Why this team?"]');const e=q.querySelector('textarea');e.value='Unsaved local narrative';e.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('#letter-editor').open=true;const f=document.querySelector('[data-ai-generate]');f.querySelector('[name=confirm_external]').checked=true;f.querySelector('button').click()})()`)
	page = app.URL + "/jobs/1?cover=output"
	wait(`document.querySelector('.letter-notice')?.textContent.includes('no usable letter')&&document.querySelector('[data-question-label="Why this team?"] textarea').value==='Unsaved local narrative'&&document.querySelector('#letter-editor').getBoundingClientRect().top>=0&&document.querySelector('#letter-editor').getBoundingClientRect().top<innerHeight`)
	run(`window.confirm=()=>{throw Error('Generation prompted')};document.querySelector('[data-ai-generate] [name=confirm_external]').checked=true;document.querySelector('[data-ai-generate] button').click()`)
	page = app.URL + "/jobs/1?cover=generated"
	wait(`document.querySelector('#letter-body')?.value===` + fmt.Sprintf("%q", body) + `&&document.querySelector('[data-question-label="Why this team?"] textarea').value==='Unsaved local narrative'&&!!document.querySelector('.saved-letter-transfer')`)
	capture := func(name, selector string) {
		t.Helper()
		evidence := "/private/tmp/yourmove-letter-independence-evidence"
		if err := os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{1100, 390} {
			shot, err := ui.FixtureScreenshotAt(ctx, page, width, selector, "light")
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(shot)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(evidence, fmt.Sprintf("%s-%d.png", name, width)), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	capture("ready-transfer", ".saved-letter-transfer")
	if generationCalls.Load() != 2 {
		t.Fatal("generation was blocked or duplicated")
	}
	run(`(()=>{window.confirm=()=>{throw Error('Letter transfer prompted about questions')};const e=document.querySelector('#letter-body');e.value='Unsaved letter edits that must remain local and must never be transferred.';e.dispatchEvent(new Event('input',{bubbles:true}));const f=document.querySelector('.saved-letter-transfer');f.querySelector('[name=mode]').value='text';f.querySelector('button').click()})()`)
	page = app.URL + "/jobs/1?cover=transferring&draft=1"
	wait(`document.querySelector('#letter-body')?.value.startsWith('Unsaved letter edits')&&document.querySelector('[data-question-label="Why this team?"] textarea').value==='Unsaved local narrative'`)
	for deadline := time.Now().Add(10 * time.Second); a.browsers.state(1).LetterStatus != "complete"; {
		if time.Now().After(deadline) {
			t.Fatalf("letter: %+v", a.browsers.state(1))
		}
		time.Sleep(50 * time.Millisecond)
	}
	checkEmployer(`document.querySelector('[name=cover_letter]').value===` + fmt.Sprintf("%q", body) + `&&document.querySelector('[name=why]').value==='Employer manual narrative'&&document.querySelector('[name=_systemfield_name]').value==='Casey Fixture'&&uploads===1&&submits===0`)
	wait(`!!document.querySelector('[data-new-questions] a')`)
	run(`window.confirm=()=>{throw Error('Results refresh prompted')};document.querySelector('[data-new-questions] a').click()`)
	wait(`document.body.textContent.includes('Cover letter · complete')&&document.querySelector('#letter-body').value.startsWith('Unsaved letter edits')&&document.querySelector('[data-question-label="Why this team?"] textarea').value==='Unsaved local narrative'`)
	if a.browsers.sessions[1] != fs {
		t.Fatal("browser session replaced")
	}
	run(`document.querySelector('#letter-editor').open=true`)
	capture("saved-letter", "#letter-editor")
	// A concurrent saved revision must win over restored unsaved text.
	const newer = "A newer saved letter revision must remain authoritative while unsaved edits are recovered separately."
	if err := a.saveCoverLetterBody(1, 1, newer); err != nil {
		t.Fatal(err)
	}
	run(`window.confirm=()=>{throw Error('Recheck prompted')};document.querySelector('form[action$="/browser/observe"] button').click()`)
	page = app.URL + "/jobs/1"
	wait(`document.querySelector('#letter-body')?.value===` + fmt.Sprintf("%q", newer) + `&&document.querySelector('[data-answer-recovery] textarea')?.value.startsWith('Unsaved letter edits')&&document.querySelector('[data-question-label="Why this team?"] textarea').value==='Unsaved local narrative'`)
	checkEmployer(`document.querySelector('[name=cover_letter]').value===` + fmt.Sprintf("%q", body) + `&&uploads===1&&submits===0`)
	t.Log("Approved Name/email filled; generation failure stayed at letter error, retry produced draft despite dirty unresolved question; direct text transfer used saved body, preserved both unsaved editors, manual employer narrative and original session; results refreshed without discard prompt; one resume selection, zero employer submissions")
}
