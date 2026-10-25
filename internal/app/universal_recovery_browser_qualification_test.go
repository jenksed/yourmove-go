//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// This exercises the actual local UI with synthetic content. CDP input is
// controlled Chromium evidence, not native Wails or human usability acceptance.
func TestUniversalRecoveryBrowserJourney(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_RECOVERY") != "1" {
		t.Skip("opt-in controlled recovery journey")
	}
	dir := os.Getenv("YOURMOVE_RECOVERY_EVIDENCE_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute YOURMOVE_RECOVERY_EVIDENCE_DIR required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a, _ := testWorkspace(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	router := http.NewServeMux()
	router.HandleFunc("GET /jobs/{id}/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/"+r.PathValue("id"), http.StatusSeeOther)
	})
	router.Handle("/", newMux(a))
	server := httptest.NewUnstartedServer(router)
	server.Listener = listener
	server.Start()
	defer server.Close()
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.PostForm(server.URL+"/jobs", url.Values{"job_url": {"https://careers.example.com/jobs/recovery-engineer"}, "save_first": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("save status %d", resp.StatusCode)
	}
	var id int64
	if err := a.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, "https://careers.example.com/jobs/recovery-engineer").Scan(&id); err != nil {
		t.Fatal(err)
	}
	pageURL := server.URL + "/jobs/" + strconv.FormatInt(id, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	waitM1Session(t, ctx, chrome, pageURL, `document.readyState==='complete' && !!document.querySelector('#description')`)
	page, err := chrome.FixturePage(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()
	if err := page.Call(ctx, "Runtime.enable", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && document.documentElement.classList.contains('tabs-enabled')`)
	if !evalM1Browser[bool](t, ctx, page, `!document.getElementById('overview').hidden && document.getElementById('edit-details').open`) {
		t.Fatal("recovery not immediately visible")
	}
	capture := func(state string) {
		t.Helper()
		// Freeze visual transitions only for evidence capture. An immediate theme
		// change otherwise records intermediate button colors, not the final UI.
		evalM1Browser[bool](t, ctx, page, `(()=>{if(!document.getElementById('qualification-stable-frame')){const style=document.createElement('style');style.id='qualification-stable-frame';style.textContent='*,*::before,*::after{transition:none!important;animation:none!important;scroll-behavior:auto!important}';document.head.append(style)}return true})()`)
		for _, width := range []int{1280, 700, 390} {
			if err := page.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": 900, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
				t.Fatal(err)
			}
			if !evalM1Browser[bool](t, ctx, page, `document.documentElement.scrollWidth<=innerWidth`) {
				t.Fatalf("%s horizontal overflow at %d", state, width)
			}
			for _, theme := range []string{"light", "dark"} {
				evalM1Browser[bool](t, ctx, page, `(()=>{const s=document.querySelector('[data-theme-choice]');s.value=`+strconv.Quote(theme)+`;s.dispatchEvent(new Event('change'));scrollTo(0,0);return true})()`)
				if err := page.Call(ctx, "Runtime.evaluate", map[string]any{"expression": `new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))`, "awaitPromise": true, "returnByValue": true}, nil); err != nil {
					t.Fatal(err)
				}
				var shot struct {
					Data string `json:"data"`
				}
				if err := page.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &shot); err != nil {
					t.Fatal(err)
				}
				png, err := base64.StdEncoding.DecodeString(shot.Data)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, state+"-"+strconv.Itoa(width)+"-"+theme+".png"), png, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	capture("saved-empty")
	// CDP keyboard operation checks the native tab order in Chromium only.
	evalM1Browser[bool](t, ctx, page, `(()=>{document.getElementById('description').focus();return true})()`)
	if err := page.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Tab", "code": "Tab", "windowsVirtualKeyCode": 9}, nil); err != nil {
		t.Fatal(err)
	}
	if err := page.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "Tab", "code": "Tab", "windowsVirtualKeyCode": 9}, nil); err != nil {
		t.Fatal(err)
	}
	if !evalM1Browser[bool](t, ctx, page, `document.activeElement.id==='title'`) {
		t.Fatal("description to title keyboard order failed")
	}
	const description = "Responsibilities\nBuild dependable services.\n\nQualifications\nExperience with Go and SQLite.\n• Communicate clearly."
	// Three deliberate synthetic actions after arriving: focus, paste, save.
	evalM1Browser[bool](t, ctx, page, `(()=>{document.getElementById('description').focus();return document.activeElement.id==='description'})()`)
	if err := page.Call(ctx, "Input.insertText", map[string]any{"text": description}, nil); err != nil {
		t.Fatal(err)
	}
	evalM1Browser[bool](t, ctx, page, `(()=>{document.querySelector('.job-edit-form button[type="submit"]').click();return true})()`)
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && !!document.querySelector('[data-job-document]')`)
	job, err := a.jobByID(id)
	if err != nil || strings.ReplaceAll(job.Description, "\r\n", "\n") != description || !job.DescriptionManual {
		t.Fatalf("paste not persisted: %+v %v", job, err)
	}
	// Review remains reachable explicitly, and the content is never presented as API evidence.
	evalM1Browser[bool](t, ctx, page, `(()=>{document.querySelector('[data-tab-target="overview"]').click();return true})()`)
	if !strings.Contains(evalM1Browser[string](t, ctx, page, `document.getElementById('overview').innerText`), "Build dependable services.") {
		t.Fatal("saved description not readable")
	}
	capture("recovered-review")
	evalM1Browser[bool](t, ctx, page, `(()=>{document.querySelector('[data-tab-target="prepare"]').click();return true})()`)
	if !evalM1Browser[bool](t, ctx, page, `!document.getElementById('prepare').hidden && !!document.getElementById('resume_id') && !!document.querySelector('#prepare a[href="/answer-library"]')`) {
		t.Fatal("Prepare/resume/reuse continuation missing")
	}
	capture("recovered-prepare")
	// Reload without application JavaScript and correct essential details through
	// the same ordinary server form. All activities must remain available.
	if err := page.Call(ctx, "Emulation.setScriptExecutionDisabled", map[string]any{"value": true}, nil); err != nil {
		t.Fatal(err)
	}
	if err := page.Call(ctx, "Page.navigate", map[string]any{"url": pageURL + "?recovery_nojs=1#overview"}, nil); err != nil {
		t.Fatal(err)
	}
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && !document.documentElement.classList.contains('tabs-enabled')`)
	if !evalM1Browser[bool](t, ctx, page, `!document.getElementById('prepare').hidden && !document.getElementById('overview').hidden`) {
		t.Fatal("no-JavaScript activities unavailable")
	}
	evalM1Browser[bool](t, ctx, page, `(()=>{document.getElementById('edit-details').open=true;document.getElementById('title').value='Recovery Engineer';document.getElementById('company').value='Synthetic Employer';document.querySelector('.job-edit-form button[type="submit"]').click();return true})()`)
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && document.querySelector('h1')?.textContent==='Recovery Engineer'`)
	job, err = a.jobByID(id)
	if err != nil || job.Title != "Recovery Engineer" || job.Company != "Synthetic Employer" || strings.ReplaceAll(job.Description, "\r\n", "\n") != description {
		t.Fatalf("no-JS correction lost content: %+v %v", job, err)
	}
	if job.SubmissionCount != 0 {
		t.Fatal("recovery recorded submission")
	}
	t.Log("PASS controlled Chromium: save unknown URL, immediate paste editor, three synthetic recovery actions, durable manual description, readable review, Prepare/resume/reuse path, 1280/700/390 light/dark no overflow; no employer interactions. Native NOT VERIFIED.")
}
