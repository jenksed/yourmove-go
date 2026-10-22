//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestControlledNarrativeAnswerWorkbench(t *testing.T) {
	if testing.Short() || os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in isolated Chromium narrative drafting qualification")
	}
	a, v, evidenceID := narrativeFixture(t)
	job, _ := a.jobByID(1)
	saveLibrarySourceAnswer(t, a, job, v.Get("question_key"), "I reviewed synthetic database logs.", 0)
	if r := promoteLibrarySource(t, a, 1, v.Get("question_key"), 1); r.Code != 303 {
		t.Fatal(r.Code)
	}
	// The optional writing exists independently; start the target editor empty.
	if _, err := a.db.Exec(`DELETE FROM application_answers WHERE job_id=1`); err != nil {
		t.Fatal(err)
	}
	var calls, mode atomic.Int32
	var noScripts atomic.Bool
	a.applicationAnswerGenerate = func(ctx context.Context, _, _ string) (string, error) {
		calls.Add(1)
		switch mode.Load() {
		case 1:
			return "", errors.New("synthetic-private-provider-error")
		case 2:
			return insufficientAnswer, nil
		case 3:
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		case 4:
			_, err := a.db.Exec(`UPDATE career_evidence SET revision=revision+1 WHERE id=?`, evidenceID)
			if err != nil {
				return "", err
			}
		case 5:
			<-ctx.Done()
			return "", ctx.Err()
		}
		return "I reviewed EXPLAIN plans and recommended an index. The eventual outcome was unknown.", nil
	}
	mux := newMux(a)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if noScripts.Load() && (r.URL.Path == "/static/ui.js" || r.URL.Path == "/static/inspection.js") {
			w.Header().Set("Content-Type", "text/javascript")
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	page := server.URL + "/jobs/1#prepare"
	chrome, err := chromium.LaunchFixture(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	eval := func(script string, out any) error {
		var r struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true}, &r); err != nil {
			return err
		}
		if r.Exception != nil {
			return fmt.Errorf("browser exception: %v", r.Exception)
		}
		if out != nil {
			return json.Unmarshal(r.Result.Value, out)
		}
		return nil
	}
	run := func(script string) {
		t.Helper()
		if err := eval(script, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(script string) {
		t.Helper()
		for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
			var ok bool
			if eval("Boolean("+script+")", &ok) == nil && ok {
				return
			}
			time.Sleep(40 * time.Millisecond)
		}
		var diagnostic string
		_ = eval(`JSON.stringify({ready:document.readyState, preview:document.querySelector('[data-answer-ai-preview]')?.outerHTML, body:document.body.innerText.slice(-2500)})`, &diagnostic)
		t.Fatalf("browser condition: %s\n%s", script, diagnostic)
	}
	assert := func(script string) {
		t.Helper()
		var ok bool
		if err := eval(script, &ok); err != nil || !ok {
			t.Fatalf("browser assertion: %s (%v)", script, err)
		}
	}
	wait(`document.readyState==='complete'&&!!document.querySelector('[data-answer-ai-controls]:not([hidden])')`)
	run(`document.querySelector('[data-tab-target=prepare]').click();window.ai=document.querySelector('[data-answer-ai]');window.ed=document.querySelector('.application-question-form textarea');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']');ai.open=true`)
	assert(`ed.value===''&&!ai.querySelector('[name=career_evidence]').checked&&ai.querySelector('[name=library_entry]').value===''`)
	review := func() {
		t.Helper()
		run(`find('another').open=true;ai.querySelector('[name=career_evidence]').checked=true;ai.querySelector('[name=model]').value='synthetic-model';find('review').click()`)
		wait(`!find('review-panel').hidden`)
	}
	generate := func() {
		t.Helper()
		run(`find('authorize').checked=true;find('external').checked=true;find('generate').click()`)
	}

	press := func(key, code string, number int) {
		t.Helper()
		for _, kind := range []string{"keyDown", "keyUp"} {
			if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": kind, "key": key, "code": code, "windowsVirtualKeyCode": number}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Activate native sources, review and consent with keyboard events.
	run(`ai.querySelector('[name=career_evidence]').focus()`)
	press(" ", "Space", 32)
	assert(`ai.querySelector('[name=career_evidence]').checked`)
	run(`ai.querySelector('[name=library_entry]').value=ai.querySelector('[name=library_entry]').options[1].value;ai.querySelector('[name=library_entry]').dispatchEvent(new Event('change',{bubbles:true}))`)
	run(`ai.querySelector('[name=model]').focus()`)
	if err := chrome.FixtureCall(ctx, page, "Input.insertText", map[string]any{"text": "synthetic-model"}, nil); err != nil {
		t.Fatal(err)
	}
	assert(`ai.querySelector('[name=model]').value==='synthetic-model'`)
	run(`find('review').focus()`)
	assert(`document.activeElement===find('review')`)
	press(" ", "Space", 32)
	wait(`!find('review-panel').hidden`)
	if calls.Load() != 0 {
		t.Fatal("review invoked provider")
	}
	assert(`find('payload').textContent.includes('unknown')&&!find('payload').textContent.includes('PRIVATE_SOURCE_NOTES')&&!find('payload').textContent.includes('PRIVATE_ORIGINAL_WRITING')`)
	run(`find('generate').click()`)
	assert(`find('status').textContent.includes('Authorize')`)
	run(`find('authorize').focus()`)
	press(" ", "Space", 32)
	run(`find('external').focus()`)
	press(" ", "Space", 32)
	run(`find('generate').focus()`)
	press(" ", "Space", 32)
	wait(`!find('preview').hidden&&ai.getAttribute('aria-busy')==='false'`)
	assert(`ed.value===''&&find('text').readOnly&&find('text').value.includes('EXPLAIN')`)
	if calls.Load() != 1 {
		t.Fatal("calls", calls.Load())
	}
	// Preview survives a normal page refresh; no generation occurs on reload.
	run(`location.reload()`)
	wait(`document.readyState==='complete'&&document.querySelector('[data-answer-ai-preview]:not([hidden])')`)
	run(`window.ai=document.querySelector('[data-answer-ai]');window.ed=document.querySelector('.application-question-form textarea');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']')`)
	assert(`ed.value===''&&find('status').textContent.includes('Recovered')`)
	if calls.Load() != 1 {
		t.Fatal("reload generated")
	}
	run(`ed.value='My dirty answer';ed.dispatchEvent(new Event('input',{bubbles:true}));window.confirm=()=>false;find('adopt').click()`)
	wait(`!find('adopt').disabled`)
	assert(`ed.value==='My dirty answer'`)
	run(`window.confirm=()=>true;find('adopt').focus()`)
	press(" ", "Space", 32)
	wait(`ed.value.includes('EXPLAIN')`)
	assert(`ed.closest('form').dataset.dirty==='true'&&ed.closest('form').querySelector('[name=revision]').value==='0'`)
	// Slow generation leaves typing intact, including double clicks.
	mode.Store(3)
	review()
	before := calls.Load()
	generate()
	run(`find('generate').click();ed.value='Typed while generating';ed.dispatchEvent(new Event('input',{bubbles:true}))`)
	wait(`ai.getAttribute('aria-busy')==='false'`)
	assert(`ed.value==='Typed while generating'`)
	if calls.Load() != before+1 {
		t.Fatal("double click repeated provider")
	}
	// A change during asynchronous adoption must abort insertion.
	run(`window.confirm=()=>false;find('adopt').click();ed.value='Typed during adopt check';ed.dispatchEvent(new Event('input',{bubbles:true}))`)
	wait(`find('status').textContent.includes('preserved')`)
	assert(`ed.value==='Typed during adopt check'`)
	// Failure and insufficient evidence preserve the existing preview and editor.
	for _, m := range []int32{1, 2} {
		mode.Store(m)
		review()
		generate()
		wait(`ai.getAttribute('aria-busy')==='false'`)
		assert(`ed.value==='Typed during adopt check'&&!find('preview').hidden`)
	}
	mode.Store(5)
	review()
	generate()
	wait(`ai.getAttribute('aria-busy')==='true'`)
	run(`find('cancel').click()`)
	wait(`ai.getAttribute('aria-busy')==='false'`)
	assert(`ed.value==='Typed during adopt check'`)
	mode.Store(4)
	review()
	generate()
	wait(`ai.getAttribute('aria-busy')==='false'`)
	assert(`find('adopt').disabled&&find('status').textContent.includes('stale')`)
	// New review reloads the current evidence revision from the server-rendered choices.
	run(`ed.value='';ed.dispatchEvent(new Event('input',{bubbles:true}));window.confirm=()=>true;location.reload()`)
	wait(`document.readyState==='complete'&&!!document.querySelector('[data-answer-ai]')`)
	run(`document.querySelector('[data-tab-target=prepare]').click();window.ai=document.querySelector('[data-answer-ai]');window.ed=document.querySelector('.application-question-form textarea');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']');ai.open=true`)

	mode.Store(0)
	// Storage failure retains an in-page preview and navigation protection.
	run(`window.savedSetItem=Storage.prototype.setItem;Storage.prototype.setItem=function(){throw Error('synthetic storage failure')}`)
	review()
	generate()
	wait(`ai.getAttribute('aria-busy')==='false'`)
	assert(`ai.dataset.answerAiUnprotected==='true'&&find('status').textContent.includes('unavailable')&&!find('preview').hidden`)
	run(`Storage.prototype.setItem=window.savedSetItem;find('discard').click()`)
	assert(`!ai.dataset.answerAiUnprotected`)
	review()
	generate()
	wait(`ai.getAttribute('aria-busy')==='false'`)
	// Recheck after replacement confirmation as well as after the HTTP check.
	run(`ed.value='Existing text';ed.dispatchEvent(new Event('input',{bubbles:true}));window.confirm=()=>{ed.value='Typed during confirmation';ed.dispatchEvent(new Event('input',{bubbles:true}));return true};find('adopt').click()`)
	wait(`find('status').textContent.includes('preserved')`)
	assert(`ed.value==='Typed during confirmation'`)
	run(`ed.value='';ed.dispatchEvent(new Event('input',{bubbles:true}));window.confirm=()=>true`)
	review()
	generate()
	wait(`ai.getAttribute('aria-busy')==='false'`)
	// Keyboard-operable native disclosure and checkboxes; narrow light/dark shots.
	run(`find('another').focus();ai.querySelector('summary').focus()`)
	if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13}, nil); err != nil {
		t.Fatal(err)
	}
	run(`ai.open=true`)
	for _, theme := range []string{"light", "dark"} {
		shot, err := chrome.FixtureScreenshotAt(ctx, page, 390, ".application-questions", theme)
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(os.TempDir(), "yourmove-narrative-390-"+theme+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		assert(`document.documentElement.scrollWidth<=innerWidth`)
	}
	run(`window.confirm=()=>true;find('adopt').click()`)
	wait(`ed.value.includes('EXPLAIN')`)
	run(`ed.value+=' I documented my recommendation.';ed.dispatchEvent(new Event('input',{bubbles:true}));ed.closest('form').querySelector('button[type=submit]').click()`)
	page = server.URL + "/jobs/1?question=" + v.Get("question_key")
	wait(`document.readyState==='complete'&&document.querySelector('.application-question-form [name=revision]').value==='1'`)
	var saved string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1`).Scan(&saved); err != nil || !strings.Contains(saved, "documented") {
		t.Fatal(saved, err)
	}
	// Expired recovered previews stay copyable; server refuses adoption.
	a.answerAI.mu.Lock()
	for _, ticket := range a.answerAI.tickets {
		ticket.expires = time.Now().Add(-time.Minute)
	}
	a.answerAI.mu.Unlock()
	run(`window.ai=document.querySelector('[data-answer-ai]');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']');ai.open=true;find('adopt').disabled=false;find('adopt').click()`)
	wait(`find('status').textContent.includes('expired')||find('status').textContent.includes('changed')`)
	assert(`find('adopt').disabled&&find('text').value.includes('EXPLAIN')`)
	// Script-free ordinary saving remains the native form path.
	noScripts.Store(true)
	run(`window.confirm=()=>true;location.reload()`)
	wait(`document.readyState==='complete'&&!!document.querySelector('.application-question-form')`)
	run(`document.querySelector('.application-question-form textarea').value='No JavaScript local answer';document.querySelector('.application-question-form button[type=submit]').click()`)
	wait(`document.readyState==='complete'&&document.querySelector('.application-question-form textarea').value==='No JavaScript local answer'`)
	var submissions int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissions); err != nil || submissions != 0 {
		t.Fatal("automatic submission", submissions, err)
	}
	if a.browsers.state(1).Destination != "" {
		t.Fatal("AI opened employer browser")
	}
	dir := filepath.Dir(a.resumeDir)
	server.Close()
	a.Close()
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	restarted := newTestApp(t, db, a.resumeDir)
	currentJob, _ := restarted.jobByID(1)
	prepared, err := restarted.prepareQuestions(currentJob, CandidateProfile{})
	if err != nil || prepared.Questions[0].Value != "No JavaScript local answer" {
		t.Fatal("restart persistence", err)
	}
	t.Log("Controlled Chromium: explicit selection/review/consent, no call during review/reload, one call on double click, read-only preview/recovery, canceled replacement, typing during generation/adoption, failed/insufficient/canceled/stale results, keyboard sources/consent/generation/adoption, confirmation recheck, storage failure protection, 390px light/dark without overflow, explicit unsaved adoption/save, no-JS save, SQLite restart; no automatic employer mutation/submission. Native Wails was not exercised.")
}
