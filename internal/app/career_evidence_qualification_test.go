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
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// This opt-in qualification uses only a temporary SQLite database and a
// localhost page. It does not open employer sites or send evidence externally.
func TestControlledCareerEvidenceUI(t *testing.T) {
	if testing.Short() || !strings.EqualFold(strings.TrimSpace(os.Getenv("YOURMOVE_QUALIFY_BROWSER")), "1") {
		t.Skip("opt-in isolated Chromium Career Evidence qualification")
	}
	a, _ := testWorkspace(t)
	appHandler := newMux(a)
	var noAppScript atomic.Bool
	var scriptRequests atomic.Int32
	router := http.NewServeMux()
	router.HandleFunc("GET /career-evidence/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/career-evidence", http.StatusSeeOther)
	})
	router.HandleFunc("GET /static/ui.js", func(w http.ResponseWriter, r *http.Request) {
		scriptRequests.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/javascript")
		if noAppScript.Load() {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		appHandler.ServeHTTP(w, r)
	})
	router.Handle("/", appHandler)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, server.URL+"/career-evidence")
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	page := server.URL + "/career-evidence"
	evaluate := func(script string, out any) error {
		var result struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{
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
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			var ready bool
			if evaluate(script, &ready) == nil && ready {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("Career Evidence browser condition timed out:", script)
	}
	run := func(script string) {
		t.Helper()
		if err := evaluate(script, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait(`document.readyState==='complete' && !!document.querySelector('form[action="/career-evidence"]')`)
	run(`document.querySelector('form[action="/career-evidence"] button[type="submit"]').click()`)
	page = server.URL + "/career-evidence/1?notice=created"
	wait(`document.readyState==='complete' && location.pathname==='/career-evidence/1' && !!document.querySelector('#career-title')`)
	run(`(()=>{const values={"career-title":"Synthetic database incident","career-situation":"A customer-facing service slowed down.\nDatabase operations took longer than expected.","career-actions":"Reviewed synthetic slow-query logs and compared EXPLAIN plans."};for(const [id,value] of Object.entries(values)){const field=document.getElementById(id);field.value=value;field.dispatchEvent(new Event('input',{bubbles:true}))}document.querySelector('.career-evidence-form button[type="submit"]').click()})()`)
	page = server.URL + "/career-evidence/1?notice=saved"
	wait(`document.readyState==='complete' && location.search.includes('notice=saved')`)
	evidence, err := careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "draft" || evidence.Revision != 2 || evidence.Situation != "A customer-facing service slowed down.\nDatabase operations took longer than expected." {
		t.Fatalf("browser draft save: %+v %v", evidence, err)
	}
	run(`(()=>{document.querySelector('[name="confirm_review"]').checked=true;document.querySelector('.career-evidence-form button[formaction$="/review"]').click()})()`)
	page = server.URL + "/career-evidence/1?notice=reviewed"
	wait(`document.readyState==='complete' && location.search.includes('notice=reviewed')`)
	evidence, err = careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "reviewed" || evidence.Revision != 3 {
		t.Fatalf("browser applicant review: %+v %v", evidence, err)
	}

	noAppScript.Store(true)
	if err := chrome.FixtureCall(ctx, page, "Page.navigate", map[string]any{"url": server.URL + "/career-evidence/1"}, nil); err != nil {
		t.Fatal(err)
	}
	page = server.URL + "/career-evidence/1"
	wait(`document.readyState==='complete' && !!document.querySelector('#career-title')`)
	if scriptRequests.Load() < 2 {
		t.Fatalf("Career Evidence page did not request its optional application script: %d", scriptRequests.Load())
	}
	run(`document.getElementById('career-title').focus()`)
	if err := chrome.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Tab", "code": "Tab", "windowsVirtualKeyCode": 9}, nil); err != nil {
		t.Fatal(err)
	}
	var focusID string
	if err := evaluate(`document.activeElement.id`, &focusID); err != nil || focusID != "career-situation" {
		t.Fatalf("keyboard tab order from title: %q %v", focusID, err)
	}
	run(`(()=>{const field=document.getElementById('career-situation');field.value+='\nA no-script edit.';document.querySelector('.career-evidence-form button[type="submit"]').click()})()`)
	page = server.URL + "/career-evidence/1?notice=saved"
	wait(`document.readyState==='complete' && location.search.includes('notice=saved')`)
	evidence, err = careerEvidenceByID(a.db, 1)
	if err != nil || evidence.Status != "draft" || evidence.Revision != 4 || !strings.Contains(evidence.Situation, "A no-script edit.") {
		t.Fatalf("native form submit without app JavaScript: %+v %v", evidence, err)
	}
	for _, width := range []int{1280, 390} {
		shot, err := chrome.FixtureScreenshotAt(ctx, page, width, ".career-evidence-editor", "")
		if err != nil || shot == "" {
			t.Fatalf("%dpx Career Evidence layout: screenshot=%t err=%v", width, shot != "", err)
		}
		if dir := os.Getenv("YOURMOVE_QUALIFY_SCREENSHOT_DIR"); dir != "" {
			image, err := base64.StdEncoding.DecodeString(shot)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("career-evidence-%d.png", width)), image, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Log("Created a draft, marked it reviewed, and saved a no-application-JavaScript edit with keyboard focus in an isolated Chromium fixture.")
}
