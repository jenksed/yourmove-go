//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestControlledProfileCompletion(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("isolated Chromium journey")
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic profile completion"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{{ID: "browser:pronouns", Label: "Pronouns", Kind: "text"}}}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	router := http.NewServeMux()
	router.HandleFunc("GET /jobs/1/application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1#prepare", 303) })
	router.Handle("/", mux)
	app := httptest.NewServer(router)
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, app.URL+"/jobs/1")
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	page := app.URL + "/jobs/1"
	eval := func(url, script string, out any) error {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, url, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &reply); err != nil {
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
	run := func(url, script string) {
		t.Helper()
		if err := eval(url, script, nil); err != nil {
			t.Fatal(err)
		}
	}
	check := func(url, script string) {
		t.Helper()
		var ok bool
		if err := eval(url, script, &ok); err != nil || !ok {
			t.Fatalf("assertion %s: %v", script, err)
		}
	}
	wait := func(url, script string) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; {
			var ok bool
			if eval(url, script, &ok) == nil && ok {
				return
			}
			if time.Now().After(deadline) {
				var body string
				_ = eval(url, `document.body.innerText`, &body)
				t.Fatalf("timeout %s; state=%+v; body=%s", script, a.browsers.state(2), body)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	capture := func(name, selector string) {
		t.Helper()
		dir := "/private/tmp/yourmove-profile-completion-evidence"
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{1280, 390} {
			shot, err := chrome.FixtureScreenshotAt(ctx, page, width, selector, "light")
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(shot)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, fmt.Sprintf("%s-%d.png", name, width))
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			t.Log("screenshot", path)
		}
	}
	wait(page, `document.readyState==='complete'&&!!document.querySelector('[name=answer]')`)
	run(page, `(()=>{document.querySelector('[data-tab-target=prepare]').click();const e=document.querySelector('[name=answer]');e.value='Unsaved pronoun edit';e.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('#record-submission').open=true;document.querySelector('.submission-form [name=confirm]').checked=true;window.messages=[];window.confirm=m=>{messages.push(m);return false};document.querySelector('.submission-form button').click()})()`)
	if os.Getenv("YOURMOVE_COMPLETION_BASELINE") == "1" {
		check(page, `messages[0].includes('Leave and discard')&&document.activeElement===document.querySelector('[name=answer]')`)
		t.Log("Reproduced: completion POST cancelled by generic leave/discard prompt; focus jumps to dirty Pronouns editor; no submission recorded")
		return
	}
	check(page, `messages.length===1&&messages[0].includes('Record completion anyway')&&document.querySelector('[name=answer]').value==='Unsaved pronoun edit'&&document.activeElement!==document.querySelector('[name=answer]')`)
	capture("completion-cancel-preserves-edits", "#record-submission")
	run(page, `window.confirm=m=>{messages.push(m);return true};document.querySelector('.submission-form button').click()`)
	wait(page, `document.body.textContent.includes('Submission recorded')&&document.querySelectorAll('#history .timeline li').length===1`)
	job, _ := a.jobByID(1)
	if job.Stage != StageApplied || job.SubmissionCount != 1 {
		t.Fatalf("completion: %+v", job)
	}
	if len(a.browsers.answers) != 0 || len(a.browsers.handoffs) != 0 {
		t.Fatal("recording armed transfer")
	}
	t.Log("Completion: Cancel preserves local edit without question jump; explicit record-anyway confirmation creates Applied + one history record, without starting a browser operation")
	// Separate application: actual profile approval, normal Continue, live control readback.
	if err := a.saveJob(Job{URL: strings.Replace(testAshbyPosting, "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 1), Host: "jobs.ashbyhq.com", Title: "Synthetic profile transfer"}); err != nil {
		t.Fatal(err)
	}
	run(page, `location.href='/candidate'`)
	page = app.URL + "/candidate"
	wait(page, `!!document.querySelector('#pronouns')`)
	run(page, `(()=>{for(const [id,value] of Object.entries({first_name:'Casey',last_name:'Fixture',email:'casey@example.test',linkedin:'https://www.linkedin.com/in/synthetic-fixture',pronouns:'He/him'}))document.getElementById(id).value=value;document.querySelector('[name=approve]').checked=true;document.querySelector('form[action="/candidate"] button').click()})()`)
	wait(page, `document.body.textContent.includes('Approved · Revision 1')&&document.querySelector('#pronouns').value==='He/him'`)
	capture("approved-profile", ".field:has(#pronouns)")
	run(page, `location.reload()`)
	wait(page, `document.querySelector('#pronouns')?.value==='He/him'`)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><form class="ashby-application-form">
<label>Pronouns<select name="pronouns"><option value="">Choose</option><option>He/Him/His</option><option>She/Her/Hers</option></select></label>
<label>Preferred pronouns<select name="case-pronouns"><option value="">Choose</option><option>He/Him</option><option>They/Them</option></select></label>
<label>Preferred pronouns<select name="manual-pronouns"><option value="">Choose</option><option>He/Him</option><option selected>They/Them</option></select></label>
<label>What are your pronouns?<input name="text-pronouns"></label>
<label>Preferred pronouns<select name="ambiguous"><option value="">Choose</option><option>He/Him</option><option>He/Him/His</option></select></label>
<label>Pronouns<input name="custom" role="combobox"></label>
<label>Gender<input name="gender"></label>
<label>LinkedIn Profile Link<input name="ff5bb721-1230-4753-bb72-2f8324c0bf26_d06b8fc6-34c2-4815-bb88-e4d9fd035a61" type="text"></label>
<button type="submit">Submit application</button></form><script>window.events=0;window.submits=0;for(const type of ['input','change'])document.addEventListener(type,()=>events++);document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`)
	}))
	defer fixture.Close()
	base := fixture.URL + "/fixture/job"
	// LaunchFixture owns a private, loopback-only Chromium process; use that existing
	// test seam for employer interaction, never a live employer or the user's profile.
	employerChrome, err := chromium.LaunchFixture(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer employerChrome.Close()
	employer := base + "/application"
	// Observe through the production browser adapter and store its exact current definition.
	second, _ := a.jobByID(2)
	fs := &contactFixtureSession{Session: employerChrome, fixture: base, saved: second.URL}
	a.browsers.pollEvery = 80 * time.Millisecond
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fs, nil }
	defer a.browsers.closeAll()
	if rec := postApp(mux, "/jobs/2/browser/start", mapValues(), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	for deadline := time.Now().Add(10 * time.Second); a.browsers.state(2).LatestID == 0; {
		if time.Now().After(deadline) {
			t.Fatal("initial employer observation timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	run(page, `location.href='/jobs/2#prepare'`)
	page = app.URL + "/jobs/2"
	wait(page, `!!document.querySelector('form[action$="/continue"] [name=include_answers]')`)
	profile, _ := a.candidateProfile()
	prepared, err := a.prepareQuestions(second, profile)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, q := range prepared.Questions {
		values[q.Field.ID] = q.Value
	}
	for id, want := range map[string]string{"browser:pronouns": "He/Him/His", "browser:case-pronouns": "He/Him", "browser:manual-pronouns": "He/Him", "browser:text-pronouns": "He/him", "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26_d06b8fc6-34c2-4815-bb88-e4d9fd035a61": profile.LinkedIn, "browser:ambiguous": "", "browser:custom": "", "browser:gender": ""} {
		if values[id] != want {
			t.Fatalf("%s=%q want %q", id, values[id], want)
		}
	}
	check(page, `document.querySelector('.pending-answer-scope').textContent.includes('Approved candidate profile')&&document.querySelector('.pending-answer-scope').textContent.includes('He/Him/His')`)
	capture("prepared-profile-facts", "#application-action")
	run(page, `document.querySelector('form[action$="/continue"] button').click()`)
	wait(page, `document.body.textContent.includes('Filled and observed in Chromium')`)
	for deadline := time.Now().Add(10 * time.Second); a.browsers.state(2).AnswerStatus != "complete"; {
		if time.Now().After(deadline) {
			t.Fatal("answer transfer timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	var reply struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	if err := employerChrome.FixtureCall(ctx, employer, "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `document.querySelector('[name=pronouns]').value==='He/Him/His'&&document.querySelector('[name=case-pronouns]').value==='He/Him'&&document.querySelector('[name=manual-pronouns]').value==='They/Them'&&document.querySelector('[name=text-pronouns]').value==='He/him'&&document.querySelector('[name=ff5bb721-1230-4753-bb72-2f8324c0bf26_d06b8fc6-34c2-4815-bb88-e4d9fd035a61]').value==='https://www.linkedin.com/in/synthetic-fixture'&&['ambiguous','custom','gender'].every(n=>document.querySelector('[name='+n+']').value==='')&&events===8&&submits===0`}, &reply); err != nil || !reply.Result.Value {
		t.Fatalf("live values/events failed: %+v %v", reply, err)
	}
	t.Log("Profile UI save/approve/reload; exact He/Him and He/Him/His native options + text He/him + LinkedIn filled with one Continue; manual They/Them preserved; ambiguous, custom and gender untouched; 8 change/input events, zero submissions")
	run(page, `document.querySelector('.observed-results details').open=true`)
	capture("observed-results", ".observed-results")
	// The test driver represents the human who submits on the synthetic employer.
	if err := employerChrome.FixtureCall(ctx, employer, "Runtime.evaluate", map[string]any{"expression": `document.querySelector('button[type=submit]').click()`}, nil); err != nil {
		t.Fatal(err)
	}
	// Recording a completed application while the managed employer remains open must
	// not cause even one extra input/change or employer submit event.
	run(page, `(()=>{document.querySelector('#record-submission').open=true;document.querySelector('.submission-form [name=confirm]').checked=true;document.querySelector('.submission-form button').click()})()`)
	wait(page, `document.body.textContent.includes('Submission recorded')`)
	if err := employerChrome.FixtureCall(ctx, employer, "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `events===8&&submits===1`}, &reply); err != nil || !reply.Result.Value {
		t.Fatal("recording mutated employer", err)
	}
	t.Log("Recording with managed employer open: Applied/history recorded and employer event counts unchanged")
}
