//go:build qualification

package main

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// All storage is testWorkspace's temporary SQLite database; the only navigable
// application is a loopback server. No native app or normal storage is opened.
func TestControlledQuestionWorkbench(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_QUESTIONS") != "1" {
		t.Skip("opt-in isolated question UI qualification")
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Synthetic Northstar"}); err != nil {
		t.Fatal(err)
	}
	observation := schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{
		{ID: "browser:portfolio", Label: "Portfolio or work sample URL", Kind: "url", Required: true},
		{ID: "browser:motivation", Label: "Why would you like to join this team?", Kind: "textarea", Required: true},
		{ID: "browser:working-pattern", Label: "Preferred working pattern", Kind: "select", Options: []string{"Remote", "Office", "Hybrid"}},
		{ID: "browser:availability", Label: "When could you start?", Kind: "text"},
		{ID: "browser:email", Label: "Email", Kind: "email"},
		{ID: "browser:legal", Label: "Work authorization", Kind: "text", Sensitive: true},
		{ID: "browser:unsupported", Label: "Choose a location on the map", Kind: "combobox"},
	}}
	if _, err := a.saveInspection(1, observation); err != nil {
		t.Fatal(err)
	}
	// Use the normal candidate approval and answer POST paths for fixture setup.
	mux := newMux(a)
	if rec := postApp(mux, "/candidate", mapValues("first_name", "Alex", "last_name", "Example", "email", "alex@example.test", "phone", "5551231234", "approve", "yes", "revision", "0"), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	job, _ := a.jobByID(1)
	p, _ := a.prepareQuestions(job, CandidateProfile{})
	if rec := postApp(mux, "/jobs/1/answers/save", mapValues("question_key", p.Questions[3].Key, "revision", "0", "destination", job.URL, "answer", "Two weeks after an offer"), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	fake := &fakeAnswerBrowser{fakeBrowser: &fakeBrowser{url: job.URL + "/application"}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fake, nil }
	if err := a.browsers.start(context.Background(), 1, job.URL); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	a.browsers.cancels = map[int64]context.CancelFunc{1: func() {}}
	a.browsers.setState(1, "detected", "Synthetic observed form.", p.InspectionID, false)
	var noScripts atomic.Bool
	router := http.NewServeMux()
	router.HandleFunc("GET /static/ui.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if noScripts.Load() {
			w.Header().Set("Content-Type", "text/javascript")
			return
		}
		mux.ServeHTTP(w, r)
	})
	router.HandleFunc("GET /static/inspection.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if noScripts.Load() {
			w.Header().Set("Content-Type", "text/javascript")
			return
		}
		mux.ServeHTTP(w, r)
	})
	router.HandleFunc("GET /jobs/1/application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1#prepare", 303) })
	router.Handle("/", mux)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	session, err := chromium.LaunchFixture(ctx, server.URL+"/jobs/1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	page := server.URL + "/jobs/1"
	evaluate := func(script string, out any) error {
		var result struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		err := session.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true}, &result)
		if err != nil {
			return err
		}
		if result.Exception != nil {
			return fmt.Errorf("JS exception: %v", result.Exception)
		}
		if out != nil {
			return json.Unmarshal(result.Result.Value, out)
		}
		return nil
	}
	for deadline := time.Now().Add(10 * time.Second); ; {
		var ready bool
		if evaluate(`document.readyState==='complete' && !!document.querySelector('.application-question-form')`, &ready) == nil && ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("question page did not load")
		}
		time.Sleep(50 * time.Millisecond)
	}
	prefix := os.Getenv("YOURMOVE_QUESTION_EVIDENCE")
	if prefix == "" {
		prefix = "/private/tmp/yourmove-question-workbench/after"
	}
	if err := os.MkdirAll(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{1280, 1050, 800, 700, 390} {
		for _, theme := range []string{"light", "dark"} {
			shot, err := session.FixtureScreenshotAt(ctx, page, width, ".application-questions", theme)
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(shot)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(prefix, fmt.Sprintf("questions-%d-%s.png", width, theme))
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			t.Log("screenshot", path)
		}
	}

	mustEvaluate := func(script string, out any) {
		t.Helper()
		if err := evaluate(script, out); err != nil {
			t.Fatal(err)
		}
	}
	assertJS := func(script string) {
		t.Helper()
		var ok bool
		mustEvaluate(script, &ok)
		if !ok {
			t.Fatalf("rendered assertion failed: %s", script)
		}
	}
	waitJS := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(8 * time.Second); ; {
			var ok bool
			if evaluate(script, &ok) == nil && ok {
				return
			}
			if time.Now().After(deadline) {
				var state string
				_ = evaluate(`JSON.stringify({url:location.href,before:window.beforeRefresh,confirm:window.confirmCalls,values:[...document.querySelectorAll('[name=answer]')].map(e=>e.value),dirty:[...document.querySelectorAll('form[data-dirty="true"]')].map(f=>f.action),cache:sessionStorage.getItem('yourmove:pending-edits:'+location.pathname),recovery:document.querySelector('[data-answer-recovery]')?.textContent})`, &state)
				t.Fatalf("timed out: %s (%s): %s", script, page, state)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	click := func(selector string) {
		t.Helper()
		encoded, _ := json.Marshal(selector)
		var point struct{ X, Y float64 }
		mustEvaluate(`(()=>{const e=document.querySelector(`+string(encoded)+`);if(!e||!e.getClientRects().length)throw Error('unavailable control');e.scrollIntoView({block:'center',behavior:'instant'});const r=e.getBoundingClientRect();return {X:r.x+r.width/2,Y:r.y+r.height/2}})()`, &point)
		for _, typ := range []string{"mousePressed", "mouseReleased"} {
			if err := session.FixtureCall(ctx, page, "Input.dispatchMouseEvent", map[string]any{"type": typ, "x": point.X, "y": point.Y, "button": "left", "clickCount": 1}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	typeText := func(selector, text string) {
		t.Helper()
		click(selector)
		if err := session.FixtureCall(ctx, page, "Input.insertText", map[string]any{"text": text}, nil); err != nil {
			t.Fatal(err)
		}
	}
	choose := func(n int) {
		t.Helper()
		key := p.Questions[n].Key
		mustEvaluate(fmt.Sprintf(`(()=>{const c=document.querySelector('[data-question-chooser]');c.value=%q;c.dispatchEvent(new Event('change',{bubbles:true}));return true})()`, key), nil)
	}
	snapshot := func(name, selector string) {
		t.Helper()
		shot, err := session.FixtureScreenshotAt(ctx, page, 1280, selector, "light")
		if err != nil {
			t.Fatal(err)
		}
		data, _ := base64.StdEncoding.DecodeString(shot)
		if err := os.WriteFile(filepath.Join(prefix, name+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	if os.Getenv("YOURMOVE_QUESTION_BASELINE") == "1" {
		answers := []string{"https://alex.example.test/work", "I enjoy resolving difficult technical problems."}
		for i, text := range answers {
			selector := `#answer-` + p.Questions[i].Key
			typeText(selector, text)
			click(`form:has(` + selector + `) button[type=submit]`)
			waitJS(`document.querySelector('form:has(` + selector + `) [name=revision]')?.value==='1'`)
			var visible bool
			mustEvaluate(`(()=>{const r=document.querySelector('`+selector+`').getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight})()`, &visible)
			t.Logf("baseline save %d: saved editor visible without scrolling=%v; focus returns to Prepare", i+1, visible)
			snapshot(fmt.Sprintf("baseline-save-%d-return", i+1), "#prepare")
		}
		t.Log("baseline actions for two answers: click/type each editor + two Save answer clicks; each save returns to Prepare; locating next editor requires scrolling in this fixture")
		return
	}
	var window struct {
		ID int `json:"windowId"`
	}
	if err := session.FixtureCall(ctx, page, "Browser.getWindowForTarget", nil, &window); err != nil {
		t.Fatal(err)
	}
	if err := session.FixtureCall(ctx, page, "Browser.setWindowBounds", map[string]any{"windowId": window.ID, "bounds": map[string]any{"width": 1280, "height": 900}}, nil); err != nil {
		t.Fatal(err)
	}

	assertJS(`document.querySelectorAll('[data-question-panel]:not([hidden])').length===1 && document.querySelector('[data-question-previous]').disabled`)
	assertJS(`document.querySelectorAll('[data-browser-watch]').length===1`)
	typeText(`[data-question-panel]:not([hidden]) [name=answer]`, "https://alex.example.test/work")
	click(`[data-question-link="` + p.Questions[1].Key + `"]`)
	click(`[data-question-link="` + p.Questions[0].Key + `"]`)
	assertJS(`document.querySelector('[data-question-panel]:not([hidden]) [name=answer]').value==='https://alex.example.test/work'`)

	assertJS(`(()=>{const e=new Event('beforeunload',{cancelable:true});window.dispatchEvent(e);return e.defaultPrevented})()`)
	// Poll-driven observation updates and browser loss must not reload dirty DOM.
	mustEvaluate(`window.originalEditor=document.querySelector('[name=answer]');window.confirmCalls=[];window.confirm=message=>{confirmCalls.push(message);return false};true`, nil)
	observation.Fields = append(observation.Fields, schema.Field{ID: "browser:conditional", Label: "Which tools have you used? Describe your experience with " + strings.Repeat("extendedprojectname", 10), Kind: "textarea"})
	observed, err := a.saveInspection(1, observation)
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.setState(1, "detected", "Synthetic conditional question.", observed, false)
	waitJS(`!!document.querySelector('[data-new-questions]')`)
	assertJS(`window.originalEditor===document.querySelector('[name=answer]') && originalEditor.value==='https://alex.example.test/work'`)
	// Same-job refresh preserves unsaved text across the new document; leaving
	// for an unrelated page still requires an explicit discard decision.
	mustEvaluate(`window.beforeRefresh=true;true`, nil)
	click(`[data-new-questions] a`)
	waitJS(`!window.beforeRefresh && document.readyState==='complete' && document.querySelector('[name=answer]')?.value==='https://alex.example.test/work' && !!document.querySelector('form[data-dirty="true"]')`)
	mustEvaluate(`window.originalEditor=document.querySelector('[name=answer]');window.confirmCalls=[];window.confirm=message=>{confirmCalls.push(message);return false};true`, nil)
	click(`a.nav-item[href="/resumes"]`)
	assertJS(`confirmCalls.length===1 && location.pathname==='/jobs/1'`)
	// Next/Previous retain the exact same input nodes and unsaved values.
	click(`[data-question-next]`)
	assertJS(`document.querySelector('[data-question-chooser]').value===document.querySelector('[data-question-panel]:not([hidden])').dataset.questionKey`)
	typeText(`[data-question-panel]:not([hidden]) [name=answer]`, "I enjoy resolving difficult technical problems.")
	click(`[data-question-previous]`)
	assertJS(`originalEditor===document.querySelector('[name=answer]') && originalEditor.value==='https://alex.example.test/work'`)
	snapshot("dirty-navigation", ".application-questions")
	// Saving one question must guard a second unsaved answer instead of losing it.
	click(`[data-question-panel]:not([hidden]) button[type=submit]`)
	assertJS(`confirmCalls.length===2 && document.querySelectorAll('form[data-dirty="true"]').length===2`)
	// Save the other work later: explicitly clear it in this fixture, leaving only
	// the selected question dirty. No discard is implicit in question navigation.
	choose(1)
	mustEvaluate(`(()=>{const e=document.querySelector('[data-question-panel]:not([hidden]) [name=answer]');e.value='';e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`, nil)
	choose(0)
	// Save and Next are separately activated; the POST persists this identity only.
	click(`[data-question-panel]:not([hidden]) button[type=submit]`)
	page = server.URL + "/jobs/1?question=" + p.Questions[0].Key
	waitJS(`document.readyState==='complete' && document.querySelector('[data-question-panel]:not([hidden]) [name=revision]')?.value==='1' && document.activeElement===document.querySelector('[data-question-panel]:not([hidden])')`)
	assertJS(`document.activeElement===document.querySelector('[data-question-panel]:not([hidden])') && document.querySelector('[data-question-panel]:not([hidden]) [data-answer-status]').textContent.includes('Saved locally')`)
	current, _ := a.prepareQuestions(job, CandidateProfile{})
	if current.Questions[0].Value != "https://alex.example.test/work" || current.Questions[1].Revision != 0 {
		t.Fatal("wrong saved identity or content")
	}
	if len(fake.transferred) != 0 {
		t.Fatal("local save mutated employer answers")
	}
	// All common actions fit without a scroll between typing, save, and Next in
	// the inspected short-question surface (long answers may still need scrolling).
	assertJS(`(()=>{const q=s=>document.querySelector(s).getBoundingClientRect();return q('[data-question-panel]:not([hidden]) [name=answer]').top>=0 && q('[data-question-next]').bottom<=innerHeight})()`)
	snapshot("saved-return", ".application-questions")
	click(`[data-question-next]`)
	typeText(`[data-question-panel]:not([hidden]) [name=answer]`, "I enjoy resolving difficult technical problems.")
	click(`[data-question-panel]:not([hidden]) button[type=submit]`)
	page = server.URL + "/jobs/1?question=" + p.Questions[1].Key
	waitJS(`document.querySelector('[data-question-panel]:not([hidden]) [name=revision]')?.value==='1'`)
	click(`[data-question-next]`)
	// Selecting the observed option keeps its exact server-rendered string.
	mustEvaluate(`(()=>{const s=document.querySelector('[data-question-panel]:not([hidden]) [name=answer]');s.value='Remote';s.dispatchEvent(new Event('change',{bubbles:true}));return true})()`, nil)
	click(`[data-question-panel]:not([hidden]) button[type=submit]`)
	page = server.URL + "/jobs/1?question=" + p.Questions[2].Key
	waitJS(`document.querySelector('[data-question-panel]:not([hidden]) [name=revision]')?.value==='1'`)
	click(`[data-review-ready]`)
	assertJS(`document.activeElement.id==='application-action' && document.querySelector('form[action$="/continue"]')?.textContent.includes('https://alex.example.test/work') && document.querySelector('form[action$="/continue"]')?.textContent.includes('I enjoy resolving difficult technical problems.')`)
	snapshot("ready-scope", "#application-action")
	// Explicit profile overrides remain separate and manual panels have no forms.
	assertJS(`!document.querySelector('[data-question-chooser]').textContent.includes('Email') && !!document.querySelector('.profile-question-summary .question-override:not([open])')`)
	click(`.profile-question-summary > summary`)
	assertJS(`document.querySelector('.profile-question-summary').textContent.includes('alex@example.test')`)
	snapshot("profile-facts", ".profile-question-summary")
	choose(5)
	assertJS(`!document.querySelector('[data-question-panel]:not([hidden]) form') && document.querySelector('[data-question-panel]:not([hidden])').textContent.includes('manual completion')`)
	p = current // The conditional question is now part of the freshly rendered form.
	choose(7)
	assertJS(`document.querySelector('[data-question-next]').disabled`)
	longShot, err := session.FixtureScreenshotAt(ctx, page, 390, ".application-questions", "dark")
	if err != nil {
		t.Fatal(err)
	}
	longData, _ := base64.StdEncoding.DecodeString(longShot)
	if err := os.WriteFile(filepath.Join(prefix, "long-question-390-dark.png"), longData, 0600); err != nil {
		t.Fatal(err)
	}
	// Keyboard activation of Previous changes focus and editor together; no wrap.
	mustEvaluate(`document.querySelector('[data-question-previous]').focus();true`, nil)
	for _, typ := range []string{"keyDown", "keyUp"} {
		params := map[string]any{"type": typ, "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13}
		if typ == "keyDown" {
			params["text"] = "\r"
			params["unmodifiedText"] = "\r"
		}
		if err := session.FixtureCall(ctx, page, "Input.dispatchKeyEvent", params, nil); err != nil {
			t.Fatal(err)
		}
	}
	assertJS(`document.activeElement===document.querySelector('[data-question-panel]:not([hidden])')`)
	// Controlled revision conflict: preserve the submitted text in the rendered
	// recovery page while the separately saved value remains authoritative.
	choose(1)
	typeText(`[data-question-panel]:not([hidden]) [name=answer]`, " Extra unsaved sentence.")

	if err := a.browsers.close(1); err != nil {
		t.Fatal(err)
	}
	waitJS(`!!document.querySelector('[data-new-questions]')`)
	assertJS(`document.querySelector('[data-question-panel]:not([hidden]) [name=answer]').value.includes('Extra unsaved sentence.')`)
	persisted, _ := a.prepareQuestions(job, CandidateProfile{})
	if persisted.Questions[0].Value != "https://alex.example.test/work" {
		t.Fatal("browser loss lost local saves")
	}
	value := "Another window saved this answer."
	if rec := postApp(mux, "/jobs/1/answers/save", mapValues("question_key", p.Questions[1].Key, "revision", "1", "destination", job.URL, "answer", value), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	click(`[data-question-panel]:not([hidden]) button[type=submit]`)
	page = server.URL + "/jobs/1/answers/save"
	waitJS(`!!document.querySelector('[data-answer-recovery]')`)
	assertJS(`document.querySelector('#unsaved-answer').value.includes('Extra unsaved sentence.') && !document.querySelector('form[action$="/continue"]')`)
	snapshot("conflict-recovery", ".answer-save-error")
	current, _ = a.prepareQuestions(job, CandidateProfile{})
	if current.Questions[1].Value != value {
		t.Fatal("conflict overwrote saved value")
	}
	// No-JS qualification: serve the exact templates/styles with both progressive
	// enhancement scripts omitted; all editors and normal POST saves remain usable.
	noScripts.Store(true)
	mustEvaluate(`window.confirm=()=>true;document.querySelector('.answer-save-error a').click();true`, nil)
	page = server.URL + "/jobs/1"
	waitJS(`document.readyState==='complete' && document.querySelectorAll('.application-question-form').length===6`)
	click(`.question-override summary`)
	assertJS(`!document.querySelector('.questions-enabled') && [...document.querySelectorAll('.application-question-form')].every(f=>f.getClientRects().length>0) && !document.querySelector('.question-panel[hidden]')`)
	typeText(`[name=question_key][value="`+p.Questions[0].Key+`"] ~ .field [name=answer]`, " Updated without JavaScript.")
	var submittedWithoutJS string
	mustEvaluate(`document.querySelector('#question-`+p.Questions[0].Key+` [name=answer]').value`, &submittedWithoutJS)
	click(`#question-` + p.Questions[0].Key + ` button[type=submit]`)
	page = server.URL + "/jobs/1?question=" + p.Questions[0].Key
	waitJS(`document.querySelector('#question-` + p.Questions[0].Key + ` [name=revision]')?.value==='2'`)
	current, _ = a.prepareQuestions(job, CandidateProfile{})
	if current.Questions[0].Value != submittedWithoutJS {
		t.Fatal("no-JS local save failed")
	}
	snapshot("no-javascript", ".application-questions")
	if len(fake.transferred) != 0 || len(a.browsers.answers) != 0 {
		t.Fatal("editing armed or replayed answer transfer")
	}
	t.Log("UI journey: typed/navigated/saved three exact answers; same-question save focus; explicit Next; ready scope; canceled destructive navigation; dirty poll preservation; profile/manual panels; keyboard Previous; conflict recovery; no-JS POST; zero answer transfer calls")

}

func mapValues(pairs ...string) map[string][]string {
	out := map[string][]string{}
	for i := 0; i < len(pairs); i += 2 {
		out[pairs[i]] = []string{pairs[i+1]}
	}
	return out
}
