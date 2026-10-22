//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestControlledNarrativeAnswerWorkbench(t *testing.T) {
	if testing.Short() || os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in isolated Chromium narrative drafting qualification")
	}
	a, v, evidenceID := narrativeFixture(t)
	requireQualificationStorage(t, a)
	evidenceRoot, err := os.MkdirTemp("/private/tmp", "yourmove-checkpoint2-ai-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("isolated narrative screenshots: %s", evidenceRoot)
	job, _ := a.jobByID(1)
	observed, err := a.latestInspectionBySource(job.ID, "browser")
	if err != nil {
		t.Fatal(err)
	}
	fields := make([]schema.Field, 0, len(observed.Fields)+2)
	for _, field := range observed.Fields {
		fields = append(fields, field.Field)
	}
	fields = append(fields,
		schema.Field{ID: "browser:story-second", Label: "Describe how you support a second technical customer", Kind: "textarea"},
		schema.Field{ID: "browser:story-third", Label: "Describe how you prioritized competing commitments while coordinating a cross-functional product launch, keeping stakeholders informed, and resolving delays that affected several teams", Kind: "textarea"},
	)
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: observed.Platform, Source: observed.Source, ApplicationURL: observed.ApplicationURL, Fields: fields}); err != nil {
		t.Fatal(err)
	}
	addCareerEvidenceForSearch(t, a, careerEvidenceContent{Title: "Enterprise production incident response", Situation: "An enterprise customer reported a production database incident.", Actions: "I reviewed service and database logs, coordinated the investigation, and explained the recovery steps to the customer.", OutcomeState: "not_recorded"}, true)
	saveLibrarySourceAnswer(t, a, job, v.Get("question_key"), "I reviewed synthetic database logs.", 0)
	if r := promoteLibrarySource(t, a, 1, v.Get("question_key"), 1); r.Code != 303 {
		t.Fatal(r.Code)
	}
	// The optional writing exists independently; start the target editor empty.
	if _, err := a.db.Exec(`DELETE FROM application_answers WHERE job_id=1`); err != nil {
		t.Fatal(err)
	}
	var calls, mode atomic.Int32
	var helpRequests atomic.Int32
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
		if r.URL.Path == "/jobs/1/answers/help" {
			helpRequests.Add(1)
		}
		if noScripts.Load() && (r.URL.Path == "/static/ui.js" || r.URL.Path == "/static/inspection.js") {
			w.Header().Set("Content-Type", "text/javascript")
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	started := time.Now()
	initialResponse, err := http.Get(server.URL + "/jobs/1")
	if err != nil {
		t.Fatal(err)
	}
	initialBody, err := io.ReadAll(initialResponse.Body)
	_ = initialResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	initialRenderDuration := time.Since(started)
	initialRenderBytes := len(initialBody)
	if strings.Contains(string(initialBody), `data-answer-ai-controls`) || strings.Contains(string(initialBody), `data-answer-ai-review`) {
		t.Fatal("initial Prepare response eagerly rendered AI controls")
	}
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
	capture := func(name string, width int, selector, theme string) {
		t.Helper()
		shot, err := chrome.FixtureScreenshotAt(ctx, page, width, selector, theme)
		if err != nil {
			t.Fatalf("capture %s: %v", name, err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatalf("decode screenshot %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceRoot, name+".png"), data, 0600); err != nil {
			t.Fatalf("write screenshot %s: %v", name, err)
		}
	}
	wait(`document.readyState==='complete'&&!!document.querySelector('[data-question-workbench]')`)
	run(`document.querySelector('[data-tab-target=prepare]').click()`)
	var activationTimes []time.Duration
	activateSelected := func() {
		started := time.Now()
		run(`(()=>{const p=document.querySelector('[data-question-panel]:not([hidden])');const h=p?.querySelector('[data-question-help]');if(h)h.open=true;return !!h})()`)
		wait(`!!document.querySelector('[data-question-panel]:not([hidden]) [data-answer-ai]')`)
		run(`window.ai=document.querySelector('[data-question-panel]:not([hidden]) [data-answer-ai]');window.ed=document.querySelector('[data-question-panel]:not([hidden]) .application-question-form textarea');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']');ai.open=true`)
		activationTimes = append(activationTimes, time.Since(started))
	}
	if helpRequests.Load() != 0 {
		t.Fatal("assistance loaded before the user opened Help me answer")
	}
	activateSelected()
	capture("initial-actionable-1280-light", 1280, ".application-questions", "light")
	capture("initial-actionable-1440-dark", 1440, ".application-questions", "dark")
	capture("initial-actionable-390-light", 390, ".application-questions", "light")
	capture("initial-actionable-390-dark", 390, ".application-questions", "dark")
	firstQuestionKey := ""
	if err := eval(`document.querySelector('[data-question-panel]:not([hidden])').dataset.questionKey`, &firstQuestionKey); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		run(`document.querySelector('[data-question-next]').click()`)
		activateSelected()
		assert(`ai.dataset.key===document.querySelector('[data-question-panel]:not([hidden])').dataset.questionKey&&ai.dataset.key!==` + fmt.Sprintf("%q", firstQuestionKey))
	}
	run(`ed.value='unsaved third question';ed.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-question-previous]').click()`)
	activateSelected()
	assert(`ed.value===''`)
	run(`document.querySelector('[data-question-next]').click()`)
	activateSelected()
	assert(`ed.value==='unsaved third question'`)
	run(`ed.value='';ed.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-question-previous]').click()`)
	activateSelected()
	run(`document.querySelector('[data-question-previous]').click()`)
	activateSelected()
	assert(`ai.dataset.key===` + fmt.Sprintf("%q", firstQuestionKey))
	run(`document.querySelectorAll('[data-question-index] a[data-question-link]')[2].click()`)
	activateSelected()
	assert(`ai.dataset.key===document.querySelector('[data-question-panel]:not([hidden])').dataset.questionKey&&ai.dataset.key!==` + fmt.Sprintf("%q", firstQuestionKey))
	capture("long-question-1280-light", 1280, ".application-questions", "light")
	run(`document.querySelectorAll('.question-help-content > .help-tool')[0].open=true;document.querySelectorAll('.question-help-content > .help-tool')[0].scrollIntoView({block:'center'})`)
	capture("no-relevant-suggestions-390-dark", 390, "[data-question-help]", "dark")
	run(`document.querySelectorAll('.question-help-content > .help-tool')[0].open=false;document.querySelectorAll('.question-help-content > .help-tool')[1].open=true;document.querySelectorAll('.question-help-content > .help-tool')[1].scrollIntoView({block:'center'})`)
	capture("no-related-experience-390-light", 390, "[data-question-help]", "light")
	run(`document.querySelectorAll('[data-question-index] a[data-question-link]')[0].click()`)
	activateSelected()
	var initialProvider struct{ Provider, Model, DefaultModel, Help string }
	if err := eval(`(()=>{const ai=document.querySelector('[data-answer-ai]'),provider=ai.querySelector('[name=provider]'),model=ai.querySelector('[name=model]');return {Provider:provider.value,Model:model.value,DefaultModel:provider.options[provider.selectedIndex].dataset.defaultModel,Help:ai.querySelector('[data-answer-ai-provider-help]').textContent}})()`, &initialProvider); err != nil {
		t.Fatal(err)
	}
	t.Logf("rendered answer-drafting provider/model before user selection: %q / %q", initialProvider.Provider, initialProvider.Model)
	if initialProvider.Provider != "ollama" || initialProvider.Model == "" || initialProvider.Model != initialProvider.DefaultModel || !strings.Contains(initialProvider.Help, "checked when you generate") {
		t.Fatalf("provider readiness/default model is unclear: %+v", initialProvider)
	}
	activateSelected()
	run(`document.querySelectorAll('.question-help-content > .help-tool')[0].open=true;document.querySelector('[data-related-answer]').open=true`)
	wait(`document.querySelector('[data-related-answer]').dataset.loaded==='true'`)
	run(`document.querySelector('.related-writing-full-answer').scrollIntoView({block:'center'})`)
	capture("previous-writing-1280-light", 1280, ".related-writing-example", "light")
	run(`document.querySelectorAll('.question-help-content > .help-tool')[0].open=false;document.querySelectorAll('.question-help-content > .help-tool')[1].open=true`)
	assert(`document.querySelectorAll('.question-help-content > .help-tool')[1].textContent.includes('Enterprise production incident response')`)
	run(`document.querySelector('.related-career-evidence').scrollIntoView({block:'center'})`)
	capture("career-evidence-1280-dark", 1280, ".related-career-evidence", "dark")
	run(`document.querySelectorAll('.question-help-content > .help-tool')[1].open=false;find('another').open=true`)
	run(`find('selection').scrollIntoView({block:'center'})`)
	capture("ai-source-selection-1280-light", 1280, "[data-answer-ai-selection]", "light")
	capture("ai-source-selection-390-dark", 390, "[data-answer-ai-selection]", "dark")
	assert(`ed.value===''&&!ai.querySelector('[name=career_evidence]').checked&&ai.querySelector('[name=library_entry]').value===''`)
	var sourceReviewCaptured bool
	review := func() {
		t.Helper()
		run(`find('another').open=true;[...ai.querySelectorAll('[name=career_evidence]')].find(input=>input.value.startsWith(` + fmt.Sprintf("%q", fmt.Sprintf("%d:", evidenceID)) + `)).checked=true;ai.querySelector('[name=model]').value='synthetic-model';find('review').click()`)
		wait(`!find('review-panel').hidden`)
		if !sourceReviewCaptured {
			capture("ai-source-review-1280-light", 1280, "[data-answer-ai-review-panel]", "light")
			sourceReviewCaptured = true
		}
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
	run(`find('another').open=true`)
	run(`([...ai.querySelectorAll('[name=career_evidence]')].find(input=>input.value.startsWith(` + fmt.Sprintf("%q", fmt.Sprintf("%d:", evidenceID)) + `))).focus()`)
	press(" ", "Space", 32)
	assert(`[...ai.querySelectorAll('[name=career_evidence]')].find(input=>input.value.startsWith(` + fmt.Sprintf("%q", fmt.Sprintf("%d:", evidenceID)) + `)).checked`)
	run(`ai.querySelector('[name=library_entry]').value=ai.querySelector('[name=library_entry]').options[1].value;ai.querySelector('[name=library_entry]').dispatchEvent(new Event('change',{bubbles:true}))`)
	run(`ai.querySelector('[name=model]').focus()`)
	run(`ai.querySelector('[name=model]').select()`)
	if err := chrome.FixtureCall(ctx, page, "Input.insertText", map[string]any{"text": "synthetic-model"}, nil); err != nil {
		t.Fatal(err)
	}
	assert(`ai.querySelector('[name=model]').value==='synthetic-model'`)
	run(`find('review').focus()`)
	assert(`document.activeElement===find('review')`)
	press(" ", "Space", 32)
	wait(`!find('review-panel').hidden`)
	run(`find('review-panel').scrollIntoView({block:'center'})`)
	capture("ai-source-review-1280-light", 1280, "[data-answer-ai-review-panel]", "light")
	capture("ai-source-review-390-dark", 390, "[data-answer-ai-review-panel]", "dark")
	if calls.Load() != 0 {
		t.Fatal("review invoked provider")
	}
	assert(`find('payload').textContent.includes('unknown')&&!find('payload').textContent.includes('PRIVATE_SOURCE_NOTES')&&!find('payload').textContent.includes('PRIVATE_ORIGINAL_WRITING')`)
	capture("ai-source-review-confirmation-1440-dark", 1440, "[data-answer-ai-review-panel]", "dark")
	run(`ai.querySelector('[name=model]').value='different-synthetic-model';ai.querySelector('[name=model]').dispatchEvent(new Event('input',{bubbles:true}))`)
	assert(`find('review-panel').hidden&&!find('authorize').checked&&!find('external').checked`)
	run(`ai.querySelector('[name=provider]').value='openai';ai.querySelector('[name=provider]').dispatchEvent(new Event('change',{bubbles:true}))`)
	assert(`find('review-panel').hidden&&!find('authorize').checked&&!find('external').checked&&ai.querySelector('[data-answer-ai-provider-help]').textContent.trim().length>0`)
	run(`ai.querySelector('[name=provider]').value='ollama';ai.querySelector('[name=provider]').dispatchEvent(new Event('change',{bubbles:true}));ai.querySelector('[name=model]').value='synthetic-model';ai.querySelector('[name=model]').dispatchEvent(new Event('input',{bubbles:true}))`)
	review()
	if calls.Load() != 0 {
		t.Fatal("provider/model changes called the injected generator before authorization")
	}
	run(`find('generate').click()`)
	assert(`find('status').textContent.includes('Authorize')`)
	run(`find('authorize').focus()`)
	press(" ", "Space", 32)
	run(`find('external').focus()`)
	press(" ", "Space", 32)
	run(`find('generate').focus()`)
	press(" ", "Space", 32)
	wait(`!find('preview').hidden&&ai.getAttribute('aria-busy')==='false'`)
	assert(`ed.value===''&&find('text').readOnly&&find('text').value.includes('EXPLAIN')&&!find('preview-sources').hidden&&find('preview-payload').textContent.includes('MySQL investigation')`)
	run(`find('preview').scrollIntoView({block:'center'})`)
	capture("generated-proposal-1440-dark", 1440, "[data-answer-ai-preview]", "dark")
	capture("generated-proposal-390-light", 390, "[data-answer-ai-preview]", "light")
	capture("generated-proposal-390-dark", 390, "[data-answer-ai-preview]", "dark")
	shot, err := chrome.FixtureScreenshotAt(ctx, page, 1280, ".application-questions", "light")
	if err != nil {
		t.Fatal(err)
	}
	previewImage, err := base64.StdEncoding.DecodeString(shot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceRoot, "narrative-preview-1280.png"), previewImage, 0600); err != nil {
		t.Fatal(err)
	}
	assert(`Object.keys(JSON.parse(sessionStorage.getItem('yourmove:pending-edits:'+location.pathname+':ai:'+ai.dataset.key))).every(key=>!['source_payload','sources','prompt','instructions'].includes(key))`)
	if calls.Load() != 1 {
		t.Fatal("calls", calls.Load())
	}
	// Preview survives a normal page refresh; no generation occurs on reload.
	run(`location.reload()`)
	wait(`document.readyState==='complete'&&!!document.querySelector('[data-question-help]')`)
	activateSelected()
	wait(`document.querySelector('[data-answer-ai-preview]:not([hidden])')`)
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
	wait(`document.readyState==='complete'&&!!document.querySelector('[data-question-help]')`)
	activateSelected()

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
		if err := os.WriteFile(filepath.Join(evidenceRoot, "narrative-390-"+theme+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		assert(`document.documentElement.scrollWidth<=innerWidth`)
	}
	run(`window.confirm=()=>true;find('adopt').click()`)
	wait(`ed.value.includes('EXPLAIN')`)
	run(`ed.value+=' I documented my recommendation.';ed.dispatchEvent(new Event('input',{bubbles:true}));ed.closest('form').querySelector('button[type=submit]').click()`)
	page = server.URL + "/jobs/1?question=" + v.Get("question_key")
	wait(`document.readyState==='complete'&&document.querySelector('.application-question-form [name=revision]').value==='1'`)
	run(`document.querySelector('.application-question-form').scrollIntoView({block:'center'})`)
	capture("saved-answer-confirmation-1280-light", 1280, ".application-question-form", "light")
	capture("saved-answer-confirmation-1280-dark", 1280, ".application-question-form", "dark")
	shot, err = chrome.FixtureScreenshotAt(ctx, page, 1280, ".application-questions", "light")
	if err != nil {
		t.Fatal(err)
	}
	savedImage, err := base64.StdEncoding.DecodeString(shot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceRoot, "narrative-saved-1280.png"), savedImage, 0600); err != nil {
		t.Fatal(err)
	}
	var saved string
	if err := a.db.QueryRow(`SELECT answer FROM application_answers WHERE job_id=1`).Scan(&saved); err != nil || !strings.Contains(saved, "documented") {
		t.Fatal(saved, err)
	}
	// Expired recovered previews stay copyable; server refuses adoption.
	activateSelected()
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
	var activationTotal time.Duration
	for _, duration := range activationTimes {
		activationTotal += duration
	}
	meanActivation := time.Duration(0)
	if len(activationTimes) > 0 {
		meanActivation = activationTotal / time.Duration(len(activationTimes))
	}
	t.Logf("Synthetic Chromium measures: initial GET %d bytes in %s; assistance requests before first Help me answer=0; total question-help requests=%d; question activation mean=%s max=%s", initialRenderBytes, initialRenderDuration, helpRequests.Load(), meanActivation, maxDuration(activationTimes))
	t.Log("Controlled Chromium: three eligible narrative questions expose question-bound drafting by Next/Previous/direct selection; explicit source review/consent, no call during review/reload, one call on double click, read-only preview/recovery, canceled replacement, typing during generation/adoption, failed/insufficient/canceled/stale results, keyboard sources/consent/generation/adoption, confirmation recheck, storage failure protection, dark/light 390px without overflow, explicit unsaved adoption/save, no-JS save, SQLite restart; no automatic employer mutation/submission. Native Wails was not exercised.")
}

func maxDuration(values []time.Duration) time.Duration {
	var max time.Duration
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}
