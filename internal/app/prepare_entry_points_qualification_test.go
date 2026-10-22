//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
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

// The fixture uses temporary SQLite storage and a loopback-only application.
// It exercises the real page scripts without opening an employer destination.
func TestControlledPrepareEntryPoints(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_PREPARE") != "1" {
		t.Skip("opt-in isolated Prepare entry-point qualification")
	}
	a, _ := testWorkspace(t)
	job := Job{
		URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io", Company: "Synthetic Northstar",
		Title: "Support Engineer", Description: "Resolve customer issues with a technical support team.",
	}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(1, schema.Result{
		Platform: "greenhouse", Source: "public_api", Completeness: "public_schema", ApplicationURL: job.URL,
		Fields: []schema.Field{{ID: "application:why", Label: "Why this support role?", Kind: "textarea", Required: true}},
	}); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var noScripts atomic.Bool
	var noScriptRequests atomic.Int32
	mux := newMux(a)
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if noScripts.Load() && (r.URL.Path == "/static/ui.js" || r.URL.Path == "/static/inspection.js") {
			noScriptRequests.Add(1)
			w.Header().Set("Content-Type", "text/javascript")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})}}
	server.Start()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	session, err := chromium.LaunchFixture(ctx, server.URL+"/jobs/1#prepare")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	defer a.browsers.closeAll()
	page := server.URL + "/jobs/1"
	connection, err := session.FixturePage(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	mode := os.Getenv("YOURMOVE_PREPARE_ENTRY_CAPTURE_MODE")
	if mode == "" {
		mode = "after"
	}
	evaluate := func(script string, out any) error {
		var result struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		if err := connection.Call(ctx, "Runtime.evaluate", map[string]any{
			"expression": script, "returnByValue": true, "awaitPromise": true,
		}, &result); err != nil {
			return err
		}
		if result.Exception != nil {
			return fmt.Errorf("JavaScript exception: %v", result.Exception)
		}
		if out != nil {
			return json.Unmarshal(result.Result.Value, out)
		}
		return nil
	}
	wait := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; {
			var ready bool
			if evaluate(script, &ready) == nil && ready {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", script)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	if mode == "before" {
		wait(`document.readyState==='complete' && !!document.querySelector('#letter-editor')`)
	} else {
		wait(`document.readyState==='complete' && !!document.querySelector('[data-prepare-question-entry]') && !!document.querySelector('[data-open-details="letter-editor"]')`)
	}
	prefix := os.Getenv("YOURMOVE_PREPARE_ENTRY_EVIDENCE")
	if prefix == "" {
		prefix = filepath.Join(os.TempDir(), "yourmove-prepare-entry-points", mode)
	}
	if err := os.MkdirAll(prefix, 0700); err != nil {
		t.Fatal(err)
	}
	captureWorkspace := func(name string, width, height int, selector, theme string) {
		t.Helper()
		shot, err := session.FixtureScreenshotViewport(ctx, page, width, height, selector, theme)
		if err != nil {
			t.Fatalf("capture %s: %v", name, err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(prefix, name+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("writing workspace screenshot", filepath.Join(prefix, name+".png"))
	}
	for _, viewport := range []struct{ width, height int }{{1100, 800}, {900, 700}} {
		if err := connection.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
			"width": viewport.width, "height": viewport.height, "deviceScaleFactor": 1, "mobile": false,
		}, nil); err != nil {
			t.Fatal(err)
		}
		var topState struct {
			Overflow bool `json:"overflow"`
			Actions  []struct {
				Text   string `json:"text"`
				Top    int    `json:"top"`
				Bottom int    `json:"bottom"`
				Fits   bool   `json:"fits"`
			} `json:"actions"`
		}
		if err := evaluate(`(()=>{window.scrollTo(0,0);document.documentElement.style.scrollBehavior='auto';return {overflow:document.documentElement.scrollWidth>innerWidth,actions:[...document.querySelectorAll('.prepare-entry-action')].map(e=>({text:e.innerText,top:Math.round(e.getBoundingClientRect().top),bottom:Math.round(e.getBoundingClientRect().bottom),fits:e.scrollWidth<=e.clientWidth&&e.scrollHeight<=e.clientHeight}))}})()`, &topState); err != nil {
			t.Fatal(err)
		}
		if topState.Overflow {
			t.Fatalf("horizontal overflow at %dx%d", viewport.width, viewport.height)
		}
		if mode != "before" {
			if len(topState.Actions) != 2 {
				t.Fatalf("expected two writing entry actions at %dx%d, got %+v", viewport.width, viewport.height, topState.Actions)
			}
			for _, action := range topState.Actions {
				if action.Top < 0 || action.Bottom > viewport.height {
					t.Fatalf("writing action is outside the initial viewport at %dx%d: %+v", viewport.width, viewport.height, action)
				}
				if !action.Fits {
					t.Fatalf("writing action text is clipped at %dx%d: %+v", viewport.width, viewport.height, action)
				}
			}
		}
		var shot struct {
			Data string `json:"data"`
		}
		if err := connection.Call(ctx, "Page.captureScreenshot", map[string]any{
			"format": "png", "captureBeyondViewport": false,
		}, &shot); err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(shot.Data)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("%s-%dx%d.png", mode, viewport.width, viewport.height)
		if err := os.WriteFile(filepath.Join(prefix, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("screenshot", filepath.Join(prefix, name))
	}
	if mode == "before" {
		return
	}

	click := func(selector string) {
		t.Helper()
		encoded, _ := json.Marshal(selector)
		var point struct{ X, Y float64 }
		if err := evaluate(`(()=>{const e=document.querySelector(`+string(encoded)+`);if(!e||!e.getClientRects().length)throw Error('control unavailable');let r=e.getBoundingClientRect();if(r.top<0||r.bottom>innerHeight){e.scrollIntoView({block:'center',behavior:'instant'});r=e.getBoundingClientRect()}return {X:r.x+r.width/2,Y:r.y+r.height/2}})()`, &point); err != nil {
			t.Fatal(err)
		}
		for _, typ := range []string{"mousePressed", "mouseReleased"} {
			if err := connection.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
				"type": typ, "x": point.X, "y": point.Y, "button": "left", "clickCount": 1,
			}, nil); err != nil {
				t.Fatal(err)
			}
		}
	}
	click(`[data-question-panel]:not([hidden]) textarea`)
	if err := connection.Call(ctx, "Input.insertText", map[string]any{"text": "Unsaved answer stays in this editor."}, nil); err != nil {
		t.Fatal(err)
	}
	evaluate(`window.scrollTo(0,0);true`, nil)
	click(`[data-prepare-question-entry]`)
	wait(`document.activeElement===document.querySelector('[data-question-panel]:not([hidden])') && document.querySelector('[data-question-panel]:not([hidden]) textarea')?.value==='Unsaved answer stays in this editor.'`)
	var questionState struct {
		Hash  string `json:"hash"`
		Top   int    `json:"top"`
		Dirty bool   `json:"dirty"`
	}
	if err := evaluate(`(()=>{const q=document.querySelector('[data-question-panel]:not([hidden])');return {hash:location.hash,top:Math.round(q.getBoundingClientRect().top),dirty:q.querySelector('form').dataset.dirty==='true'}})()`, &questionState); err != nil {
		t.Fatal(err)
	}
	if questionState.Hash != "#early-application-questions" || questionState.Top < 0 || !questionState.Dirty {
		t.Fatalf("question action missed the selected editor or changed unsaved state: %+v", questionState)
	}
	captureWorkspace("answers-1100x800-dark", 1100, 800, "#application-materials", "dark")
	captureWorkspace("answers-900x700-dark", 900, 700, "#application-materials", "dark")
	captureWorkspace("answers-750x650-dark", 750, 650, "#application-materials", "dark")

	evaluate(`window.scrollTo(0,0);true`, nil)
	click(`[data-open-details="letter-editor"]`)
	wait(`document.querySelector('#letter-editor')?.open===true && document.activeElement?.id==='letter-studio-title'`)
	var letterState struct {
		Hash        string `json:"hash"`
		Top         int    `json:"top"`
		MissingText bool   `json:"missingText"`
	}
	if err := evaluate(`(()=>{const h=document.querySelector('#letter-studio-title');const r=h.getBoundingClientRect();return {hash:location.hash,top:Math.round(r.top),missingText:document.querySelector('#letter-editor').innerText.includes('Choose an active role-specific resume for this job.')}})()`, &letterState); err != nil {
		t.Fatal(err)
	}
	if letterState.Hash != "#letter-editor" || letterState.Top < 0 || letterState.Top >= 700 || !letterState.MissingText {
		t.Fatalf("cover-letter action did not reveal the editor and its source prerequisite: %+v", letterState)
	}
	captureWorkspace("generation-1100x800-dark", 1100, 800, "#letter-editor", "dark")
	captureWorkspace("selection-900x700-light", 900, 700, "#letter-editor", "light")
	click(`#application-answers-workspace > summary`)
	wait(`document.querySelector('#application-answers-workspace')?.open===true && document.querySelector('#letter-editor')?.open===false && document.querySelector('[data-question-panel]:not([hidden]) textarea')?.value==='Unsaved answer stays in this editor.'`)
	var switchedState struct {
		Key   string `json:"key"`
		Dirty bool   `json:"dirty"`
		Help  bool   `json:"help"`
	}
	if err := evaluate(`(()=>{const q=document.querySelector('[data-question-panel]:not([hidden])'),f=q?.querySelector('form'),h=q?.querySelector('[data-question-help]');return {key:q?.dataset.questionKey,dirty:f?.dataset.dirty==='true',help:!!h&&h.open}})()`, &switchedState); err != nil {
		t.Fatal(err)
	}
	if switchedState.Key == "" || !switchedState.Dirty {
		t.Fatalf("switching activities lost the selected dirty answer: %+v", switchedState)
	}
	wait(`location.hash==='#application-answers-workspace'`)
	captureWorkspace("answers-return-900x700-light", 900, 700, "#application-materials", "light")
	var navigation struct {
		Entries []struct {
			ID  int64  `json:"id"`
			URL string `json:"url"`
		} `json:"entries"`
	}
	if err := connection.Call(ctx, "Page.getNavigationHistory", map[string]any{}, &navigation); err != nil {
		t.Fatal(err)
	}
	findHistoryEntry := func(suffix string) int64 {
		t.Helper()
		for _, entry := range navigation.Entries {
			if strings.HasSuffix(entry.URL, suffix) {
				return entry.ID
			}
		}
		t.Fatalf("history entry %q not found in %+v", suffix, navigation)
		return 0
	}
	letterEntry := findHistoryEntry("#letter-editor")
	if err := connection.Call(ctx, "Page.navigateToHistoryEntry", map[string]any{"entryId": letterEntry}, nil); err != nil {
		t.Fatal(err)
	}
	wait(`location.hash==='#letter-editor' && document.querySelector('#letter-editor').open && !document.querySelector('#application-answers-workspace').open && document.querySelector('[data-question-panel]:not([hidden]) textarea').value==='Unsaved answer stays in this editor.'`)
	if err := connection.Call(ctx, "Page.getNavigationHistory", map[string]any{}, &navigation); err != nil {
		t.Fatal(err)
	}
	answerEntry := findHistoryEntry("#application-answers-workspace")
	if err := connection.Call(ctx, "Page.navigateToHistoryEntry", map[string]any{"entryId": answerEntry}, nil); err != nil {
		t.Fatal(err)
	}
	wait(`location.hash==='#application-answers-workspace' && document.querySelector('#application-answers-workspace').open && !document.querySelector('#letter-editor').open && document.querySelector('[data-question-panel]:not([hidden]) textarea').value==='Unsaved answer stays in this editor.'`)
	if err := evaluate(`(()=>{const e=document.querySelector('[data-question-panel]:not([hidden]) textarea');e.value='';e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := connection.Call(ctx, "Page.reload", map[string]any{"ignoreCache": true}, nil); err != nil {
		t.Fatal(err)
	}
	wait(`document.readyState==='complete' && location.hash==='#application-answers-workspace' && document.querySelector('#application-answers-workspace').open && !document.querySelector('#letter-editor').open`)
	noScripts.Store(true)
	noJSPage := server.URL + "/jobs/1?qualification=nojs"
	noJSChrome, err := chromium.LaunchFixture(ctx, noJSPage+"#application-materials")
	if err != nil {
		t.Fatal(err)
	}
	defer noJSChrome.Close()
	noJSConnection, err := noJSChrome.FixturePage(ctx, noJSPage)
	if err != nil {
		t.Fatal(err)
	}
	defer noJSConnection.Close()
	evaluateNoJS := func(script string, out any) error {
		var result struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		if err := noJSConnection.Call(ctx, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &result); err != nil {
			return err
		}
		if result.Exception != nil {
			return fmt.Errorf("script-free browser assertion failed: %v", result.Exception)
		}
		if out != nil {
			return json.Unmarshal(result.Result.Value, out)
		}
		return nil
	}
	waitNoJS := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			var ok bool
			if evaluateNoJS("Boolean("+script+")", &ok) == nil && ok {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("script-free browser timed out waiting for %s", script)
	}
	waitNoJS(`document.readyState==='complete' && !!document.querySelector('[data-writing-activity]')`)
	if noScriptRequests.Load() < 2 {
		t.Fatalf("expected the UI scripts to be withheld, received %d script requests", noScriptRequests.Load())
	}
	assertNoJS := func(script string) {
		t.Helper()
		var ok bool
		if err := evaluateNoJS(script, &ok); err != nil || !ok {
			t.Fatalf("script-free writing activity assertion failed: %s (%v)", script, err)
		}
	}
	assertNoJS(`document.querySelector('#application-answers-workspace').open && !document.querySelector('#letter-editor').open`)
	noJSShot, err := noJSChrome.FixtureScreenshotViewport(ctx, noJSPage, 900, 700, "#application-materials", "light")
	if err != nil {
		t.Fatal(err)
	}
	noJSImage, err := base64.StdEncoding.DecodeString(noJSShot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "no-js-900x700-light.png"), noJSImage, 0600); err != nil {
		t.Fatal(err)
	}
	if err := evaluateNoJS(`document.querySelector('#letter-editor > summary').click()`, nil); err != nil {
		t.Fatal(err)
	}
	assertNoJS(`document.querySelector('#letter-editor').open && !document.querySelector('#application-answers-workspace').open`)
	if err := evaluateNoJS(`document.querySelector('#application-answers-workspace > summary').click()`, nil); err != nil {
		t.Fatal(err)
	}
	assertNoJS(`document.querySelector('#application-answers-workspace').open && !document.querySelector('#letter-editor').open`)
}
