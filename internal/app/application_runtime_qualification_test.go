//go:build qualification && !desktop

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Uses the ordinary HTTP executable, a dynamically allocated loopback port, and
// an explicitly isolated data directory. Never stops an existing runtime.
func TestControlledRuntimeRestart(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in isolated runtime qualification")
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot allocate isolated loopback port; existing processes left untouched: %v", err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	probe.Close()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	a, dir := testWorkspace(t)
	requireQualificationStorage(t, a)
	for i := 1; i <= 2; i++ {
		if err = a.saveJob(Job{URL: fmt.Sprintf("https://jobs.ashbyhq.com/fixture/%d", i), Host: "jobs.ashbyhq.com", Title: fmt.Sprintf("Runtime fixture %d", i), Stage: StageReady}); err != nil {
			t.Fatal(err)
		}
	}
	// Use a canonical supported destination for the active job.
	if _, err = a.db.Exec("UPDATE jobs SET url=? WHERE id=1", testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("restart-resume.pdf", strings.NewReader("%PDF-1.4\nsynthetic runtime fixture"))
	if err != nil {
		t.Fatal(err)
	}
	for i := int64(1); i <= 2; i++ {
		if err = a.assignResume(i, resume.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err = a.saveCandidateProfile(CandidateProfile{FirstName: "Runtime", LastName: "Fixture", Email: "runtime@example.test", Pronouns: "They/them", Approved: true}); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:why", Label: "Why this team?", Kind: "textarea"}
	if _, err = a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	if rec := postApp(newMux(a), "/jobs/1/answers/save", url.Values{"destination": {testAshbyPosting}, "question_key": {questionKey(testAshbyPosting, "ashby", field)}, "revision": {"0"}, "answer": {"Saved runtime answer"}}, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if err = a.recordSubmission(2, "Runtime confirmation", reviewedSubmissionToken(t, a, 2)); err != nil {
		t.Fatal(err)
	}
	a.db.Close()
	bin := filepath.Join(t.TempDir(), "yourmove-runtime")
	build := exec.Command("go", "build", "-o", bin, "../..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	client := &http.Client{Timeout: time.Second}
	start := func() func() {
		t.Helper()
		cmd := exec.Command(bin)
		cmd.Dir = dir
		cmd.Env = make([]string, 0, len(os.Environ())+1)
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "YOURMOVE_DATA_DIR=") && !strings.HasPrefix(entry, "YOURMOVE_HTTP_PORT=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "YOURMOVE_DATA_DIR="+dir)
		cmd.Env = append(cmd.Env, "YOURMOVE_HTTP_PORT="+fmt.Sprint(port))
		var logs bytes.Buffer
		cmd.Stdout = &logs
		cmd.Stderr = &logs
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		stopped := false
		stop := func() {
			if !stopped {
				stopped = true
				_ = cmd.Process.Signal(os.Interrupt)
				_ = cmd.Wait()
			}
		}
		t.Cleanup(stop)
		for deadline := time.Now().Add(10 * time.Second); ; {
			response, e := client.Get(baseURL + "/jobs/1")
			if e == nil {
				body, _ := io.ReadAll(response.Body)
				response.Body.Close()
				if response.StatusCode == 200 && strings.Contains(string(body), "Runtime fixture 1") {
					break
				}
			}
			if time.Now().After(deadline) {
				stop()
				t.Fatalf("runtime did not start: %s", logs.String())
			}
			time.Sleep(50 * time.Millisecond)
		}
		return stop
	}
	stop := start()
	stop()
	stop = start()
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	for deadline := time.Now().Add(8 * time.Second); ; {
		err = chrome.FixtureCall(ctx, baseURL+"/jobs", "Target.createTarget", map[string]any{"url": baseURL + "/jobs/2"}, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	page := baseURL + "/jobs/2"
	eval := func(script string) bool {
		var reply struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		return chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &reply) == nil && reply.Result.Value
	}
	wait := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(8 * time.Second); ; {
			if eval(script) {
				return
			}
			if time.Now().After(deadline) {
				var diagnostic struct {
					Result struct {
						Value string `json:"value"`
					} `json:"result"`
				}
				_ = chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": `JSON.stringify({url:location.href,title:document.title,text:document.body.innerText.slice(0,1600),prepareHidden:document.querySelector('#prepare')?.hidden,continueText:document.querySelector('form[action$="/continue"]')?.innerText,savedSession:document.querySelector('.saved-session-context')?.innerText})`, "returnByValue": true}, &diagnostic)
				t.Fatalf("rendered runtime check failed %s; state=%s", script, diagnostic.Result.Value)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	wait(`document.readyState==='complete'&&!document.querySelector('#history').hidden&&document.querySelector('#history').textContent.includes('Runtime confirmation')&&document.querySelector('#history').textContent.includes('restart-resume.pdf')&&!!document.querySelector('#history form[action="/jobs/2/next"]')`)
	_ = eval(`document.querySelector('#history form[action="/jobs/2/next"] button').click();true`)
	page = baseURL + "/jobs/1"
	wait(`!document.querySelector('#prepare').hidden&&document.querySelector('form[action$="/continue"]').textContent.includes('Saved runtime answer')&&document.querySelector('form[action$="/continue"]').textContent.includes('runtime@example.test')&&document.querySelector('[name=include_resume]')?.value==='yes'&&document.querySelector('#letter_resume_id').value===document.querySelector('#resume_id').value&&document.querySelector('#letter_resume_id').selectedOptions[0].text===document.querySelector('#resume_id').selectedOptions[0].text&&document.querySelector('.browser-reconnect button').textContent.includes('Open browser without filling')`)
	// Reproduce a real local blocker without touching a live application.
	path := filepath.Join(dir, "resumes", resume.StoredName)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("changed fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = eval(`location.reload();true`)
	wait(`document.body.textContent.includes('The selected resume needs attention.')&&!document.querySelector('form[action$="/continue"]')`)
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	_ = eval(`location.reload();true`)
	wait(`!!document.querySelector('form[action$="/continue"]')&&document.querySelector('form[action$="/continue"]').textContent.includes('Saved runtime answer')`)
	evidence, err := os.MkdirTemp("/private/tmp", "yourmove-checkpoint1-runtime-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("isolated runtime screenshots: %s", evidence)
	for _, width := range []int{1100, 390} {
		shot, e := chrome.FixtureScreenshotAt(ctx, page, width, "#application-action", "light")
		if e != nil {
			t.Fatal(e)
		}
		data, e := base64.StdEncoding.DecodeString(shot)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(evidence, fmt.Sprintf("runtime-restarted-%d.png", width)), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	response, err := client.Get(baseURL + "/candidate")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !strings.Contains(string(body), "They/them") {
		t.Fatal("profile preference did not survive restart")
	}
	t.Log("Ordinary executable stopped and restarted with isolated YOURMOVE_DATA_DIR; approved profile/pronouns, jobs, resume selection/file, saved answer and submission/document history survived. Rendered history → Next job → Fill observed. Changed resume suppressed Fill; restoring exact file recovered it without losing saved answers. No employer browser or writes.")
}
