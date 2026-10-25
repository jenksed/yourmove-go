//go:build qualification

package app

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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Rendered synthetic state qualification complements the real employer-fixture
// transfers in TestControlledContactJourney and TestControlledCombinedContinuation.
func TestControlledScopeFirstWorkbench(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in isolated Chromium qualification")
	}
	a, f := scopeFirstWorkspace(t)
	role, err := a.importResume(strings.Repeat("long-support-engineer-", 7)+"resume.docx", bytes.NewReader(sampleDOCXForPreview(t, "Synthetic long filename evidence")))
	if err != nil {
		t.Fatal(err)
	}
	if r := postApp(newMux(a), "/jobs/1/resume", mapValues("resume_id", fmt.Sprint(role.ID)), "", ""); r.Code != 303 {
		t.Fatal(r.Code)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Alex", LastName: "Example", Email: "alex@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	var noScripts atomic.Bool
	router := http.NewServeMux()
	for _, path := range []string{"/static/ui.js", "/static/inspection.js"} {
		router.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if noScripts.Load() {
				w.Header().Set("Content-Type", "text/javascript")
				return
			}
			mux.ServeHTTP(w, r)
		})
	}
	router.HandleFunc("GET /jobs/1/application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1?draft=1#prepare", 303) })
	router.Handle("/", mux)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, server.URL+"/jobs/1")
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	page := server.URL + "/jobs/1?draft=1"
	evaluate := func(script string, out any) error {
		var result struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true}, &result); err != nil {
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
	wait := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
			var ok bool
			if evaluate(script, &ok) == nil && ok {
				return
			}
			time.Sleep(40 * time.Millisecond)
		}
		t.Fatal("rendered wait failed", script)
	}
	assert := func(script string) {
		t.Helper()
		var ok bool
		if err := evaluate(script, &ok); err != nil || !ok {
			t.Fatal("rendered assertion failed", script, err)
		}
	}
	run := func(script string) {
		t.Helper()
		if err := evaluate(script, nil); err != nil {
			t.Fatal(err)
		}
	}
	press := func(key, code string, number int) {
		t.Helper()
		for _, kind := range []string{"keyDown", "keyUp"} {
			if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": kind, "key": key, "code": code, "windowsVirtualKeyCode": number}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	clickWithMouse := func(selector string) {
		t.Helper()
		run(`document.querySelector(` + fmt.Sprintf("%q", selector) + `).scrollIntoView({block:'center',behavior:'instant'})`)
		var point struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		if err := evaluate(`(()=>{const r=document.querySelector(`+fmt.Sprintf("%q", selector)+`).getBoundingClientRect();return{x:r.left+r.width/2,y:r.top+r.height/2}})()`, &point); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"mousePressed", "mouseReleased"} {
			if err := chrome.FixtureCall(ctx, page, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": point.X, "y": point.Y, "button": "left", "clickCount": 1}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	navigate := func(path string) {
		t.Helper()
		run(`window.scopePreviousDocument=true`)
		target := server.URL + strings.Split(path, "#")[0]
		method, params := "Page.navigate", map[string]any{"url": server.URL + path}
		if target == page {
			method, params = "Page.reload", map[string]any{"ignoreCache": true}
		}
		if err := chrome.FixtureCall(ctx, page, method, params, nil); err != nil {
			t.Fatal(err)
		}
		page = target
		wait(`!window.scopePreviousDocument && document.readyState==='complete' && !!document.querySelector('#letter-editor')`)
	}
	wait(`document.readyState==='complete' && !!document.querySelector('#letter-body')`)
	// The letter editor is present in the DOM while its workspace tab may be
	// hidden. Select the actual activity before exercising focus or typing.
	run(`document.querySelector('.workspace-tab[data-tab-target="cover-letter"]').click()`)
	wait(`!document.querySelector('#cover-letter').hidden && document.querySelector('#letter-body').getClientRects().length>0`)
	evidenceDir := os.Getenv("YOURMOVE_WRITING_EVIDENCE")
	if evidenceDir == "" {
		evidenceDir = "/private/tmp/yourmove-writing-workspace-evidence"
	}
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	savedLetterShot, err := chrome.FixtureScreenshotViewport(ctx, page, 900, 700, "#letter-editor", "dark")
	if err != nil {
		t.Fatal(err)
	}
	savedLetterImage, err := base64.StdEncoding.DecodeString(savedLetterShot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "saved-letter-900x700-dark.png"), savedLetterImage, 0600); err != nil {
		t.Fatal(err)
	}
	// Switching activities keeps a dirty saved letter in its original form.
	run(`(()=>{const e=document.querySelector('#letter-body');window.originalSavedLetter=e.defaultValue;e.value+=' Unsaved cover-letter text stays local.';e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	assert(`document.querySelector('#letter-editor').open && document.querySelector('[data-ai-draft] [data-answer-status]').textContent==='Unsaved changes'`)
	run(`document.querySelector('#application-answers-workspace > summary').click()`)
	assert(`document.querySelector('#application-answers-workspace').open`)
	assert(`!document.querySelector('#letter-editor').open`)
	assert(`document.querySelector('#letter-body').value.includes('Unsaved cover-letter text stays local.')`)
	assert(`document.querySelector('[data-ai-draft] [data-answer-status]').textContent==='Unsaved changes'`)
	run(`document.querySelector('#letter-editor > summary').click()`)
	assert(`document.querySelector('#letter-editor').open && !document.querySelector('#application-answers-workspace').open && document.querySelector('#letter-body').value.includes('Unsaved cover-letter text stays local.')`)
	run(`(()=>{const e=document.querySelector('#letter-body');e.value=window.originalSavedLetter;e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	assert(`document.querySelector('[data-ai-draft] [data-answer-status]').textContent==='Saved locally · unsaved edits are excluded from transfer and download.'`)
	// Word guidance remains editorial and never constrains edits.
	assert(`document.querySelector('[data-ai-word-count]').textContent.includes('target 120–160; ceiling 180')`)
	run(`(()=>{const e=document.querySelector('#letter-body');e.value=('word ').repeat(185);e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	assert(`document.querySelector('[data-ai-word-count]').textContent.includes('185 words · over editorial ceiling') && document.querySelector('#letter-body').checkValidity()`)
	// Existing draft opens intentionally. Enter a real edit and save via the form.
	assert(`document.querySelector('#letter-editor').open`)
	const edited = "Dear Hiring Team, this is the selected draft edited through the rendered local workspace."
	run(`(()=>{const e=document.querySelector('#letter-body');e.focus();e.select()})()`)
	assert(`document.activeElement===document.querySelector('#letter-body') && document.querySelector('#letter-body').getClientRects().length>0`)
	if err := chrome.FixtureCall(ctx, page, "Input.insertText", map[string]any{"text": edited}, nil); err != nil {
		t.Fatal(err)
	}
	assert(`document.querySelector('#letter-body').value===` + fmt.Sprintf("%q", edited))
	assert(`document.querySelector('[data-ai-draft] [data-answer-status]').textContent==='Unsaved changes' && !document.querySelector('.transfer-letter-scope').textContent.includes('edited through')`)
	assert(`!document.querySelector('.local-save-confirmation')`)
	clickWithMouse(`[data-ai-draft] button[type="submit"]`)
	page = server.URL + "/jobs/1?cover=saved&draft=1"
	wait(`document.readyState==='complete' && !!document.querySelector('.local-save-confirmation') && document.activeElement===document.querySelector('.local-save-confirmation [data-focus-after-save][tabindex="-1"]')`)
	assert(`location.hash==='#prepare'`)
	assert(`(()=>{const target=document.querySelector('.local-save-confirmation [data-focus-after-save][tabindex="-1"]');return document.activeElement===target&&target.textContent.includes('Saved locally')})()`)
	assert(`document.activeElement.getClientRects().length>0`)
	var focusPosition struct {
		Top, Bottom, InnerHeight, ScrollY float64
		Text                              string `json:"text"`
	}
	if err := evaluate(`(()=>{const r=document.activeElement.getBoundingClientRect();return{top:r.top,bottom:r.bottom,innerHeight,scrollY,text:document.activeElement.textContent}})()`, &focusPosition); err != nil {
		t.Fatal(err)
	}
	t.Logf("cover save focus position: %+v", focusPosition)
	assert(`(()=>{const r=document.activeElement.getBoundingClientRect();return r.top>=6&&r.bottom<=innerHeight-6})()`)
	assert(`document.activeElement.getBoundingClientRect().width<document.querySelector('#application-action').getBoundingClientRect().width`)
	assert(`!document.querySelector('#letter-editor').open`)
	var savedScope string
	if err := evaluate(`document.querySelector('.transfer-letter-scope')?.textContent`, &savedScope); err != nil || savedScope != edited {
		t.Fatalf("saved transfer scope = %q, want exact edited text %q (evaluation error: %v)", savedScope, edited, err)
	}
	savedDraft, err := a.coverLetterByID(1, 1)
	if err != nil || savedDraft.Body != edited {
		t.Fatalf("persisted cover letter = %q, want exact edited text %q (load error: %v)", savedDraft.Body, edited, err)
	}
	if got := f.calls + f.contacts + f.resumes + f.answers; got != 0 {
		t.Fatalf("saving a local letter triggered %d employer browser actions", got)
	}
	t.Log("visible Cover letter activity → exact edit/save → Prepare navigation, closed editor, persisted transfer scope; zero employer actions")

	dir := os.Getenv("YOURMOVE_SCOPE_STATES")
	if dir == "" {
		dir = "/private/tmp/yourmove-scope-first/states"
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	capture := func(name string) {
		t.Helper()
		for _, width := range []int{1280, 1050, 800, 700, 390} {
			for _, theme := range []string{"light", "dark"} {
				shot, err := chrome.FixtureScreenshotAt(ctx, page, width, "#prepare", theme)
				if err != nil {
					t.Fatal(name, err)
				}
				data, err := base64.StdEncoding.DecodeString(shot)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%d-%s.png", name, width, theme)), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	capture("first-fill")
	// Keyboard reopening preserves the selected saved draft; a second edit/save
	// still returns to the same canonical context.
	run(`document.querySelector('.workspace-tab[data-tab-target="cover-letter"]').click()`)
	wait(`!document.querySelector('#cover-letter').hidden && document.querySelector('#letter-body').getClientRects().length>0`)
	run(`if(document.querySelector('#letter-editor').open)document.querySelector('#letter-editor > summary').click()`)
	assert(`!document.querySelector('#letter-editor').open`)
	run(`document.querySelector('#letter-editor > summary').focus()`)
	for _, typ := range []string{"keyDown", "keyUp"} {
		if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": typ, "key": " ", "code": "Space", "windowsVirtualKeyCode": 32}, nil); err != nil {
			t.Fatal(err)
		}
	}
	assert(`document.querySelector('#letter-editor').open && document.querySelector('#letter-body').value===` + fmt.Sprintf("%q", edited))
	run(`(()=>{const e=document.querySelector('#letter-body');e.focus();e.select()})()`)
	if err := chrome.FixtureCall(ctx, page, "Input.insertText", map[string]any{"text": edited + " Saved again."}, nil); err != nil {
		t.Fatal(err)
	}
	run(`document.querySelector('[data-ai-draft] button[type=submit]').focus()`)
	press(" ", "Space", 32)
	wait(`document.readyState==='complete' && !document.querySelector('#letter-editor').open && document.querySelector('.transfer-letter-scope').textContent.endsWith('Saved again.')`)
	assert(`document.activeElement===document.querySelector('.local-save-confirmation [data-focus-after-save][tabindex="-1"]')&&document.activeElement.matches(':focus-visible')`)
	assert(`document.activeElement.getBoundingClientRect().width<document.querySelector('#application-action').getBoundingClientRect().width`)
	assertFocusOutlineContrast(t, evaluate, "light")
	assertFocusOutlineContrast(t, evaluate, "dark")
	run(`document.documentElement.dataset.theme='light'`)
	assert(`!document.querySelector('.resume-change').open`)
	run(`document.querySelector('.resume-change > summary').focus()`)
	for _, typ := range []string{"keyDown", "keyUp"} {
		if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": typ, "key": " ", "code": "Space", "windowsVirtualKeyCode": 32}, nil); err != nil {
			t.Fatal(err)
		}
	}
	assert(`document.querySelector('.resume-change').open && !!document.querySelector('#resume_id').getClientRects().length`)

	// These are synthetic session states, not claims of employer operations.
	a.browsers.mu.Lock()
	a.browsers.sessions = map[int64]readOnlySession{1: f}
	a.browsers.states = map[int64]browserRunState{1: {Phase: "detected", Destination: testAshbyPosting}}
	a.browsers.handoffs = map[int64]*resumeHandoff{1: {status: "selected", name: "previously-selected.pdf", message: "File selected in Chromium; employer acceptance not established", contactStatus: "complete"}}
	a.browsers.mu.Unlock()
	field := schema.Field{ID: "browser:why", Label: "Why this team?", Kind: "textarea"}
	if _, err := a.saveInspection(1, schema.Result{Source: "browser", Platform: "ashby", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	if r := postApp(mux, "/jobs/1/answers/save", mapValues("destination", testAshbyPosting, "question_key", questionKey(testAshbyPosting, "ashby", field), "revision", "0", "answer", "I enjoy resolving difficult technical incidents."), "", ""); r.Code != 303 {
		t.Fatal(r.Code)
	}
	navigate("/jobs/1?cover=saved&draft=1#prepare")
	assert(`document.querySelector('form[action$="/continue"] button').textContent==='Continue application' && !document.querySelector('form[action$="/continue"] [name=include_resume]') && document.querySelector('form[action$="/continue"]').textContent.includes('I enjoy resolving difficult technical incidents.')`)
	capture("combined")
	for _, status := range []string{"complete", "attention"} {
		a.browsers.mu.Lock()
		a.browsers.letters = map[int64]*coverLetterTransfer{1: {status: status, draftID: 1, body: "Previously attempted letter", message: "Synthetic prior attachment result; inspect manually."}}
		a.browsers.mu.Unlock()
		navigate("/jobs/1?cover=saved&draft=1#prepare")
		assert(`!document.querySelector('form[action$="/continue"] [name=include_letter]') && document.querySelector('form[action$="/continue"] button').textContent==='Continue application'`)
		capture("letter-" + status)
	}
	a.browsers.mu.Lock()
	a.browsers.sessions = nil
	a.browsers.handoffs = nil
	a.browsers.letters = nil
	a.browsers.states = map[int64]browserRunState{1: {Phase: "interrupted", Message: "Browser unavailable; browser-only work may be lost."}}
	a.browsers.mu.Unlock()
	navigate("/jobs/1?cover=saved&draft=1#prepare")
	assert(`!document.querySelector('form[action$="/continue"]') && document.querySelector('#letter-body').value.endsWith('Saved again.')`)
	capture("browser-unavailable")
	a.browsers.mu.Lock()
	a.browsers.states = nil
	a.browsers.mu.Unlock()
	if err := a.assignResume(1, 0); err != nil {
		t.Fatal(err)
	}
	navigate("/jobs/1?cover=saved&draft=1#prepare")
	assert(`!document.querySelector('.resume-change') && !!document.querySelector('#resume_id').getClientRects().length && !document.querySelector('form[action$="/continue"]')`)
	capture("missing-resume")
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	longLetter := "Dear Hiring Team,\n\n" + strings.Repeat("Verified support experience and careful communication. ", 160) + "\nComplete final line of the saved letter."
	if r := postApp(mux, "/jobs/1/cover-letter/save", mapValues("draft_id", "1", "body", longLetter), "", ""); r.Code != 303 {
		t.Fatal(r.Code)
	}
	navigate("/jobs/1?cover=saved&draft=1#prepare")
	assert(`(()=>{const e=document.querySelector('.transfer-letter-scope');return e.tabIndex===0 && e.scrollHeight>e.clientHeight && e.textContent.endsWith('Complete final line of the saved letter.')})()`)
	capture("long-letter")
	run(`document.querySelector('.transfer-letter-review > summary').click()`)
	run(`document.querySelector('.transfer-letter-scope').focus()`)
	assert(`document.activeElement===document.querySelector('.transfer-letter-scope')`)
	for _, typ := range []string{"keyDown", "keyUp"} {
		if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": typ, "key": "PageDown", "code": "PageDown", "windowsVirtualKeyCode": 34}, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait(`document.querySelector('.transfer-letter-scope').scrollTop>0`)
	// No-JS renders native disclosures and authoritative forms, including the
	// saved return and complete letter body. No alternate transfer algorithm.
	noScripts.Store(true)
	navigate("/jobs/1?cover=saved&draft=1#prepare")
	assert(`!document.documentElement.classList.contains('tabs-enabled') && !document.querySelector('#letter-editor').open && document.querySelector('.transfer-letter-scope').textContent.endsWith('Complete final line of the saved letter.')`)
	capture("no-js")
	if f.calls+f.contacts+f.resumes+f.answers != 0 {
		t.Fatal("local rendered interactions caused a browser transfer")
	}
	t.Log("keyboard letter/resume disclosures, repeat save, full keyboard-scrollable long scope, no-JS forms and all five light/dark widths passed; synthetic browser operations: 0")
}
