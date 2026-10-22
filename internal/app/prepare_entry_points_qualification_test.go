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
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: newMux(a)}}
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
}
