//go:build qualification

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appbrowser "github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestControlledApplicationBriefClipboardAndWorkspace(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BRIEF") != "1" {
		t.Skip("opt-in synthetic application brief Chromium qualification")
	}
	a, _ := testWorkspace(t)
	job := makeAshbyBriefFixture(t, a, "Synthetic saved description for the clipboard brief.\nFull description end marker.", "$90,000–$120,000")
	liveFields := []schema.Field{
		{ID: "browser:why-team", Label: "Why are you interested in joining our team?", Kind: "textarea", Required: true},
		{ID: "browser:product", Label: "What interests you about this product?", Kind: "textarea"},
		{ID: "browser:ff5bb721-1230-4753-bb72-2f8324c0bf26__systemfield_name", Label: "Full Name", Kind: "text", Required: true},
		{ID: "browser:race", Label: "Race and ethnicity", Kind: "checkbox_group", Sensitive: true},
		{ID: "browser:attest", Label: "I agree to the terms", Kind: "checkbox", Required: true},
		{ID: "browser:custom", Label: "Choose a location on the map", Kind: "custom_widget"},
	}
	liveID, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL + "/application", Fields: liveFields})
	if err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("Synthetic TAM.docx", bytes.NewReader(sampleDOCX(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(job.ID, resume.ID); err != nil {
		t.Fatal(err)
	}
	letterBody := "A saved synthetic cover letter, available for review and transfer only after explicit authorization."
	if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, job.ID, letterBody, "fixture", "none", resume.ID, "synthetic-master", resume.ID, resume.SHA256, "synthetic-description", "2026-10-22T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	fake := &fakeBrowser{url: job.URL + "/application"}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fake, nil }
	if err := a.browsers.start(context.Background(), job.ID, job.URL); err != nil {
		t.Fatal(err)
	}
	a.browsers.setState(job.ID, "detected", "Synthetic current browser observation.", liveID, false)
	var previewCalls, inspectCalls, answerCalls, letterCalls int
	a.previewApplication = func(context.Context, string) (schema.Result, error) { previewCalls++; return schema.Result{}, nil }
	a.inspectApplication = func(context.Context, string) (schema.Result, error) { inspectCalls++; return schema.Result{}, nil }
	a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
		answerCalls++
		return "Synthetic provider output", nil
	}
	a.coverLetterGenerate = func(context.Context, string) (string, error) { letterCalls++; return "Synthetic provider output", nil }
	base := newMux(a)
	router := http.NewServeMux()
	router.HandleFunc("GET /jobs/1/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/1#prepare", http.StatusSeeOther)
	})
	router.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base.ServeHTTP(w, r)
	}))
	server := httptest.NewServer(router)
	defer server.Close()
	defer a.browsers.closeAll()
	page := server.URL + "/jobs/1"
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	waitBriefPage(t, ctx, chrome, page, "#prepare")
	currentPage := page
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('[data-copy-export][data-export-format="brief"]')?.hidden===false`)
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('#application-readiness-title')?.textContent==='Application readiness'`)
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('[data-brief-view]')?.href.endsWith('scope=unanswered')`)
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('#application-action form[action$="/continue"] button[type="submit"]')?.textContent.trim()==='Fill application'`)
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('#letter-editor') && document.querySelector('#letter-editor').open===false`)

	evidenceDir := filepath.Join("..", "..", "docs", "implementation", "checkpoint3-visuals", "after")
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	capture := func(name string, width int, selector, theme string) {
		t.Helper()
		encoded, err := chrome.FixtureScreenshotAt(ctx, currentPage, width, selector, theme)
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(evidenceDir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Log("screenshot", path)
	}
	capture("prepare-ready-1280-light.png", 1280, "#prepare", "light")
	capture("prepare-ready-1440-dark.png", 1440, "#prepare", "dark")
	capture("prepare-ready-390-light.png", 390, "#prepare", "light")
	capture("prepare-ready-390-dark.png", 390, "#prepare", "dark")
	clickBriefUI(t, ctx, chrome, page, `a[data-open-details="letter-editor"]`)
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('#letter-editor').open===true`)
	if err := evalBriefJS(ctx, chrome, page, `document.querySelector('#letter-editor').open=false;true`, nil); err != nil {
		t.Fatal(err)
	}

	var expected string
	response, err := http.Get(server.URL + "/jobs/1/brief?format=brief&scope=unanswered")
	if err != nil {
		t.Fatal(err)
	}
	expectedBytes, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("fetch exact synthetic export: status=%d read=%v close=%v", response.StatusCode, readErr, closeErr)
	}
	expected = string(expectedBytes)
	if err := evalBriefJS(ctx, chrome, page, `Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.syntheticClipboard=text}}});document.execCommand=()=>false;true`, nil); err != nil {
		t.Fatal(err)
	}
	clickBriefUI(t, ctx, chrome, page, `[data-copy-export][data-export-format="brief"]`)
	waitBriefJS(t, ctx, chrome, page, `document.querySelector('[data-copy-export-status]').textContent==='Application brief copied.'`)
	var copied string
	if err := evalBriefJS(ctx, chrome, page, `window.syntheticClipboard`, &copied); err != nil {
		t.Fatal(err)
	}
	if copied != expected || !strings.Contains(copied, "Current browser-observed form") || strings.Contains(copied, "PRIVATE_EMAIL") || strings.Contains(copied, "Full Name") || strings.Contains(copied, "Race and ethnicity") || strings.Contains(copied, "PRIVATE_SAVED_ANSWER_TEXT") {
		t.Fatal("clipboard text did not match the authoritative, privacy-filtered server export")
	}

	if err := evalBriefJS(ctx, chrome, page, `Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async()=>{throw new Error('denied')}}});document.execCommand=()=>false;true`, nil); err != nil {
		t.Fatal(err)
	}
	clickBriefUI(t, ctx, chrome, page, `[data-copy-export][data-export-format="brief"]`)
	waitBriefJS(t, ctx, chrome, page, `document.querySelector('[data-copy-export-status]').textContent==='Clipboard unavailable. Select and copy the text below.'`)
	assertBriefJS(t, ctx, chrome, page, `!document.querySelector('[data-copy-export-fallback]').hidden && document.querySelector('[data-copy-export-text]').value.includes('JOB APPLICATION BRIEF') && document.querySelector('[data-copy-export-text]').selectionEnd===document.querySelector('[data-copy-export-text]').value.length`)
	capture("prepare-copy-fallback-390-light.png", 390, "[data-copy-export-fallback]", "light")

	if err := evalBriefJS(ctx, chrome, page, `document.querySelector('[data-brief-scope]').value='all';document.querySelector('[data-brief-scope]').dispatchEvent(new Event('change',{bubbles:true}));Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.syntheticClipboard=text}}});true`, nil); err != nil {
		t.Fatal(err)
	}
	assertBriefJS(t, ctx, chrome, page, `document.querySelector('[data-brief-view]')?.href.endsWith('scope=all')`)
	clickBriefUI(t, ctx, chrome, page, `[data-copy-export][data-export-format="questions"]`)
	waitBriefJS(t, ctx, chrome, page, `document.querySelector('[data-copy-export-status]').textContent==='Questions copied.'`)
	if err := evalBriefJS(ctx, chrome, page, `window.syntheticClipboard`, &copied); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copied, "ACTIONABLE APPLICATION QUESTIONS (WORDING ONLY)") || !strings.Contains(copied, "Why are you interested in joining our team?") || strings.Contains(copied, "Synthetic saved description") || strings.Contains(copied, "PRIVATE_SAVED_ANSWER_TEXT") {
		t.Fatalf("Copy questions used the wrong scope/content: %s", copied)
	}
	clickBriefUI(t, ctx, chrome, page, `[data-copy-export][data-export-format="details"]`)
	waitBriefJS(t, ctx, chrome, page, `document.querySelector('[data-copy-export-status]').textContent==='Job details copied.'`)
	if err := evalBriefJS(ctx, chrome, page, `window.syntheticClipboard`, &copied); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copied, "Synthetic saved description") || strings.Contains(copied, "QUESTIONS NEEDING MY INPUT") {
		t.Fatalf("Copy job details used the wrong format: %s", copied)
	}

	// Show an uncertain transfer result without treating the local fill as an
	// employer submission or replaying a transfer.
	a.browsers.mu.Lock()
	state := a.browsers.states[job.ID]
	state.AnswerStatus = "complete"
	state.AnswerOutcomes = []appbrowser.AnswerOutcome{{Label: "Why are you interested in joining our team?", Status: "uncertain", Detail: "Synthetic manual review required."}}
	a.browsers.states[job.ID] = state
	a.browsers.mu.Unlock()
	currentPage = page + "?visual=manual"
	if err := chrome.FixtureCall(ctx, page, "Page.navigate", map[string]any{"url": currentPage + "#prepare"}, nil); err != nil {
		t.Fatal(err)
	}
	waitBriefJS(t, ctx, chrome, currentPage, `document.body.innerText.includes('Uncertain transfer') && document.body.innerText.includes('Synthetic manual review required.')`)
	capture("prepare-manual-review-1280-dark.png", 1280, "#application-action", "dark")
	if fake.observes != 0 || previewCalls != 0 || inspectCalls != 0 || answerCalls != 0 || letterCalls != 0 {
		t.Fatalf("copy caused employer or provider activity: browser=%d preview=%d inspect=%d answer=%d letter=%d", fake.observes, previewCalls, inspectCalls, answerCalls, letterCalls)
	}
	var submissions int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissions); err != nil || submissions != 0 {
		t.Fatalf("copy or transfer review recorded a submission: count=%d err=%v", submissions, err)
	}

	// Capture an actionable missing-resume state in the same isolated job.
	if err := a.browsers.close(job.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(job.ID, 0); err != nil {
		t.Fatal(err)
	}
	previousPage := currentPage
	currentPage = page + "?visual=missing"
	if err := chrome.FixtureCall(ctx, previousPage, "Page.navigate", map[string]any{"url": currentPage + "#prepare"}, nil); err != nil {
		t.Fatal(err)
	}
	waitBriefJS(t, ctx, chrome, currentPage, `document.body.innerText.includes('Choose or repair a resume') && !!document.querySelector('#prepare-resume-heading')`)
	missingPage := currentPage
	encoded, err := chrome.FixtureScreenshotAt(ctx, missingPage, 1280, "#prepare", "light")
	if err != nil {
		t.Fatal(err)
	}
	missingBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "prepare-missing-resume-1280-light.png"), missingBytes, 0600); err != nil {
		t.Fatal(err)
	}

	// This is a synthetic local history record. No employer page was submitted.
	if err := a.recordSubmission(job.ID, "Synthetic visual qualification only", reviewedSubmissionToken(t, a, job.ID)); err != nil {
		t.Fatal(err)
	}
	previousPage = currentPage
	currentPage = page + "?visual=history"
	if err := chrome.FixtureCall(ctx, previousPage, "Page.navigate", map[string]any{"url": currentPage + "#history"}, nil); err != nil {
		t.Fatal(err)
	}
	waitBriefJS(t, ctx, chrome, currentPage, `document.body.innerText.includes('Application recorded as Applied.')`)
	encoded, err = chrome.FixtureScreenshotAt(ctx, currentPage, 1280, "#history", "dark")
	if err != nil {
		t.Fatal(err)
	}
	historyBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "history-recorded-1280-dark.png"), historyBytes, 0600); err != nil {
		t.Fatal(err)
	}

	// Run the no-JavaScript browser path with script execution disabled in
	// Chromium. The plain-text GET form must still render selectable text.
	noJSRouter := http.NewServeMux()
	noJSRouter.HandleFunc("GET /jobs/1/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/1#prepare", http.StatusSeeOther)
	})
	noJSRouter.HandleFunc("GET /static/ui.js", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "text/javascript") })
	noJSRouter.HandleFunc("GET /static/inspection.js", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "text/javascript") })
	noJSRouter.Handle("/", newMux(a))
	noJSServer := httptest.NewServer(noJSRouter)
	defer noJSServer.Close()
	noJSPage := noJSServer.URL + "/jobs/1"
	noJSChrome, err := chromium.LaunchFixture(ctx, noJSPage)
	if err != nil {
		t.Fatal(err)
	}
	defer noJSChrome.Close()
	var cdp *chromium.FixturePage
	for deadline := time.Now().Add(10 * time.Second); ; {
		cdp, err = noJSChrome.FixturePage(ctx, noJSPage)
		if err != nil {
			cdp, err = noJSChrome.FixturePage(ctx, noJSPage+"/application")
		}
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer cdp.Close()
	if err := cdp.Call(ctx, "Emulation.setScriptExecutionDisabled", map[string]any{"value": true}, nil); err != nil {
		t.Fatal(err)
	}
	if err := cdp.Call(ctx, "Page.navigate", map[string]any{"url": noJSPage + "#prepare"}, nil); err != nil {
		t.Fatal(err)
	}
	waitNoJSText(t, ctx, cdp, "data-copy-export-fallback")
	if err := domClickBrief(ctx, cdp, "details.brief-export-options > summary"); err != nil {
		t.Fatal(err)
	}
	if err := domClickBrief(ctx, cdp, `button[name="format"][value="brief"]`); err != nil {
		t.Fatal(err)
	}
	plainText := waitNoJSBriefResponse(t, ctx, cdp)
	if !strings.Contains(plainText, "JOB APPLICATION BRIEF") || !strings.Contains(plainText, "Synthetic Northstar") || !strings.Contains(plainText, "Application questions have not been discovered yet.") {
		t.Fatalf("no-JavaScript plain-text view was incomplete: %s", plainText)
	}
}

func waitBriefPage(t *testing.T, ctx context.Context, chrome *chromium.Session, page, selector string) {
	t.Helper()
	encoded, _ := json.Marshal(selector)
	for deadline := time.Now().Add(12 * time.Second); ; {
		var result struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": "document.readyState==='complete' && !!document.querySelector(" + string(encoded) + ")", "returnByValue": true}, &result)
		if err == nil && result.Result.Value {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("page did not settle at %s selector %s: %v", page, selector, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func evalBriefJS(ctx context.Context, chrome *chromium.Session, page, expression string, out any) error {
	var result struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &result); err != nil {
		return err
	}
	if len(result.Exception) > 0 && string(result.Exception) != "null" {
		return fmt.Errorf("JavaScript exception: %s", result.Exception)
	}
	if out != nil {
		return json.Unmarshal(result.Result.Value, out)
	}
	return nil
}

func assertBriefJS(t *testing.T, ctx context.Context, chrome *chromium.Session, page, expression string) {
	t.Helper()
	var ok bool
	if err := evalBriefJS(ctx, chrome, page, expression, &ok); err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("browser assertion failed: %s", expression)
	}
}

func waitBriefJS(t *testing.T, ctx context.Context, chrome *chromium.Session, page, expression string) {
	t.Helper()
	for deadline := time.Now().Add(8 * time.Second); ; {
		var ok bool
		if evalBriefJS(ctx, chrome, page, expression, &ok) == nil && ok {
			return
		}
		if time.Now().After(deadline) {
			var body string
			_ = evalBriefJS(ctx, chrome, page, `document.body.innerText`, &body)
			t.Fatalf("timed out waiting for browser state: %s\npage: %s\nbody: %s", expression, page, body)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func clickBriefUI(t *testing.T, ctx context.Context, chrome *chromium.Session, page, selector string) {
	t.Helper()
	encoded, _ := json.Marshal(selector)
	var point struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := evalBriefJS(ctx, chrome, page, `(()=>{const e=document.querySelector(`+string(encoded)+`);if(!e||e.hidden||!e.getClientRects().length)throw Error('control is not visible');e.scrollIntoView({block:'center',behavior:'instant'});const r=e.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()`, &point); err != nil {
		t.Fatal(err)
	}
	for _, eventType := range []string{"mousePressed", "mouseReleased"} {
		if err := chrome.FixtureCall(ctx, page, "Input.dispatchMouseEvent", map[string]any{"type": eventType, "x": point.X, "y": point.Y, "button": "left", "clickCount": 1}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func waitNoJSText(t *testing.T, ctx context.Context, cdp *chromium.FixturePage, needle string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; {
		body, err := noJSBodyHTML(ctx, cdp)
		if err == nil && strings.Contains(body, needle) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no-JavaScript page did not render %q: err=%v body=%s", needle, err, body)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func noJSBodyHTML(ctx context.Context, cdp *chromium.FixturePage) (string, error) {
	var document struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := cdp.Call(ctx, "DOM.getDocument", map[string]any{"depth": -1}, &document); err != nil {
		return "", err
	}
	var body struct {
		NodeID int `json:"nodeId"`
	}
	if err := cdp.Call(ctx, "DOM.querySelector", map[string]any{"nodeId": document.Root.NodeID, "selector": "body"}, &body); err != nil {
		return "", err
	}
	var result struct {
		OuterHTML string `json:"outerHTML"`
	}
	if err := cdp.Call(ctx, "DOM.getOuterHTML", map[string]any{"nodeId": body.NodeID}, &result); err != nil {
		return "", err
	}
	return result.OuterHTML, nil
}

func domClickBrief(ctx context.Context, cdp *chromium.FixturePage, selector string) error {
	var document struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := cdp.Call(ctx, "DOM.getDocument", map[string]any{"depth": -1}, &document); err != nil {
		return err
	}
	var node struct {
		NodeID int `json:"nodeId"`
	}
	if err := cdp.Call(ctx, "DOM.querySelector", map[string]any{"nodeId": document.Root.NodeID, "selector": selector}, &node); err != nil || node.NodeID == 0 {
		return fmt.Errorf("find no-JavaScript control %q: %v", selector, err)
	}
	if err := cdp.Call(ctx, "DOM.scrollIntoViewIfNeeded", map[string]any{"nodeId": node.NodeID}, nil); err != nil {
		return err
	}
	var box struct {
		Model struct {
			Content []float64 `json:"content"`
		} `json:"model"`
	}
	if err := cdp.Call(ctx, "DOM.getBoxModel", map[string]any{"nodeId": node.NodeID}, &box); err != nil {
		return err
	}
	if len(box.Model.Content) < 8 {
		return fmt.Errorf("no box for no-JavaScript control %q", selector)
	}
	x := (box.Model.Content[0] + box.Model.Content[2] + box.Model.Content[4] + box.Model.Content[6]) / 4
	y := (box.Model.Content[1] + box.Model.Content[3] + box.Model.Content[5] + box.Model.Content[7]) / 4
	for _, eventType := range []string{"mousePressed", "mouseReleased"} {
		if err := cdp.Call(ctx, "Input.dispatchMouseEvent", map[string]any{"type": eventType, "x": x, "y": y, "button": "left", "clickCount": 1}, nil); err != nil {
			return err
		}
	}
	return nil
}

func waitNoJSBriefResponse(t *testing.T, ctx context.Context, cdp *chromium.FixturePage) string {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; {
		var tree struct {
			FrameTree struct {
				Frame struct {
					URL string `json:"url"`
				} `json:"frame"`
			} `json:"frameTree"`
		}
		if err := cdp.Call(ctx, "Page.getFrameTree", nil, &tree); err == nil && strings.Contains(tree.FrameTree.Frame.URL, "/brief?") {
			body, bodyErr := noJSBodyHTML(ctx, cdp)
			if bodyErr == nil {
				return body
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no-JavaScript export form did not load plain text")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
