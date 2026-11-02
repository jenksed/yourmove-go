//go:build qualification

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// This uses the production Vite build and Go handlers, with only the employer
// origin mapped to a local controlled Ashby form by the existing fixture adapter.
func TestReactJourneyDocumentTransfer(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, dir := testWorkspace(t)
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if len(a.resources.frontendIndex) == 0 {
		t.Fatal("build frontend first")
	}
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic React journey", Description: "Synthetic support role"}); err != nil {
		t.Fatal(err)
	}
	resumeBytes := mustQualificationPDF(t)
	if _, err := a.importResume("journey-resume.pdf", bytes.NewReader(resumeBytes)); err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><form class="ashby-application-form">
<section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".pdf"></section>
<section data-field-entry-id="cover"><div class="ashby-application-form-question-title">Cover Letter</div><input type="file" name="cover" accept=".pdf"></section>
<label>Why this team?<textarea name="motivation"></textarea></label><label>Tell us more<textarea name="other"></textarea></label><label>Manual field<input name="manual" value="Already entered"></label><button type="submit">Submit</button></form>
<script>window.resumeChanges=0;window.coverChanges=0;window.submits=0;document.querySelector('[name=resume]').onchange=()=>resumeChanges++;document.querySelector('[name=cover]').onchange=()=>coverChanges++;document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	base := fixture.URL + "/fixture/job"
	employer, err := chromium.LaunchFixture(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer employer.Close()
	fs := &contactFixtureSession{Session: employer, fixture: base, saved: testAshbyPosting}
	a.browsers.pollEvery = 100 * time.Millisecond
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fs, nil }
	defer a.browsers.closeAll()
	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1", http.StatusSeeOther) })
	router.Handle("/", newMux(a))
	app := httptest.NewServer(router)
	defer app.Close()
	page := app.URL + "/jobs/1"
	ui, err := chromium.LaunchFixture(ctx, app.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Close()
	eval := func(session *chromium.Session, target, expression string, out any) error {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		if err := session.FixtureCall(ctx, target, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &reply); err != nil {
			return err
		}
		if len(reply.Exception) > 0 {
			return fmt.Errorf("JS %s", reply.Exception)
		}
		if out != nil {
			return json.Unmarshal(reply.Result.Value, out)
		}
		return nil
	}
	run := func(expression string) {
		t.Helper()
		if err := eval(ui, page, expression, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(expression string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			if eval(ui, page, expression, &ok) == nil && ok {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		var body string
		_ = eval(ui, page, `document.body.innerText`, &body)
		t.Fatalf("timeout %s\n%s", expression, body)
	}
	click := func(label string) {
		t.Helper()
		run(fmt.Sprintf(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()===%q && x.offsetParent!==null).click()`, label))
	}
	wait(`document.body.innerText.includes('Synthetic React journey')`)
	click("Documents")
	wait(`!!document.querySelector('input[name=workspace-resume]:not(:checked)')`)
	run(`Array.from(document.querySelectorAll('input[name=workspace-resume]')).find(x=>!x.checked).click()`)
	wait(`document.body.innerText.includes('Selection saved')`)
	const letter = "Dear Hiring Team, I am interested in this synthetic support role and would welcome the opportunity to contribute."
	run(fmt.Sprintf(`(()=>{const e=document.querySelector('#manual-letter');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}))})()`, letter))
	click("Save new letter")
	wait(`document.body.innerText.includes('Your letter was saved')`)
	// Choose an earlier draft and ensure Apply uses that exact selection.
	run(`Array.from(document.querySelectorAll('summary')).find(x=>x.textContent.includes('Write a letter')).click()`)
	run(`(()=>{const e=document.querySelector('#manual-letter');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,'This other synthetic letter must not be transferred to the employer.');e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	click("Save new letter")
	wait(`document.querySelector('#letter-draft')?.options.length===3`)
	run(`(()=>{const e=document.querySelector('#letter-draft');e.value='1';e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	wait(`document.querySelector('#letter-body')?.value.includes('synthetic support role')`)

	click("Apply")
	wait(`document.body.innerText.includes('journey-resume.pdf')&&document.body.innerText.includes('Cover letter:')`)
	// Starting opens the managed session without transferring anything.
	run(`Array.from(document.querySelectorAll('#employer button')).find(x=>x.classList.contains('button-primary')).click()`)
	wait(`document.body.innerText.includes('Recheck form')`)
	// A user-supplied file in the controlled form must not be replaced.
	if err := eval(employer, base+"/application", `(()=>{const e=document.querySelector('[name=cover]');const d=new DataTransfer();d.items.add(new File(['existing employer file'],'existing.pdf',{type:'application/pdf'}));e.files=d.files;e.dispatchEvent(new Event('change',{bubbles:true}))})()`, nil); err != nil {
		t.Fatal(err)
	}

	wait(`Array.from(document.querySelectorAll('.continuation-item label')).filter(x=>x.textContent.includes('Résumé:') || x.textContent.includes('Cover letter:')).every(x=>x.querySelector('input').checked)`)
	run(`Array.from(document.querySelectorAll('#employer button')).filter(x=>x.classList.contains('button-primary')).at(-1).click()`)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		s := a.browsers.state(1)
		if s.ResumeStatus == "selected" && s.LetterStatus == "review" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var existing string
	if err := eval(employer, base+"/application", `document.querySelector('[name=cover]').files[0]?.name`, &existing); err != nil || existing != "existing.pdf" {
		t.Fatalf("existing attachment changed: %q %v", existing, err)
	}
	wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Cover letter')`)
	// The applicant explicitly clears the existing file in the employer form,
	// then authorizes the selected saved letter again.
	if err := eval(employer, base+"/application", `(()=>{const e=document.querySelector('[name=cover]');e.value='';e.dispatchEvent(new Event('change',{bubbles:true}))})()`, nil); err != nil {
		t.Fatal(err)
	}
	wait(`Array.from(document.querySelectorAll('.continuation-item label')).find(x=>x.textContent.includes('Cover letter:'))?.querySelector('input')?.disabled===false`)
	run(`Array.from(document.querySelectorAll('.continuation-item label')).find(x=>x.textContent.includes('Cover letter:')).querySelector('input').click()`)
	run(`Array.from(document.querySelectorAll('#employer button')).filter(x=>x.classList.contains('button-primary')).at(-1).click()`)
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && a.browsers.state(1).LetterStatus != "complete" {
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("Existing employer cover attachment preserved; explicit manual removal and reauthorization allowed the saved letter to attach.")
	state := a.browsers.state(1)
	t.Logf("transfer state: resume=%s letter=%s", state.ResumeStatus, state.LetterStatus)
	var observed struct {
		ResumeName    string `json:"resumeName"`
		ResumeHash    string `json:"resumeHash"`
		LetterHash    string `json:"letterHash"`
		ResumeSize    int    `json:"resumeSize"`
		LetterSize    int    `json:"letterSize"`
		ResumeChanges int    `json:"resumeChanges"`
		CoverChanges  int    `json:"coverChanges"`
		Submits       int    `json:"submits"`
		Manual        string `json:"manual"`
	}
	err = eval(employer, base+"/application", `(async()=>({letterHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=cover]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),resumeHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=resume]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),resumeName:document.querySelector('[name=resume]').files[0]?.name||'',resumeSize:document.querySelector('[name=resume]').files[0]?.size||0,letterSize:document.querySelector('[name=cover]').files[0]?.size||0,resumeChanges,coverChanges,submits,manual:document.querySelector('[name=manual]').value}))()`, &observed)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("employer form: %+v", observed)
	letterPDF, err := coverLetterPDF(letter)
	if err != nil {
		t.Fatal(err)
	}
	letterSum := sha256.Sum256(letterPDF)
	if observed.LetterHash != hex.EncodeToString(letterSum[:]) {
		t.Fatal("employer letter differs from selected saved draft")
	}
	sum := sha256.Sum256(resumeBytes)
	if observed.ResumeHash != hex.EncodeToString(sum[:]) {
		t.Fatal("employer resume bytes differ")
	}
	wait(`document.querySelector(".continuation-outcomes")?.innerText.includes("Résumé") && document.querySelector(".continuation-outcomes")?.innerText.includes("Cover letter")`)
	click("Recheck form")
	time.Sleep(500 * time.Millisecond)
	var counts []int
	if err := eval(employer, base+"/application", `[resumeChanges,coverChanges,submits]`, &counts); err != nil || fmt.Sprint(counts) != "[1 3 0]" {
		t.Fatalf("recheck changed employer form: %v %v", counts, err)
	}
	click("Questions")
	wait(`Array.from(document.querySelectorAll('.writing-question')).some(x=>x.textContent.includes('Why this team?'))`)
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Why this team?')).click()`)
	const answer = "I enjoy helping customers solve challenging support problems."
	run(fmt.Sprintf(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}))})()`, answer))
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally.')`)
	click("Apply")
	wait(`document.querySelector('.continuation-item')?.parentElement?.innerText.includes('saved answer')`)
	wait(`Array.from(document.querySelectorAll('.continuation-item label')).find(x=>x.textContent.includes('saved answer'))?.querySelector('input')?.checked`)
	run(`Array.from(document.querySelectorAll('#employer button')).filter(x=>x.classList.contains('button-primary')).at(-1).click()`)
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && a.browsers.state(1).AnswerStatus != "complete" {
		time.Sleep(100 * time.Millisecond)
	}
	var employerValues []string
	if err := eval(employer, base+"/application", `[document.querySelector('[name=motivation]').value,document.querySelector('[name=other]').value,document.querySelector('[name=manual]').value]`, &employerValues); err != nil || fmt.Sprint(employerValues) != fmt.Sprint([]string{answer, "", "Already entered"}) {
		t.Fatalf("answers: %q %v", employerValues, err)
	}
	if a.browsers.state(1).AnswerStatus != "complete" {
		t.Fatalf("answer status: %+v", a.browsers.state(1))
	}
	t.Log("Observed question appeared without manual refresh; one saved answer transferred, another remained blank and the manual field was preserved.")
	if err := eval(employer, base+"/application", `[resumeChanges,coverChanges,submits]`, &counts); err != nil || fmt.Sprint(counts) != "[1 3 0]" {
		t.Fatalf("unexpected final employer effects: %v %v", counts, err)
	}
	if state.ResumeStatus != "selected" || state.LetterStatus != "complete" || observed.ResumeSize != len(resumeBytes) || observed.LetterSize == 0 || observed.ResumeChanges != 1 || observed.CoverChanges != 3 || observed.Submits != 0 || observed.Manual != "Already entered" {
		t.Fatalf("transfer not demonstrated: %+v", observed)
	}
	if !strings.Contains(observed.ResumeName, "journey") {
		t.Fatalf("unexpected resume name %q", observed.ResumeName)
	}

	// Stop the fixture host and its sole SQLite writer, then run the same
	// ordinary executable twice against that exact synthetic workspace.
	ui.Close()
	a.browsers.closeAll()
	app.Close()
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "yourmove")
	if output, err := exec.Command("go", "build", "-o", binary, "../..").CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v %s", err, output)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	start := func() func() {
		cmd := exec.Command(binary)
		cmd.Dir = dir
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "YOURMOVE_DATA_DIR=") && !strings.HasPrefix(v, "YOURMOVE_HTTP_PORT=") {
				cmd.Env = append(cmd.Env, v)
			}
		}
		cmd.Env = append(cmd.Env, "YOURMOVE_DATA_DIR="+dir, fmt.Sprintf("YOURMOVE_HTTP_PORT=%d", port))
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
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			response, e := http.Get(baseURL + "/jobs/1")
			if e == nil {
				response.Body.Close()
				if response.StatusCode == 200 {
					return stop
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		stop()
		t.Fatalf("runtime startup: %s", logs.String())
		return nil
	}
	stop := start()
	stop()
	stop = start()
	defer stop()
	runtimeUI, err := chromium.LaunchFixture(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeUI.Close()
	if err := runtimeUI.FixtureCall(ctx, baseURL+"/application", "Target.createTarget", map[string]any{"url": baseURL + "/jobs/1"}, nil); err != nil {
		t.Fatal(err)
	}
	page = baseURL + "/jobs/1"
	ui = runtimeUI
	wait(`document.body.innerText.includes('Synthetic React journey')`)
	click("Documents")
	wait(`document.body.innerText.includes('journey-resume.pdf') && document.querySelector('input[name=workspace-resume]:checked')?.parentElement.innerText.includes('journey-resume.pdf') && document.querySelector('#letter-body')?.value.includes('synthetic support role')`)
	click("Questions")
	wait(`document.body.innerText.includes('Why this team?')`)
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Why this team?')).click()`)
	wait(`document.querySelector('#writing-answer')?.value === "I enjoy helping customers solve challenging support problems."`)
	click("Apply")
	wait(`document.body.innerText.includes('saved materials remain') || document.body.innerText.includes('disconnected')`)
	click("Applied")
	run(`document.querySelector('.submission-disclosure summary').click()`)
	click("Review current selection for recording")
	wait(`document.body.innerText.includes('I confirm that I submitted')`)
	run(`Array.from(document.querySelectorAll('#submission label')).find(x=>x.textContent.includes('I confirm that I submitted')).querySelector('input').click()`)
	click("Record completed application")
	wait(`document.body.innerText.includes('Submission recorded locally.')`)
	t.Log("Same executable restarted in the same isolated workspace; selected resume, manual letter and captured question remained in React, disconnected session disclosed; explicit local completion recorded.")
}
