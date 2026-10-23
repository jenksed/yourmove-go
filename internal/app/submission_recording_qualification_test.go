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

// This qualification uses private SQLite/resume storage and real loopback
// Chromium. It drives the rendered completion form instead of submitting a
// constructed POST, and keeps all employer actions in synthetic fixtures.
func TestControlledSubmissionRecordingWorkflow(t *testing.T) {
	a, workspaceRoot := testWorkspace(t)
	var sequence int
	var databaseName, databasePath string
	if err := a.db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &databaseName, &databasePath); err != nil {
		t.Fatal(err)
	}
	canonicalWorkspace, workspaceErr := filepath.EvalSymlinks(workspaceRoot)
	canonicalDB, databaseErr := filepath.EvalSymlinks(databasePath)
	canonicalResumes, resumesErr := filepath.EvalSymlinks(a.resumeDir)
	withinWorkspace := func(path string) bool {
		relative, err := filepath.Rel(canonicalWorkspace, path)
		return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
	}
	if databaseName != "main" || workspaceErr != nil || databaseErr != nil || resumesErr != nil ||
		!withinWorkspace(canonicalDB) || !withinWorkspace(canonicalResumes) {
		t.Fatalf("qualification storage escaped isolated workspace: db=%q resumes=%q workspace=%q", databasePath, a.resumeDir, workspaceRoot)
	}
	t.Logf("isolated SQLite: %s; resumes: %s; workspace: %s", canonicalDB, canonicalResumes, canonicalWorkspace)
	evidenceRoot, err := os.MkdirTemp("/private/tmp", "yourmove-checkpoint1-submission-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("isolated submission screenshots: %s", evidenceRoot)
	for _, job := range []Job{
		{URL: testAshbyPosting, Title: "Synthetic reviewed application", Company: "Fixture Employer"},
		{URL: "https://jobs.ashbyhq.com/fixture/second", Title: "Synthetic next opportunity", Stage: StageReady},
		{URL: "https://jobs.ashbyhq.com/fixture/recovery", Title: "Synthetic resume recovery"},
	} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	resume, err := a.importResume("Synthetic_Reviewed_Resume.pdf", strings.NewReader("%PDF-1.4\nsynthetic reviewed resume"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(3, resume.ID); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:why", Label: "Why this synthetic team?", Kind: "textarea"}
	if _, err = a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	const savedLetter = "A synthetic saved cover letter that remains available as the original local draft."
	if _, err = a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,?,'ollama','fixture-model',?,?,?,?,'fixture-job-description','2026-10-22T00:00:00Z')`, savedLetter, resume.ID, resume.SHA256, resume.ID, resume.SHA256); err != nil {
		t.Fatal(err)
	}

	desktopMux := newDesktopMux(a)
	desktopRouter := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.Clone(r.Context())
		r.Host = "localhost"
		r.Header.Del("Origin")
		desktopMux.ServeHTTP(w, r)
	})
	desktopServer := httptest.NewServer(desktopRouter)
	defer desktopServer.Close()
	browserServer := httptest.NewServer(newMux(a))
	defer browserServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pages := map[*chromium.Session]*chromium.FixturePage{}
	fixturePage := func(chrome *chromium.Session, page string) *chromium.FixturePage {
		t.Helper()
		if current := pages[chrome]; current != nil {
			return current
		}
		current, err := chrome.FixturePage(ctx, page)
		if err != nil {
			t.Fatalf("open Chromium fixture tab %s: %v", page, err)
		}
		pages[chrome] = current
		return current
	}
	call := func(chrome *chromium.Session, page, method string, params, out any) {
		t.Helper()
		if err := fixturePage(chrome, page).Call(ctx, method, params, out); err != nil {
			t.Fatalf("Chromium %s on %s: %v", method, page, err)
		}
	}
	evaluate := func(chrome *chromium.Session, page, script string, out any) {
		t.Helper()
		var result struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		call(chrome, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &result)
		if len(result.Exception) != 0 {
			t.Fatalf("JavaScript exception for %s: %s", script, result.Exception)
		}
		if err := json.Unmarshal(result.Result.Value, out); err != nil {
			t.Fatalf("decode JavaScript result for %s (%s): %v", script, result.Result.Value, err)
		}
	}
	waitFor := func(chrome *chromium.Session, page, script string) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			var result struct {
				Result struct {
					Value bool `json:"value"`
				} `json:"result"`
			}
			if fixturePage(chrome, page).Call(ctx, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &result) == nil && result.Result.Value {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		var diagnostic string
		evaluate(chrome, page, `JSON.stringify({url:location.href,title:document.title,ready:document.readyState,body:document.body.innerText.slice(0,400)})`, &diagnostic)
		t.Fatalf("timed out waiting for browser state: %s; current page: %s", script, diagnostic)
	}
	setViewport := func(chrome *chromium.Session, page string, width, height int) {
		t.Helper()
		call(chrome, page, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false}, nil)
		var actual struct{ Width, Height int }
		evaluate(chrome, page, `({Width:innerWidth,Height:innerHeight})`, &actual)
		if actual.Width != width || actual.Height != height {
			t.Fatalf("Chromium viewport mismatch: got %dx%d, want %dx%d", actual.Width, actual.Height, width, height)
		}
	}
	activateVisible := func(chrome *chromium.Session, page, selector string) {
		t.Helper()
		var positioned bool
		evaluate(chrome, page, `(()=>{const e=document.querySelector(`+fmt.Sprintf("%q", selector)+`);if(!e)return false;document.documentElement.style.scrollBehavior='auto';e.scrollIntoView({block:'center',behavior:'instant'});return true})()`, &positioned)
		if !positioned {
			t.Fatalf("visible control not found: %s", selector)
		}
		var point struct {
			Visible bool
			Hit     string
		}
		evaluate(chrome, page, `(()=>{const e=document.querySelector(`+fmt.Sprintf("%q", selector)+`),r=e.getBoundingClientRect(),hit=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return {Visible:r.width>0&&r.height>0&&r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight,Hit:hit?.outerHTML.slice(0,180)||''}})()`, &point)
		if !point.Visible {
			t.Fatalf("control is not visible after scrolling: %s", selector)
		}
		if !strings.Contains(point.Hit, "data-open-details") && strings.Contains(selector, "data-open-details") {
			t.Fatalf("visible recording shortcut is covered at its pointer target: %s", point.Hit)
		}
		var activated bool
		evaluate(chrome, page, `(()=>{const e=document.querySelector(`+fmt.Sprintf("%q", selector)+`);e.click();return true})()`, &activated)
		if !activated {
			t.Fatalf("visible control could not be activated: %s", selector)
		}
	}
	press := func(chrome *chromium.Session, page, key, code string, vk int) {
		t.Helper()
		for _, typ := range []string{"keyDown", "keyUp"} {
			call(chrome, page, "Input.dispatchKeyEvent", map[string]any{"type": typ, "key": key, "code": code, "windowsVirtualKeyCode": vk}, nil)
		}
	}
	focusVisible := func(chrome *chromium.Session, page, selector string) {
		t.Helper()
		var focused bool
		evaluate(chrome, page, `(()=>{const e=document.querySelector(`+fmt.Sprintf("%q", selector)+`);if(!e)return false;e.scrollIntoView({block:'center',behavior:'instant'});e.focus();return document.activeElement===e})()`, &focused)
		if !focused {
			t.Fatalf("visible control could not receive keyboard focus: %s", selector)
		}
	}
	capture := func(chrome *chromium.Session, page, name string) {
		t.Helper()
		var shot struct {
			Data string `json:"data"`
		}
		call(chrome, page, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &shot)
		data, err := base64.StdEncoding.DecodeString(shot.Data)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceRoot, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	navigate := func(chrome *chromium.Session, current, target string) {
		t.Helper()
		call(chrome, current, "Page.navigate", map[string]any{"url": target}, nil)
	}

	// Wails-style POST redirect: reach and expand the disclosure, then record
	// with actual keyboard input while a real question editor has unsaved work.
	chrome, err := chromium.LaunchFixture(ctx, desktopServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	defer func() {
		if page := pages[chrome]; page != nil {
			_ = page.Close()
		}
	}()
	basePage := desktopServer.URL + "/application"
	page := desktopServer.URL + "/jobs/1?draft=1"
	navigate(chrome, basePage, page+"#prepare")
	waitFor(chrome, page, `document.readyState==='complete'&&!!document.querySelector('[data-record-completion]')&&!!document.querySelector('form.application-question-form textarea')&&!!document.querySelector('form[action$="/cover-letter/save"] textarea')`)
	setViewport(chrome, page, 1100, 800)
	var ignored bool
	evaluate(chrome, page, `(()=>{const c=document.querySelector('[data-theme-choice]');if(c){c.value='light';c.dispatchEvent(new Event('change'))}return true})()`, &ignored)
	var firstLink struct {
		Visible bool
		Text    string
	}
	evaluate(chrome, page, `(()=>{const e=document.querySelector('a[data-open-details="record-submission"]');const r=e.getBoundingClientRect();return {Visible:r.width>0&&r.height>0&&r.left>=0&&r.right<=innerWidth&&r.top>=0&&r.bottom<=innerHeight,Text:e.innerText}})()`, &firstLink)
	if !firstLink.Visible || !strings.Contains(firstLink.Text, "Record completed application") {
		t.Fatalf("recording action is not visibly discoverable in Prepare: %+v", firstLink)
	}
	capture(chrome, page, "submission-prepare-initial.png")
	evaluate(chrome, page, `(()=>{const e=document.querySelector('form.application-question-form textarea');e.focus({preventScroll:true});return true})()`, &ignored)
	call(chrome, page, "Input.insertText", map[string]any{"text": "Synthetic unsaved question edit"}, nil)
	var dirty bool
	evaluate(chrome, page, `document.querySelector('form.application-question-form').dataset.dirty==='true'`, &dirty)
	if !dirty {
		t.Fatal("synthetic question edit did not enter the unsaved state")
	}
	const unsavedLetter = "Synthetic unsaved cover-letter edits remain recoverable after cancellation."
	evaluate(chrome, page, `(()=>{const e=document.querySelector('form[action$="/cover-letter/save"] textarea');e.focus({preventScroll:true});e.select();return true})()`, &ignored)
	call(chrome, page, "Input.insertText", map[string]any{"text": unsavedLetter}, nil)
	var dirtyLetter bool
	evaluate(chrome, page, `document.querySelector('form[action$="/cover-letter/save"]').dataset.dirty==='true'`, &dirtyLetter)
	if !dirtyLetter {
		t.Fatal("synthetic cover-letter edit did not enter the unsaved state")
	}
	focusVisible(chrome, page, `a[data-open-details="record-submission"]`)
	press(chrome, page, "Enter", "Enter", 13)
	waitFor(chrome, page, `(()=>{const e=document.querySelector('#record-submission'),r=e.querySelector('summary').getBoundingClientRect();return e.open&&r.top>=0&&r.bottom<=innerHeight})()`)
	var disclosureState struct {
		Open           bool
		Hash           string
		ScrollY        float64
		Top, Bottom    float64
		ActiveTagName  string
		SummaryVisible bool
	}
	evaluate(chrome, page, `(()=>{const e=document.querySelector('#record-submission'),r=e.querySelector('summary').getBoundingClientRect();return {Open:e.open,Hash:location.hash,ScrollY:scrollY,Top:r.top,Bottom:r.bottom,ActiveTagName:document.activeElement.tagName,SummaryVisible:r.top>=0&&r.bottom<=innerHeight}})()`, &disclosureState)
	if !disclosureState.Open || !disclosureState.SummaryVisible {
		capture(chrome, page, "submission-after-record-link.png")
		t.Fatalf("keyboard activation of the visible recording link did not reveal the disclosure: %+v", disclosureState)
	}
	capture(chrome, page, "submission-1100x800-expanded.png")
	setViewport(chrome, page, 700, 550)
	capture(chrome, page, "submission-700x550-expanded.png")
	var scrollable bool
	evaluate(chrome, page, `document.documentElement.scrollHeight>innerHeight`, &scrollable)
	if !scrollable {
		t.Fatal("small Prepare viewport does not allow scrolling")
	}
	// The native summary toggles from the keyboard. The next form controls are
	// then reached in source order: notes, required confirmation, submit.
	focusVisible(chrome, page, `#record-submission>summary`)
	press(chrome, page, " ", "Space", 32)
	waitFor(chrome, page, `document.querySelector('#record-submission').open===false`)
	press(chrome, page, " ", "Space", 32)
	waitFor(chrome, page, `document.querySelector('#record-submission').open===true`)
	capture(chrome, page, "submission-700x550-expanded.png")
	press(chrome, page, "Tab", "Tab", 9)
	var active string
	evaluate(chrome, page, `document.activeElement.id`, &active)
	if active != "notes" {
		t.Fatalf("keyboard did not reach notes after the disclosure: active=%q", active)
	}
	var notesVisible bool
	evaluate(chrome, page, `(()=>{const r=document.querySelector('#notes').getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight})()`, &notesVisible)
	if !notesVisible {
		t.Fatal("keyboard focus did not scroll notes into view at 700x550")
	}
	call(chrome, page, "Input.insertText", map[string]any{"text": "Synthetic confirmation SG-1100"}, nil)
	press(chrome, page, "Tab", "Tab", 9)
	var checkboxFocused bool
	evaluate(chrome, page, `document.activeElement.name==='confirm'`, &checkboxFocused)
	if !checkboxFocused {
		t.Fatal("keyboard did not reach the explicit submission confirmation")
	}
	var checkboxVisible bool
	evaluate(chrome, page, `(()=>{const r=document.querySelector('[data-record-completion] [name=confirm]').getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight})()`, &checkboxVisible)
	if !checkboxVisible {
		t.Fatal("keyboard focus did not scroll the confirmation into view at 700x550")
	}
	press(chrome, page, " ", "Space", 32)
	press(chrome, page, "Tab", "Tab", 9)
	var buttonFocused bool
	evaluate(chrome, page, `document.activeElement.matches('[data-record-completion] button[type=submit]')`, &buttonFocused)
	if !buttonFocused {
		t.Fatal("keyboard did not reach the record button")
	}
	var buttonVisible bool
	evaluate(chrome, page, `(()=>{const r=document.querySelector('[data-record-completion] button[type=submit]').getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight})()`, &buttonVisible)
	if !buttonVisible {
		t.Fatal("keyboard focus did not scroll the record button into view at 700x550")
	}
	var checkboxChecked bool
	evaluate(chrome, page, `document.querySelector('[data-record-completion] [name=confirm]').checked`, &checkboxChecked)
	if !checkboxChecked {
		t.Fatal("keyboard did not explicitly confirm employer submission")
	}
	capture(chrome, page, "submission-700x550-action.png")
	evaluate(chrome, page, `(()=>{window.__recordConfirmMessages=[];window.confirm=message=>{window.__recordConfirmMessages.push(message);return false};return true})()`, &dirty)
	press(chrome, page, " ", "Space", 32)
	waitFor(chrome, page, `window.__recordConfirmMessages.length===1`)
	var cancelledState struct {
		OnPrepare   bool
		DirtyAnswer bool
		DirtyLetter bool
		Answer      string
		Letter      string
		Note        string
		Prompt      string
	}
	evaluate(chrome, page, `({OnPrepare:location.pathname==='/jobs/1',DirtyAnswer:document.querySelector('form.application-question-form').dataset.dirty==='true',DirtyLetter:document.querySelector('form[action$="/cover-letter/save"]').dataset.dirty==='true',Answer:document.querySelector('form.application-question-form textarea').value,Letter:document.querySelector('form[action$="/cover-letter/save"] textarea').value,Note:document.querySelector('#notes').value,Prompt:window.__recordConfirmMessages[0]})`, &cancelledState)
	if !cancelledState.OnPrepare || !cancelledState.DirtyAnswer || !cancelledState.DirtyLetter || !strings.Contains(cancelledState.Answer, "Synthetic unsaved question edit") || cancelledState.Letter != unsavedLetter || cancelledState.Note != "Synthetic confirmation SG-1100" || !strings.Contains(cancelledState.Prompt, "unsaved answer or letter edits") {
		t.Fatalf("cancel did not preserve the recoverable unsaved form: %+v", cancelledState)
	}
	evaluate(chrome, page, `(()=>{window.confirm=()=>true;return true})()`, &dirty)
	press(chrome, page, " ", "Space", 32)
	waitFor(chrome, page, `location.hash==='#history'&&document.body.textContent.includes('Submission recorded locally.')`)
	job, err := a.jobByID(1)
	if err != nil || job.Stage != StageApplied {
		t.Fatalf("desktop submission did not advance to Applied: %+v %v", job, err)
	}
	persistedLetter, err := a.coverLetterByID(1, 1)
	if err != nil || persistedLetter.Body != savedLetter {
		t.Fatalf("recording unexpectedly changed the saved cover letter: %+v %v", persistedLetter, err)
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 || history[0].ResumeID != resume.ID || history[0].ResumeSHA256 != resume.SHA256 || history[0].Notes != "Synthetic confirmation SG-1100" {
		t.Fatalf("desktop submission history mismatch: %+v %v", history, err)
	}
	var successActions struct {
		History, Next bool
	}
	evaluate(chrome, page, `({History:document.querySelector('#history').textContent.includes('Synthetic confirmation SG-1100'),Next:!!document.querySelector('#history form[action="/jobs/1/next"] button')})`, &successActions)
	if !successActions.History || !successActions.Next {
		t.Fatalf("history confirmation or next-job action missing: %+v", successActions)
	}
	capture(chrome, page, "submission-history-success.png")
	activateVisible(chrome, page, `#history form[action="/jobs/1/next"] button`)
	waitFor(chrome, desktopServer.URL+"/jobs/3", `location.pathname==='/jobs/3'&&location.hash==='#prepare'`)
	page = desktopServer.URL + "/jobs/3"
	activateVisible(chrome, page, `.prepare-handoff a[href="/jobs"]`)
	waitFor(chrome, desktopServer.URL+"/jobs", `location.pathname==='/jobs'&&document.readyState==='complete'`)
	page = desktopServer.URL + "/jobs"
	var scoreboard struct {
		Remaining, Overall string
		Progress, Maximum  float64
	}
	evaluate(chrome, page, `({Remaining:document.querySelector('[data-remaining-count]').textContent.trim(),Overall:document.querySelector('[data-recorded-overall]').textContent.trim(),Progress:Number(document.querySelector('.queue-progress-bar').value),Maximum:Number(document.querySelector('.queue-progress-bar').max)})`, &scoreboard)
	if !strings.Contains(scoreboard.Remaining, "2") || scoreboard.Overall != "1" || scoreboard.Progress != 1 || scoreboard.Maximum != 3 {
		t.Fatalf("Opportunities scoreboard did not update after recording: %+v", scoreboard)
	}
	capture(chrome, page, "submission-opportunities-scoreboard.png")

	// No-JavaScript browser route: native disclosure, explicit checkbox, no
	// notes, and the ordinary 303 POST/Redirect/GET path all remain usable.
	noJS, err := chromium.LaunchFixture(ctx, browserServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer noJS.Close()
	defer func() {
		if page := pages[noJS]; page != nil {
			_ = page.Close()
		}
	}()
	browserBase := browserServer.URL + "/application"
	page = browserServer.URL + "/jobs/2"
	call(noJS, browserBase, "Emulation.setScriptExecutionDisabled", map[string]any{"value": true}, nil)
	navigate(noJS, browserBase, page+"#prepare")
	waitFor(noJS, page, `document.readyState==='complete'&&!!document.querySelector('[data-record-completion]')`)
	setViewport(noJS, page, 700, 550)
	activateVisible(noJS, page, `a[data-open-details="record-submission"]`)
	activateVisible(noJS, page, `#record-submission>summary`)
	activateVisible(noJS, page, `[data-record-completion] [name=confirm]`)
	activateVisible(noJS, page, `[data-record-completion] button[type=submit]`)
	waitFor(noJS, page, `location.hash==='#history'&&document.body.textContent.includes('Submission recorded locally.')`)
	history, err = a.jobSubmissions(2)
	if err != nil || len(history) != 1 || history[0].ResumeID != 0 || history[0].Notes != "" {
		t.Fatalf("no-JS, no-resume submission failed: %+v %v", history, err)
	}

	// A missing selected resume rejects the stale review without recording,
	// returns through the real browser to an actionable explanation, and lets
	// the user clear the unavailable selection before recording.
	recovery, err := chromium.LaunchFixture(ctx, desktopServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer recovery.Close()
	defer func() {
		if page := pages[recovery]; page != nil {
			_ = page.Close()
		}
	}()
	page = desktopServer.URL + "/jobs/3"
	navigate(recovery, desktopServer.URL+"/application", page+"#prepare")
	waitFor(recovery, page, `document.readyState==='complete'&&!!document.querySelector('[data-record-completion]')`)
	if err := os.Remove(filepath.Join(a.resumeDir, resume.StoredName)); err != nil {
		t.Fatal(err)
	}
	activateVisible(recovery, page, `a[data-open-details="record-submission"]`)
	focusVisible(recovery, page, `#record-submission #notes`)
	call(recovery, page, "Input.insertText", map[string]any{"text": "Synthetic stale-recovery note"}, nil)
	var typedNote string
	evaluate(recovery, page, `document.querySelector('#record-submission #notes').value`, &typedNote)
	if typedNote != "Synthetic stale-recovery note" {
		t.Fatalf("controlled note entry failed: %q", typedNote)
	}
	activateVisible(recovery, page, `#record-submission [name=confirm]`)
	activateVisible(recovery, page, `#record-submission button[type=submit]`)
	stalePage := desktopServer.URL + "/jobs/3?submission=stale"
	waitFor(recovery, stalePage, `location.search==='?submission=stale'&&!!document.querySelector('.submission-recovery[role=alert]')`)
	var recoveryCopy struct {
		Notice, ResumeLink, FormLink, Notes, StoredNotes string
	}
	evaluate(recovery, stalePage, `({Notice:document.querySelector('.submission-recovery').textContent,ResumeLink:document.querySelector('.submission-recovery a[href="#prepare-resume-heading"]')?.textContent,FormLink:document.querySelector('.submission-recovery a[data-open-details="record-submission"]')?.textContent,Notes:document.querySelector('#record-submission #notes')?.value,StoredNotes:sessionStorage.getItem('yourmove:submission-note:3')})`, &recoveryCopy)
	if !strings.Contains(recoveryCopy.Notice, "Your confirmation was not recorded") || recoveryCopy.ResumeLink == "" || recoveryCopy.FormLink == "" {
		t.Fatalf("stale recording has no direct recovery path: %+v", recoveryCopy)
	}
	if recoveryCopy.Notes != "Synthetic stale-recovery note" {
		t.Fatalf("stale recording discarded optional notes: %+v", recoveryCopy)
	}
	if history, err := a.jobSubmissions(3); err != nil || len(history) != 0 {
		t.Fatalf("stale resume was recorded: %+v %v", history, err)
	}
	activateVisible(recovery, stalePage, `a[href="#prepare-resume-heading"]`)
	var noResumeSelected bool
	evaluate(recovery, stalePage, `(()=>{const e=document.querySelector('#resume_id');e.value='0';e.dispatchEvent(new Event('change',{bubbles:true}));return e.value==='0'})()`, &noResumeSelected)
	if !noResumeSelected {
		t.Fatal("recovery select did not accept the no-resume choice")
	}
	activateVisible(recovery, stalePage, `form[action="/jobs/3/resume"] button[type=submit]`)
	waitFor(recovery, page, `location.hash==='#prepare'&&document.querySelector('#resume_id').value==='0'`)
	activateVisible(recovery, page, `a[data-open-details="record-submission"]`)
	activateVisible(recovery, page, `#record-submission [name=confirm]`)
	activateVisible(recovery, page, `#record-submission button[type=submit]`)
	waitFor(recovery, page, `location.hash==='#history'&&document.body.textContent.includes('Submission recorded locally.')`)
	history, err = a.jobSubmissions(3)
	if err != nil || len(history) != 1 || history[0].ResumeID != 0 || history[0].Notes != "Synthetic stale-recovery note" {
		t.Fatalf("resume recovery did not permit a valid no-resume submission: %+v %v", history, err)
	}

	// Force a real transaction failure through a synthetic SQLite trigger. The
	// Wails page must report uncertainty and offer both History and a retry path;
	// after the fault is removed, the user can record the same completed action.
	if err := a.saveJob(Job{URL: "https://jobs.ashbyhq.com/fixture/transaction-failure", Title: "Synthetic transaction recovery"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`CREATE TRIGGER fixture_fail_submission BEFORE INSERT ON submissions BEGIN SELECT RAISE(ABORT, 'synthetic fixture transaction failure'); END`); err != nil {
		t.Fatal(err)
	}
	failedPage := desktopServer.URL + "/jobs/4"
	navigate(recovery, page, failedPage+"#prepare")
	page = failedPage
	waitFor(recovery, page, `document.readyState==='complete'&&!!document.querySelector('[data-record-completion]')`)
	activateVisible(recovery, page, `a[data-open-details="record-submission"]`)
	focusVisible(recovery, page, `#record-submission #notes`)
	call(recovery, page, "Input.insertText", map[string]any{"text": "Synthetic database-recovery note"}, nil)
	activateVisible(recovery, page, `#record-submission [name=confirm]`)
	activateVisible(recovery, page, `#record-submission button[type=submit]`)
	failedRecoveryPage := desktopServer.URL + "/jobs/4?submission=failed"
	waitFor(recovery, failedRecoveryPage, `location.search==='?submission=failed'&&!!document.querySelector('.submission-recovery[role=alert]')`)
	var failedCopy struct{ Notice, HistoryLink, RetryLink, Notes string }
	evaluate(recovery, failedRecoveryPage, `({Notice:document.querySelector('.submission-recovery').textContent,HistoryLink:document.querySelector('.submission-recovery a[href="#history"]')?.textContent,RetryLink:document.querySelector('.submission-recovery a[data-open-details="record-submission"]')?.textContent,Notes:document.querySelector('#record-submission #notes')?.value})`, &failedCopy)
	if !strings.Contains(failedCopy.Notice, "Check submission history") || failedCopy.HistoryLink == "" || failedCopy.RetryLink == "" {
		t.Fatalf("failed recording did not preserve a user recovery path: %+v", failedCopy)
	}
	if failedCopy.Notes != "Synthetic database-recovery note" {
		t.Fatalf("failed recording discarded optional notes: %+v", failedCopy)
	}
	capture(recovery, failedRecoveryPage, "submission-recording-failure-recovery.png")
	failedJob, err := a.jobByID(4)
	if err != nil || failedJob.Stage == StageApplied {
		t.Fatalf("failed transaction falsely marked the job applied: %+v %v", failedJob, err)
	}
	if history, err := a.jobSubmissions(4); err != nil || len(history) != 0 {
		t.Fatalf("failed transaction left a partial submission: %+v %v", history, err)
	}
	if _, err := a.db.Exec(`DROP TRIGGER fixture_fail_submission`); err != nil {
		t.Fatal(err)
	}
	page = failedRecoveryPage
	activateVisible(recovery, page, `a[data-open-details="record-submission"]`)
	activateVisible(recovery, page, `#record-submission [name=confirm]`)
	activateVisible(recovery, page, `#record-submission button[type=submit]`)
	waitFor(recovery, page, `location.hash==='#history'&&document.body.textContent.includes('Submission recorded locally.')`)
	if history, err := a.jobSubmissions(4); err != nil || len(history) != 1 || history[0].ResumeID != 0 || history[0].Notes != "Synthetic database-recovery note" {
		t.Fatalf("retry after transaction recovery failed: %+v %v", history, err)
	}
	t.Log("Real Chromium showed the completion action at 1100x800 and 700x550, exercised keyboard submission in Wails, verified no-JavaScript browser submission, preserved dirty answer and cover-letter edits on cancellation, applied history and next-job navigation, updated the Opportunities scoreboard, and recovered from missing-resume and transaction failures.")
}
