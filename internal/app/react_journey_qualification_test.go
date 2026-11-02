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
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/browser/playwright"
	"github.com/jenksed/yourmove-go/internal/application/schema"
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
	if err := a.saveJob(Job{URL: "https://jobs.ashbyhq.com/fixture/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Host: "jobs.ashbyhq.com", Title: "Next synthetic opportunity"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", LinkedIn: "https://example.test/casey", Approved: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{{ID: "browser:linkedin", Label: "LinkedIn", Kind: "text"}}}); err != nil {
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
<label>Why this team?<textarea name="motivation"></textarea></label><label>Tell us more<textarea name="other"></textarea></label><label>LinkedIn<input name="linkedin"></label><label>Manual field<input name="manual" value="Already entered"></label><label>Phone<input type="tel" name="phone" value="Manual phone"></label><button type="submit">Submit</button></form>
<script>window.resumeChanges=0;window.coverChanges=0;window.submits=0;window.contactChanges={first_name:0,last_name:0,email:0};document.querySelector('form').addEventListener('input',e=>{if(e.target.name in contactChanges)contactChanges[e.target.name]++});document.querySelector('[name=resume]').onchange=()=>{resumeChanges++;document.querySelector('form').insertAdjacentHTML('beforeend','<label>First name<input name="first_name"></label>');setTimeout(()=>{document.querySelector('form').insertAdjacentHTML('beforeend','<label>Last name<input name="last_name"></label><label>Email<input type="email" name="email"></label>')},1200)};document.querySelector('[name=cover]').onchange=()=>coverChanges++;const existing=new DataTransfer();existing.items.add(new File(['existing employer file'],'existing.pdf',{type:'application/pdf'}));document.querySelector('[name=cover]').files=existing.files;document.querySelector('[name=cover]').dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	base := fixture.URL + "/fixture/job"
	var previews atomic.Int32
	a.previewApplication = func(c context.Context, raw string) (schema.Result, error) {
		previews.Add(1)
		return chromium.PreviewFixture(c, raw, base)
	}
	var err error
	var employer *chromium.Session
	var firefox *playwright.Session
	var launches atomic.Int32
	a.browsers.factory = func(c context.Context, _ string) (readOnlySession, error) {
		launches.Add(1)
		if os.Getenv("YOURMOVE_QUAL_ENGINE") == "firefox" {
			var launchErr error
			firefox, launchErr = playwright.LaunchFixture(c, playwright.EngineFirefox, base)
			if launchErr != nil {
				return nil, launchErr
			}
			return &reactFirefoxFixture{Session: firefox, fixture: base, saved: testAshbyPosting}, nil
		}
		var launchErr error
		employer, launchErr = chromium.LaunchFixture(c, base)
		if launchErr != nil {
			return nil, launchErr
		}
		return &contactFixtureSession{Session: employer, fixture: base, saved: testAshbyPosting}, nil
	}
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
	employerPage := base + "/application"
	employerEval := func(expression string, out any) error {
		if firefox != nil {
			return firefox.FixtureEvaluate(ctx, expression, out)
		}
		return eval(employer, employerPage, expression, out)
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
	discoveryStarted := time.Now()
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
	click("Questions")
	wait(`Array.from(document.querySelectorAll('.writing-question')).some(x=>x.textContent.includes('Why this team?'))`)
	t.Logf("Early Ashby question appeared without an applicant browser after %s", time.Since(discoveryStarted).Round(time.Millisecond))
	if launches.Load() != 0 || previews.Load() != 1 {
		t.Fatalf("unexpected browser launches: managed=%d preview=%d", launches.Load(), previews.Load())
	}
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Why this team?')).click()`)
	run(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,'Prepared before opening');e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally. It has not been filled')`)

	click("Apply")
	wait(`document.body.innerText.includes('journey-resume.pdf')&&document.body.innerText.includes('Cover letter:')`)
	if launches.Load() != 0 {
		t.Fatal("employer browser launched before applicant action")
	}
	click("View Application")
	wait(`document.body.innerText.includes('Return to Application')`)
	if launches.Load() != 1 {
		t.Fatalf("view launched %d browsers", launches.Load())
	}
	var employerReady bool
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if err := employerEval(`!!document.querySelector('[name=resume]') && !!document.querySelector('[name=cover]')`, &employerReady); err == nil && employerReady {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !employerReady {
		t.Fatal("employer fixture did not load")
	}
	var beforeView []any
	if err := employerEval(`[document.querySelector('[name=resume]').files.length,document.querySelector('[name=linkedin]').value,document.querySelector('[name=cover]').files[0]?.name,submits]`, &beforeView); err != nil || fmt.Sprint(beforeView) != "[0  existing.pdf 0]" {
		t.Fatalf("view transferred material: %v %v", beforeView, err)
	}
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Return to Application' && !x.disabled)`)
	click("Return to Application")
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Return to Application' && !x.disabled) && !document.querySelector('#employer .job-form-error')`)
	click("Questions")
	wait(`Array.from(document.querySelectorAll('.writing-question')).some(x=>x.textContent.includes('Why this team?'))`)
	jobForMatch, _ := a.jobByID(1)
	var matched bool
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
		prepared, prepErr := a.prepareEarlyQuestions(jobForMatch, nil)
		if prepErr != nil {
			t.Fatal(prepErr)
		}
		for _, q := range prepared.Questions {
			if q.Field.Label == "Why this team?" && q.Value == "Prepared before opening" && q.Reconciliation.CanAccept {
				matched = true
			}
		}
		if matched {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !matched {
		t.Fatal("early answer was not preserved and matched to the live control")
	}
	t.Log("Early answer remained saved and matched the live control without transferring it")
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Why this team?')).click()`)
	run(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,'Unsaved synthetic draft');e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	wait(`document.querySelector('#writing-answer')?.value==='Unsaved synthetic draft'`)
	discoveredAt := time.Now()
	if err := employerEval(`(()=>{history.pushState({},'',location.pathname+'/step2');document.querySelector('form').insertAdjacentHTML('beforeend','<label>Delayed question<textarea name="delayed"></textarea></label>');return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	employerPage = base + "/application/step2"
	wait(`Array.from(document.querySelectorAll('.writing-question')).some(x=>x.textContent.includes('Delayed question'))`)
	latency := time.Since(discoveredAt)
	if latency > 6*time.Second {
		t.Fatalf("question discovery took %s", latency)
	}
	wait(`document.querySelector('#writing-answer')?.value==='Unsaved synthetic draft'`)
	t.Logf("Delayed question on subsequent SPA step visible while editing after %s; unsaved draft preserved", latency.Round(time.Millisecond))
	click("Discard edits")
	click("Apply")
	wait(`Array.from(document.querySelectorAll('.continuation-item label')).find(x=>x.textContent.includes('Cover letter:'))?.querySelector('input')?.checked`)
	wait(`document.querySelector('.pending-answer-scope')?.innerText.includes('https://example.test/casey')`)
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Prepare & Fill Application' && !x.disabled)`)
	click("Prepare & Fill Application")
	wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Résumé')`)
	if launches.Load() != 1 {
		t.Fatalf("fill replaced the existing browser: %d launches", launches.Load())
	}
	if profile, err := a.candidateProfile(); err != nil || profile.Approved {
		t.Fatalf("fill changed separate profile approval: %+v %v", profile, err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		s := a.browsers.state(1)
		if s.ResumeStatus == "selected" && s.LetterStatus == "review" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var contactValues []string
	for time.Now().Before(deadline) {
		if err := employerEval(`[document.querySelector('[name=first_name]')?.value||'',document.querySelector('[name=last_name]')?.value||'',document.querySelector('[name=email]')?.value||'',document.querySelector('[name=phone]')?.value||'']`, &contactValues); err == nil && fmt.Sprint(contactValues) == "[Casey Fixture casey@example.test Manual phone]" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if fmt.Sprint(contactValues) != "[Casey Fixture casey@example.test Manual phone]" {
		t.Fatalf("delayed contacts: %v; state: %+v", contactValues, a.browsers.state(1))
	}
	t.Logf("one-click delayed contact values: %v; missing profile phone preserved manually", contactValues)
	var linkedIn string
	for time.Now().Before(deadline) {
		if err := employerEval(`document.querySelector('[name=linkedin]').value`, &linkedIn); err == nil && linkedIn == "https://example.test/casey" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if linkedIn != "https://example.test/casey" {
		t.Fatalf("profile LinkedIn not filled: %q; state=%+v", linkedIn, a.browsers.state(1))
	}

	var contactChanges map[string]int
	if err := employerEval(`contactChanges`, &contactChanges); err != nil || contactChanges["first_name"] != 1 || contactChanges["last_name"] != 1 || contactChanges["email"] != 1 {
		t.Fatalf("contact writes repeated: %v %v", contactChanges, err)
	}

	var existing string
	if err := employerEval(`document.querySelector('[name=cover]').files[0]?.name`, &existing); err != nil || existing != "existing.pdf" {
		t.Fatalf("existing attachment changed: %q %v", existing, err)
	}
	wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Cover letter')`)
	// The applicant explicitly clears the existing file in the employer form,
	// then authorizes the selected saved letter again.
	if err := employerEval(`(()=>{const e=document.querySelector('[name=cover]');e.value='';e.dispatchEvent(new Event('change',{bubbles:true}))})()`, nil); err != nil {
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
	err = employerEval(`(async()=>({letterHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=cover]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),resumeHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=resume]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),resumeName:document.querySelector('[name=resume]').files[0]?.name||'',resumeSize:document.querySelector('[name=resume]').files[0]?.size||0,letterSize:document.querySelector('[name=cover]').files[0]?.size||0,resumeChanges,coverChanges,submits,manual:document.querySelector('[name=manual]').value}))()`, &observed)
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
	run(`Array.from(document.querySelectorAll('#employer summary')).find(x=>x.textContent.includes('Browser troubleshooting')).click()`)
	click("Recheck form")
	time.Sleep(500 * time.Millisecond)
	var counts []int
	if err := employerEval(`[resumeChanges,coverChanges,submits]`, &counts); err != nil || fmt.Sprint(counts) != "[1 3 0]" {
		t.Fatalf("recheck changed employer form: %v %v", counts, err)
	}
	click("Questions")
	wait(`Array.from(document.querySelectorAll('.writing-question')).some(x=>x.textContent.includes('Delayed question'))`)
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Delayed question')).click()`)
	const answer = "I enjoy helping customers solve challenging support problems."
	run(fmt.Sprintf(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}))})()`, answer))
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally.')`)
	click("Apply")
	wait(`document.querySelector('.continuation-item')?.parentElement?.innerText.includes('saved answer')`)
	wait(`Array.from(document.querySelectorAll('.continuation-item label')).find(x=>x.textContent.includes('saved answer'))?.querySelector('input')?.checked`)
	run(`Array.from(document.querySelectorAll('#employer button')).filter(x=>x.classList.contains('button-primary')).at(-1).click()`)
	deadline = time.Now().Add(15 * time.Second)
	var employerValues []string
	for time.Now().Before(deadline) {
		err = employerEval(`[document.querySelector('[name=delayed]').value,document.querySelector('[name=other]').value,document.querySelector('[name=manual]').value]`, &employerValues)
		if err == nil && fmt.Sprint(employerValues) == fmt.Sprint([]string{answer, "", "Already entered"}) && a.browsers.state(1).AnswerStatus == "complete" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || fmt.Sprint(employerValues) != fmt.Sprint([]string{answer, "", "Already entered"}) {
		t.Fatalf("answers: %q %v", employerValues, err)
	}
	if a.browsers.state(1).AnswerStatus != "complete" {
		t.Fatalf("answer status: %+v", a.browsers.state(1))
	}
	t.Log("Observed question appeared without manual refresh; one saved answer transferred, another remained blank and the manual field was preserved.")
	if err := employerEval(`[resumeChanges,coverChanges,submits]`, &counts); err != nil || fmt.Sprint(counts) != "[1 3 0]" {
		t.Fatalf("unexpected final employer effects: %v %v", counts, err)
	}
	if state.ResumeStatus != "selected" || state.LetterStatus != "complete" || observed.ResumeSize != len(resumeBytes) || observed.LetterSize == 0 || observed.ResumeChanges != 1 || observed.CoverChanges != 3 || observed.Submits != 0 || observed.Manual != "Already entered" {
		t.Fatalf("transfer not demonstrated: %+v", observed)
	}
	if !strings.Contains(observed.ResumeName, "journey") {
		t.Fatalf("unexpected resume name %q", observed.ResumeName)
	}

	// A separate explicit Fill action must also open a closed session itself.
	run(`window.confirm=()=>true`)
	click("Close browser")
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='View Application' && !x.disabled)`)
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Prepare & Fill Application' && !x.disabled)`)
	employerPage = base + "/application"
	click("Prepare & Fill Application")
	wait(`document.body.innerText.includes('Return to Application')`)
	if launches.Load() != 2 {
		t.Fatalf("direct Fill did not launch one new browser: %d", launches.Load())
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && a.browsers.state(1).ResumeStatus != "selected" {
		time.Sleep(100 * time.Millisecond)
	}
	if a.browsers.state(1).ResumeStatus != "selected" {
		t.Fatalf("direct Fill did not attach resume: %+v", a.browsers.state(1))
	}
	t.Log("An explicit Fill click opened a fresh session and selected the résumé without a separate View action.")

	// Leaving an unsubmitted job requires an explicit decision before browser-only work is closed.
	wait(`document.querySelector('#submission')?.innerText.includes('Next synthetic opportunity')`)
	run(`window.confirm=()=>false`)
	click("Next opportunity → Next synthetic opportunity")
	wait(`Array.from(document.querySelectorAll('#submission button')).some(x=>x.textContent.includes('Next opportunity') && !x.disabled)`)
	if !a.browsers.state(1).Active {
		t.Fatal("cancelling navigation closed the browser")
	}
	run(`window.confirm=()=>true`)
	click("Next opportunity → Next synthetic opportunity")
	page = app.URL + "/jobs/2"
	wait(`document.body.innerText.includes('Next synthetic opportunity')`)
	job, err := a.jobByID(1)
	if err != nil || job.Stage == StageApplied || job.SubmissionCount != 0 || a.browsers.state(1).Active {
		t.Fatalf("unsubmitted navigation changed state: %+v %v", job, err)
	}
	t.Log("Cancelled navigation kept the browser; confirmed navigation closed it and preserved the unsubmitted job.")

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
	wait(`document.body.innerText.includes('Delayed question')`)
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Delayed question')).click()`)
	wait(`document.querySelector('#writing-answer')?.value === "I enjoy helping customers solve challenging support problems."`)
	click("Apply")
	wait(`document.body.innerText.includes('saved materials remain') || document.body.innerText.includes('disconnected')`)
	click("Applied")
	click("I submitted this application")
	wait(`document.body.innerText.includes('Submission recorded locally.')`)
	wait(`(async()=>{const history=await (await fetch('/api/jobs/1/submissions')).json();const job=await (await fetch('/api/jobs/1/workspace')).json();return history.count===1 && job.job.stage==='applied'})()`)
	click("Next opportunity → Next synthetic opportunity")
	page = baseURL + "/jobs/2"
	wait(`document.body.innerText.includes('Next synthetic opportunity')`)

	t.Log("Same executable restarted in the same isolated workspace; selected resume, manual letter and captured question remained in React, disconnected session disclosed; explicit local completion recorded.")
}

type reactFirefoxFixture struct {
	*playwright.Session
	fixture, saved string
}

func (s *reactFirefoxFixture) Observe(ctx context.Context) (browser.Observation, error) {
	o, err := s.Session.Observe(ctx)
	if err == nil {
		o.URL = strings.Replace(o.URL, s.fixture, s.saved, 1)
	}
	return o, err
}
