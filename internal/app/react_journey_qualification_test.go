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
	for _, occupied := range []bool{true, false} {
		t.Run(fmt.Sprintf("occupied=%t", occupied), func(t *testing.T) { qualifyReactJourneyDocumentTransfer(t, occupied) })
	}
}

func qualifyReactJourneyDocumentTransfer(t *testing.T, occupied bool) {
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
		html := `<!doctype html><form class="ashby-application-form">
<section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".pdf"></section>
<section data-field-entry-id="cover"><div class="ashby-application-form-question-title">Cover Letter</div><input type="file" name="cover" accept=".pdf"></section>
<label>Why this team?<textarea name="motivation"></textarea></label><label>Tell us more<textarea name="other"></textarea></label><label>Tell us about a challenging collaboration you navigated<textarea name="collaboration"></textarea></label><label>LinkedIn<input name="linkedin"></label><label>Manual field<input name="manual" value="Already entered"></label><label>Phone<input type="tel" name="phone" value="Manual phone"></label><button type="submit">Submit</button></form>
<script>window.answerChanges=0;document.querySelector("[name=motivation]").addEventListener("input",()=>answerChanges++);window.resumeChanges=0;window.coverChanges=0;window.submits=0;window.contactChanges={first_name:0,last_name:0,email:0};document.querySelector('form').addEventListener('input',e=>{if(e.target.name in contactChanges)contactChanges[e.target.name]++});document.querySelector('[name=resume]').onchange=()=>{resumeChanges++;const old=document.querySelector('[name=motivation]');old.parentElement.remove();setTimeout(()=>{document.querySelector('form').insertAdjacentHTML('beforeend','<label>Why this team?<textarea name="motivation"></textarea></label>');document.querySelector('[name=motivation]').addEventListener('input',()=>answerChanges++)},1800);document.querySelector('form').insertAdjacentHTML('beforeend','<label>First name<input name="first_name"></label>');setTimeout(()=>{document.querySelector('form').insertAdjacentHTML('beforeend','<label>Last name<input name="last_name"></label><label>Email<input type="email" name="email"></label>')},1200)};document.querySelector('[name=cover]').onchange=()=>coverChanges++;const existing=new DataTransfer();existing.items.add(new File(['existing employer file'],'existing.pdf',{type:'application/pdf'}));document.querySelector('[name=cover]').files=existing.files;document.querySelector('[name=cover]').dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('form').onsubmit=e=>{e.preventDefault();submits++}</script>`
		if !occupied {
			start := strings.Index(html, "const existing=new DataTransfer();")
			end := strings.Index(html, "document.querySelector('form').onsubmit")
			html = html[:start] + html[end:]
		}
		fmt.Fprint(w, html)
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
	a.browsers.factory = func(c context.Context, dest string) (readOnlySession, error) {
		launches.Add(1)
		if os.Getenv("YOURMOVE_QUAL_ENGINE") == "firefox" {
			var launchErr error
			firefox, launchErr = playwright.LaunchFixture(c, playwright.EngineFirefox, base)
			if launchErr != nil {
				return nil, launchErr
			}
			return &reactFirefoxFixture{Session: firefox, fixture: base, saved: dest}, nil
		}
		var launchErr error
		employer, launchErr = chromium.LaunchFixture(c, base)
		if launchErr != nil {
			return nil, launchErr
		}
		return &contactFixtureSession{Session: employer, fixture: base, saved: dest}, nil
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
	// selectQuestion retries the navigation-select handoff across a React
	// re-render: a select replaced between query and dispatch throws a
	// transient illegal-invocation instead of applying the choice.
	selectQuestion := func(fragment string) {
		t.Helper()
		expression := fmt.Sprintf(`(()=>{const e=document.querySelector('[aria-label="Question navigation"] select');e.value=Array.from(e.options).find(x=>x.textContent.includes(%q)).value;e.dispatchEvent(new Event('change',{bubbles:true}))})()`, fragment)
		var err error
		for i := 0; i < 5; i++ {
			if err = eval(ui, page, expression, nil); err == nil {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal(err)
	}
	// setTextarea retries the native editor write across a React re-render
	// and only returns once the visible editor actually holds the text: a
	// momentarily unmounted editor would otherwise throw or drop the draft.
	setTextarea := func(selector, fragment, text string) {
		t.Helper()
		expression := fmt.Sprintf(`(()=>{const e=document.querySelector(%q);Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}));return (document.querySelector(%q)?.value||'').includes(%q)})()`, selector, text, selector, fragment)
		for i := 0; i < 8; i++ {
			var ok bool
			if err := eval(ui, page, expression, &ok); err == nil && ok {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal("editor did not accept the prepared text")
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
	wait(`document.querySelector('#letter-history')?.options.length===3 && document.querySelector('#letter-history')?.value==='2'`)
	run(`(()=>{const e=document.querySelector('#letter-history');e.value='1';e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	wait(`document.querySelector('#letter-history')?.value==='1' && document.querySelector('#letter-body')?.value.includes('synthetic support role')`)
	click("Questions")
	wait(`Array.from(document.querySelectorAll('[aria-label="Question navigation"] select option')).some(x=>x.textContent.includes('Why this team?'))`)
	t.Logf("Early Ashby question appeared without an applicant browser after %s", time.Since(discoveryStarted).Round(time.Millisecond))
	if launches.Load() != 0 || previews.Load() != 1 {
		t.Fatalf("unexpected browser launches: managed=%d preview=%d", launches.Load(), previews.Load())
	}
	run(`(()=>{const e=document.querySelector('[aria-label="Question navigation"] select');e.value=Array.from(e.options).find(x=>x.textContent.includes('Why this team?')).value;e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	run(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,'Prepared before opening');e.dispatchEvent(new Event('input',{bubbles:true}))})()`)
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally. It has not been filled')`)

	click("Apply")
	wait(`document.body.innerText.includes('journey-resume.pdf')&&document.body.innerText.includes('Cover letter:')`)
	if launches.Load() != 0 {
		t.Fatal("employer browser launched before applicant action")
	}
	wait(`document.querySelector('.pending-answer-scope')?.innerText.includes('Prepared before opening')`)
	click("Fill Application")
	wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Résumé')`)
	if launches.Load() != 1 {
		t.Fatalf("original Fill launched %d browsers", launches.Load())
	}
	deadline := time.Now().Add(20 * time.Second)
	var earlyValues []any
	for time.Now().Before(deadline) {
		if err := employerEval(`[document.querySelector('[name=motivation]')?.value || '',answerChanges,document.querySelector('[name=manual]').value,submits]`, &earlyValues); err == nil && fmt.Sprint(earlyValues) == "[Prepared before opening 1 Already entered 0]" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if fmt.Sprint(earlyValues) != "[Prepared before opening 1 Already entered 0]" {
		t.Fatalf("one-click early answer: %v state=%+v", earlyValues, a.browsers.state(1))
	}
	t.Logf("one-click delayed early answer, writes, manual value, submissions: %v", earlyValues)
	if err := employerEval(`(()=>{history.pushState({},'',location.pathname+'/step2');document.querySelector('form').insertAdjacentHTML('beforeend','<label>Delayed question<textarea name="delayed"></textarea></label>');return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	employerPage = base + "/application/step2"
	if profile, err := a.candidateProfile(); err != nil || profile.Approved {
		t.Fatalf("fill changed separate profile approval: %+v %v", profile, err)
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		s := a.browsers.state(1)
		if s.ResumeStatus == "selected" && s.LetterStatus == map[bool]string{true: "review", false: "complete"}[occupied] {
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

	if occupied {
		var existing string
		if err := employerEval(`document.querySelector('[name=cover]').files[0]?.name`, &existing); err != nil || existing != "existing.pdf" {
			t.Fatalf("existing attachment changed: %q %v", existing, err)
		}
		wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Cover letter')`)
		// The applicant explicitly clears the existing file in the employer form,
		// then authorizes the selected saved letter again with one Fill.
		if err := employerEval(`(()=>{const e=document.querySelector('[name=cover]');e.value='';e.dispatchEvent(new Event('change',{bubbles:true}))})()`, nil); err != nil {
			t.Fatal(err)
		}
		wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Cover letter') && document.querySelector('.continuation-outcomes')?.innerText.includes('Review in the employer browser')`)
		wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
		click("Fill Application")
	}
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && a.browsers.state(1).LetterStatus != "complete" {
		time.Sleep(100 * time.Millisecond)
	}
	if occupied {
		t.Log("Existing employer cover attachment preserved; explicit manual removal and reauthorization allowed the saved letter to attach.")
	} else {
		t.Log("Clean form received all materials after one Fill.")
	}
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
	expectedCoverChanges := 1
	if occupied {
		expectedCoverChanges = 3
	}
	var counts []int
	if err := employerEval(`[resumeChanges,coverChanges,submits]`, &counts); err != nil || fmt.Sprint(counts) != fmt.Sprintf("[1 %d 0]", expectedCoverChanges) {
		t.Fatalf("recheck changed employer form: %v %v", counts, err)
	}
	click("Questions")
	wait(`Array.from(document.querySelectorAll('[aria-label="Question navigation"] select option')).some(x=>x.textContent.includes('Delayed question'))`)
	run(`(()=>{const e=document.querySelector('[aria-label="Question navigation"] select');e.value=Array.from(e.options).find(x=>x.textContent.includes('Delayed question')).value;e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	const answer = "I enjoy helping customers solve challenging support problems."
	run(fmt.Sprintf(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}))})()`, answer))
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally.')`)
	deadline = time.Now().Add(15 * time.Second)
answerReady:
	for {
		if payload, scopeErr := a.buildContinuation(1); scopeErr == nil {
			if pend, ok := payload["pendingAnswers"].([]continuationAnswerJSON); ok {
				for _, q := range pend {
					if q.Value == answer {
						break answerReady
					}
				}
			}
		}
		if !time.Now().Before(deadline) {
			t.Fatal("saved delayed answer not visible to continuation scope")
		}
		time.Sleep(100 * time.Millisecond)
	}
	click("Apply")
	wait(`document.querySelector('.continuation-item')?.parentElement?.innerText.includes('saved answer')`)
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
	click("Fill Application")
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
	var earlyWriteCount int
	if err := employerEval(`answerChanges`, &earlyWriteCount); err != nil || earlyWriteCount != 1 {
		t.Fatalf("early answer replayed after later actions: %d %v", earlyWriteCount, err)
	}
	t.Log("Observed question appeared without manual refresh; one saved answer transferred, another remained blank and the manual field was preserved.")
	if err := employerEval(`[resumeChanges,coverChanges,submits]`, &counts); err != nil || fmt.Sprint(counts) != fmt.Sprintf("[1 %d 0]", expectedCoverChanges) {
		t.Fatalf("unexpected final employer effects: %v %v", counts, err)
	}
	if state.ResumeStatus != "selected" || state.LetterStatus != "complete" || observed.ResumeSize != len(resumeBytes) || observed.LetterSize == 0 || observed.ResumeChanges != 1 || observed.CoverChanges != expectedCoverChanges || observed.Submits != 0 || observed.Manual != "Already entered" {
		t.Fatalf("transfer not demonstrated: %+v", observed)
	}
	if !strings.Contains(observed.ResumeName, "journey") {
		t.Fatalf("unexpected resume name %q", observed.ResumeName)
	}

	// Close, improve saved materials, and Fill again. The fresh session must
	// receive the updated answer and the updated selected letter.
	run(`window.confirm=()=>true`)
	click("Close browser")
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='View Application' && !x.disabled)`)
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
	click("Questions")
	wait(`Array.from(document.querySelectorAll('[aria-label="Question navigation"] select option')).some(x=>x.textContent.includes('Delayed question'))`)
	run(`(()=>{const e=document.querySelector('[aria-label="Question navigation"] select');e.value=Array.from(e.options).find(x=>x.textContent.includes('Delayed question')).value;e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	const updatedAnswer = "I enjoy helping customers solve challenging support problems with clear follow-up."
	run(fmt.Sprintf(`(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}))})()`, updatedAnswer))
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally.')`)
	click("Documents")
	wait(`document.querySelector('#letter-body')?.value.includes('synthetic support role')`)
	const updatedLetter = "Dear Hiring Team, I am interested in this updated synthetic support role and would welcome the opportunity to contribute with careful follow-up."
	run(fmt.Sprintf(`(()=>{const e=document.querySelector('#letter-body');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,%q);e.dispatchEvent(new Event('input',{bubbles:true}))})()`, updatedLetter))
	click("Save draft edits")
	wait(`document.body.innerText.includes('Draft saved exactly as edited.')`)
	// Gate the fresh Fill on the Go-owned scope, not on static text: the
	// Apply snapshot must carry the updated letter hash and answer.
	updatedPDF, err := coverLetterPDF(updatedLetter)
	if err != nil {
		t.Fatal(err)
	}
	updatedSum := sha256.Sum256(updatedPDF)
	updatedBodySum := sha256.Sum256([]byte(updatedLetter))
	updatedBodyHex := hex.EncodeToString(updatedBodySum[:])
	deadline = time.Now().Add(15 * time.Second)
	diagLetter, diagPending := "", ""
backendReady:
	for {
		payload, scopeErr := a.buildContinuation(1)
		if scopeErr == nil {
			if lj, ok := payload["letter"].(*continuationLetterJSON); ok && lj != nil {
				diagLetter = lj.ContentHash
				if lj.ContentHash == updatedBodyHex {
					if pend, ok := payload["pendingAnswers"].([]continuationAnswerJSON); ok {
						vals := make([]string, 0, len(pend))
						for _, q := range pend {
							vals = append(vals, q.Label+"="+q.Value)
						}
						diagPending = strings.Join(vals, "|")
						for _, q := range pend {
							if q.Value == updatedAnswer {
								break backendReady
							}
						}
					}
				}
			} else {
				diagLetter = fmt.Sprintf("unexpected letter type %T", payload["letter"])
			}
		} else {
			diagLetter = "scope error: " + scopeErr.Error()
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("saved edits not visible to continuation scope: letter=%s want=%.12s pending=[%s]", diagLetter, updatedBodyHex, diagPending)
		}
		time.Sleep(100 * time.Millisecond)
	}
	click("Apply")
	wait(`document.body.innerText.includes('journey-resume.pdf')`)
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
	employerPage = base + "/application"
	click("Fill Application")
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
	deadline = time.Now().Add(20 * time.Second)
	var refilled []string
	for time.Now().Before(deadline) {
		err = employerEval(`[document.querySelector('[name=motivation]')?.value||'',document.querySelector('[name=other]')?.value||'',document.querySelector('[name=manual]')?.value||'']`, &refilled)
		if err == nil && fmt.Sprint(refilled) == fmt.Sprint([]string{"Prepared before opening", "", "Already entered"}) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if fmt.Sprint(refilled) != fmt.Sprint([]string{"Prepared before opening", "", "Already entered"}) {
		t.Fatalf("fresh Fill did not reuse the saved early answer: %q", refilled)
	}
	// The updated delayed answer has no matching control in the fresh form
	// until the employer reveals it. Inject it as before and verify the
	// latest saved value fills, not the pre-edit one.
	if err := employerEval(`(()=>{history.pushState({},'',location.pathname+'/step2');document.querySelector('form').insertAdjacentHTML('beforeend','<label>Delayed question<textarea name="delayed"></textarea></label>');return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	employerPage = base + "/application/step2"
	deadline = time.Now().Add(20 * time.Second)
	refilled = nil
	for time.Now().Before(deadline) {
		err = employerEval(`[document.querySelector('[name=delayed]').value||'',document.querySelector('[name=other]').value||'',document.querySelector('[name=manual]').value||'']`, &refilled)
		if err == nil && fmt.Sprint(refilled) == fmt.Sprint([]string{updatedAnswer, "", "Already entered"}) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if fmt.Sprint(refilled) != fmt.Sprint([]string{updatedAnswer, "", "Already entered"}) {
		t.Fatalf("fresh Fill did not use the updated saved answer: %q", refilled)
	}
	// The fresh form starts with the fixture's pre-existing cover file, so a
	// safe Fill must preserve it rather than overwrite it with either the old
	// or the updated saved letter. The applicant then clears it manually and
	// fills once more; the updated saved letter must attach.
	if occupied {
		var preserved string
		if err := employerEval(`document.querySelector('[name=cover]').files[0]?.name`, &preserved); err != nil || preserved != "existing.pdf" {
			t.Fatalf("fresh Fill changed the existing attachment: %q %v", preserved, err)
		}
		if err := employerEval(`(()=>{const e=document.querySelector('[name=cover]');e.value='';e.dispatchEvent(new Event('change',{bubbles:true}))})()`, nil); err != nil {
			t.Fatal(err)
		}
		wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Cover letter') && document.querySelector('.continuation-outcomes')?.innerText.includes('Review in the employer browser')`)
		wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
		click("Fill Application")
	}
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && a.browsers.state(1).LetterStatus != "complete" {
		time.Sleep(100 * time.Millisecond)
	}
	var refilledLetter struct {
		LetterHash    string `json:"letterHash"`
		Submits       int    `json:"submits"`
		ResumeChanges int    `json:"resumeChanges"`
		CoverChanges  int    `json:"coverChanges"`
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		err = employerEval(`(async()=>({letterHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=cover]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),submits,resumeChanges,coverChanges}))()`, &refilledLetter)
		if err == nil && refilledLetter.LetterHash == hex.EncodeToString(updatedSum[:]) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if refilledLetter.LetterHash != hex.EncodeToString(updatedSum[:]) {
		t.Fatalf("fresh Fill did not use the updated saved letter: %+v", refilledLetter)
	}
	if refilledLetter.Submits != 0 || refilledLetter.ResumeChanges != 1 || refilledLetter.CoverChanges != expectedCoverChanges {
		t.Fatalf("fresh Fill submitted: %+v", refilledLetter)
	}
	t.Logf("Close, edit, re-Fill used updated answer and letter: answer=%q letterHash=%s", updatedAnswer, refilledLetter.LetterHash)
	t.Logf("Fresh session operations: %+v", refilledLetter)

	// Curate one reviewed narrative answer for the next application while
	// job A's reviewed work is still visible.
	click("Questions")
	wait(`Array.from(document.querySelectorAll('[aria-label="Question navigation"] select option')).some(x=>x.textContent.includes('challenging collaboration'))`)
	selectQuestion("challenging collaboration")
	const collabA = "I navigated a challenging collaboration by aligning support and engineering on a shared incident timeline for customers."
	setTextarea("#writing-answer", "shared incident timeline", collabA)
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally.')`)
	wait(`Array.from(document.querySelectorAll('button')).some(x=>x.textContent.trim()==='Save a copy to Answer Library' && x.offsetParent!==null)`)
	click("Save a copy to Answer Library")
	wait(`document.body.innerText.includes('Saved as a separate Answer Library example')`)
	t.Log("Job A curated a reusable narrative answer without leaving its workspace.")
	click("Apply")

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

	// Job B begins without a submission record for A, without job-A browser
	// state, and without another open application browser.
	if st := a.browsers.state(2); st.Active || st.ResumeStatus != "" || st.ContactStatus != "" || st.LetterStatus != "" || st.AnswerStatus != "" {
		t.Fatalf("job B starts with leaked browser state: %+v", st)
	}
	if other := a.browsers.otherApplication(2); other != 0 {
		t.Fatalf("job B sees another application browser: %d", other)
	}
	if opened, err := a.jobByID(2); err != nil || opened.Stage == StageApplied || opened.SubmissionCount != 0 {
		t.Fatalf("job B starts submitted: %+v %v", opened, err)
	}
	if launches.Load() != 2 {
		t.Fatalf("opening job B launched a browser: %d", launches.Load())
	}
	// Mirror job A's preparation baseline so job B's scope carries the same
	// profile-backed LinkedIn question from the start.
	job2, err := a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(2, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job2.URL + "/application", Fields: []schema.Field{{ID: "browser:linkedin", Label: "LinkedIn", Kind: "text"}}}); err != nil {
		t.Fatal(err)
	}

	// Prepare job B with the same résumé and job A's reviewed reusable
	// material, adapted and saved explicitly for B.
	click("Documents")
	wait(`!!document.querySelector('input[name=workspace-resume]:not(:checked)')`)
	run(`Array.from(document.querySelectorAll('input[name=workspace-resume]')).find(x=>!x.checked).click()`)
	wait(`document.body.innerText.includes('Selection saved')`)
	click("Questions")
	wait(`Array.from(document.querySelectorAll('[aria-label="Question navigation"] select option')).some(x=>x.textContent.includes('challenging collaboration'))`)
	selectQuestion("challenging collaboration")
	click("Browse all")
	wait(`document.body.innerText.includes('shared incident timeline')`)
	click("Preview full text")
	wait(`document.querySelector('.writing-preview')?.innerText.includes('shared incident timeline')`)
	click("Use as starting draft")
	wait(`document.body.innerText.includes('Starting from')`)
	const collabB = "I navigated a challenging collaboration at the second employer by aligning support and engineering on a shared incident timeline with weekly updates."
	setTextarea("#writing-answer", "weekly updates", collabB)
	click("Save answer locally")
	wait(`document.body.innerText.includes('Saved locally.')`)
	t.Log("Job B reused job A's reviewed answer through an explicit starting draft, adapted it, and saved it for B.")

	// Start job B's cover letter from job A's prior letter, edit and save it
	// as a new letter with its own identity, and leave it selected for B.
	click("Documents")
	wait(`document.body.innerText.includes('Use prior letter')`)
	click("Use prior letter")
	wait(`document.querySelector('.prior-letter-picker')?.innerText.includes('must not be transferred')`)
	run(`Array.from(document.querySelectorAll('.prior-letter-picker .writing-suggestions li')).find(li=>li.innerText.includes('synthetic support role'))?.querySelector('button')?.click()`)
	wait(`document.querySelector('.prior-letter-picker .writing-preview')?.innerText.includes('synthetic support role')`)
	click("Use as starting draft")
	wait(`!!document.querySelector('#prior-new-body')`)
	const letterB = "Dear Hiring Team, I am applying for this second synthetic support role with the same careful follow-up I brought to the first. I would welcome the opportunity to contribute."
	setTextarea("#prior-new-body", "second synthetic support role", letterB)
	click("Save new letter for this opportunity")
	wait(`document.body.innerText.includes('New letter saved for this opportunity with a new identity.')`)
	t.Log("Job B started its cover letter from a prior letter and saved it as a new letter for B.")

	// Job A's saved originals are unchanged and the reusable copy is intact.
	var priorLetterID int64
	if err := a.db.QueryRow(`SELECT id FROM cover_letters WHERE job_id=1 AND body LIKE '%synthetic support role%'`).Scan(&priorLetterID); err != nil || priorLetterID == 0 {
		t.Fatalf("job A original letter changed: %v", err)
	}
	var libraryEntryID int64
	if err := a.db.QueryRow(`SELECT id FROM answer_library WHERE answer=?`, collabA).Scan(&libraryEntryID); err != nil {
		t.Fatalf("reusable library answer changed: %v", err)
	}
	var priorLetterBody string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE id=?`, priorLetterID).Scan(&priorLetterBody); err != nil || priorLetterBody != updatedLetter {
		t.Fatalf("job A original letter changed: %q %v", priorLetterBody, err)
	}
	checkPresent := func(want string) {
		t.Helper()
		var n1, n2 int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=1 AND answer=?`, want).Scan(&n1)
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers WHERE job_id=1 AND answer=?`, want).Scan(&n2)
		if n1+n2 == 0 {
			t.Fatalf("job A saved answer missing: %q", want)
		}
	}
	checkPresent("Prepared before opening")
	checkPresent(updatedAnswer)
	checkPresent(collabA)
	for _, table := range []string{"application_answers", "early_application_answers", "cover_letters"} {
		column := "answer"
		if table == "cover_letters" {
			column = "body"
		}
		for _, unwanted := range []string{collabB, letterB} {
			var n int
			if err := a.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE job_id=1 AND `+column+`=?`, unwanted).Scan(&n); err != nil || n != 0 {
				t.Fatalf("job B material leaked into job A %s: %v", table, err)
			}
		}
	}
	var letter2Body string
	if err := a.db.QueryRow(`SELECT body FROM cover_letters WHERE job_id=1 AND id!=?`, priorLetterID).Scan(&letter2Body); err != nil || !strings.Contains(letter2Body, "must not be transferred") {
		t.Fatalf("job A unselected letter changed: %q %v", letter2Body, err)
	}
	var storedB, originB string
	for _, table := range []string{"application_answers", "early_application_answers"} {
		var answer, origin string
		if err := a.db.QueryRow(`SELECT answer,COALESCE(reuse_source_json,'') FROM `+table+` WHERE job_id=2 AND answer=?`, collabB).Scan(&answer, &origin); err == nil {
			storedB, originB = answer, origin
			break
		}
	}
	if storedB != collabB || !strings.Contains(originB, fmt.Sprint(libraryEntryID)) {
		t.Fatalf("job B adapted answer missing or unlinked: %q origin=%q", storedB, originB)
	}
	var storedLetterB, letterOriginB string
	if err := a.db.QueryRow(`SELECT body,COALESCE(reuse_source_json,'') FROM cover_letters WHERE job_id=2`).Scan(&storedLetterB, &letterOriginB); err != nil || storedLetterB != letterB {
		t.Fatalf("job B letter not saved: %q %v", storedLetterB, err)
	}
	if !strings.Contains(letterOriginB, fmt.Sprint(priorLetterID)) {
		t.Fatalf("job B letter lost its prior-letter origin: %q", letterOriginB)
	}
	if scopeB, scopeErr := a.buildContinuation(2); scopeErr != nil {
		t.Fatalf("job B scope: %v", scopeErr)
	} else if lj, ok := scopeB["letter"].(*continuationLetterJSON); !ok || lj == nil || lj.ContentHash != fmt.Sprintf("%x", sha256.Sum256([]byte(letterB))) {
		t.Fatalf("job B scope does not carry the new letter: %+v", scopeB["letter"])
	}

	// Fill job B once and verify the employer form receives B's selected
	// résumé, eligible answers, and selected letter — and nothing else.
	click("Apply")
	wait(`document.body.innerText.includes('journey-resume.pdf')`)
	wait(`document.querySelector('.pending-answer-scope')?.innerText.includes('weekly updates')`)
	wait(`document.body.innerText.includes('Selected saved letter')`)
	wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
	employerPage = base + "/application"
	click("Fill Application")
	wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Résumé')`)
	if launches.Load() != 3 {
		t.Fatalf("job B Fill launched %d browsers", launches.Load())
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		s := a.browsers.state(2)
		if s.ResumeStatus == "selected" && s.LetterStatus == map[bool]string{true: "review", false: "complete"}[occupied] {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if occupied {
		var existingB string
		if err := employerEval(`document.querySelector('[name=cover]').files[0]?.name`, &existingB); err != nil || existingB != "existing.pdf" {
			t.Fatalf("job B Fill overwrote the existing attachment: %q %v", existingB, err)
		}
		if err := employerEval(`(()=>{const e=document.querySelector('[name=cover]');e.value='';e.dispatchEvent(new Event('change',{bubbles:true}))})()`, nil); err != nil {
			t.Fatal(err)
		}
		wait(`document.querySelector('.continuation-outcomes')?.innerText.includes('Cover letter') && document.querySelector('.continuation-outcomes')?.innerText.includes('Review in the employer browser')`)
		wait(`Array.from(document.querySelectorAll('#employer button')).some(x=>x.textContent.trim()==='Fill Application' && !x.disabled)`)
		click("Fill Application")
	}
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && a.browsers.state(2).LetterStatus != "complete" {
		time.Sleep(100 * time.Millisecond)
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var contactB []string
		if err := employerEval(`[document.querySelector('[name=first_name]')?.value||'',document.querySelector('[name=last_name]')?.value||'',document.querySelector('[name=email]')?.value||'',document.querySelector('[name=linkedin]').value]`, &contactB); err == nil && fmt.Sprint(contactB) == "[Casey Fixture casey@example.test https://example.test/casey]" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	var observedB struct {
		Motivation string `json:"motivation"`
		Other      string `json:"other"`
		Collab     string `json:"collab"`
		Manual     string `json:"manual"`
		LetterHash string `json:"letterHash"`
		ResumeHash string `json:"resumeHash"`
		Submits    int    `json:"submits"`
		LinkedIn   string `json:"linkedIn"`
		FirstName  string `json:"firstName"`
		LastName   string `json:"lastName"`
		Email      string `json:"email"`
		Phone      string `json:"phone"`
	}
	err = employerEval(`(async()=>({motivation:document.querySelector('[name=motivation]')?.value||'',other:document.querySelector('[name=other]')?.value||'',collab:document.querySelector('[name=collaboration]')?.value||'',manual:document.querySelector('[name=manual]').value,linkedIn:document.querySelector('[name=linkedin]').value,firstName:document.querySelector('[name=first_name]')?.value||'',lastName:document.querySelector('[name=last_name]')?.value||'',email:document.querySelector('[name=email]')?.value||'',phone:document.querySelector('[name=phone]')?.value||'',letterHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=cover]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),resumeHash:Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256",await document.querySelector("[name=resume]").files[0].arrayBuffer()))).map(x=>x.toString(16).padStart(2,"0")).join(""),submits}))()`, &observedB)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("job B employer form: %+v", observedB)
	t.Logf("job B browser state: %+v", a.browsers.state(2))
	letterBPDF, err := coverLetterPDF(letterB)
	if err != nil {
		t.Fatal(err)
	}
	letterBSum := sha256.Sum256(letterBPDF)
	if observedB.Collab != collabB || observedB.Motivation != "" || observedB.Other != "" || observedB.Manual != "Already entered" {
		t.Fatalf("job B answers leaked or missed: %+v", observedB)
	}
	if observedB.LetterHash != hex.EncodeToString(letterBSum[:]) {
		t.Fatalf("job B letter differs from the selected new draft: %+v", observedB)
	}
	if observedB.ResumeHash != hex.EncodeToString(sum[:]) {
		t.Fatal("job B resume bytes differ")
	}
	if observedB.LinkedIn != "https://example.test/casey" || observedB.FirstName != "Casey" || observedB.LastName != "Fixture" || observedB.Email != "casey@example.test" || observedB.Phone != "Manual phone" {
		t.Fatalf("job B contacts wrong: %+v", observedB)
	}
	if observedB.Submits != 0 {
		t.Fatalf("job B submitted to the employer: %+v", observedB)
	}
	if done, err := a.jobByID(2); err != nil || done.Stage == StageApplied || done.SubmissionCount != 0 {
		t.Fatalf("job B Fill recorded a submission: %+v %v", done, err)
	}
	if st := a.browsers.state(1); st.Active {
		t.Fatalf("job B Fill reopened job A: %+v", st)
	}
	t.Log("Job B received its own selected résumé, adapted answer, and new letter; job A's originals and the unselected material stayed out of the employer form.")

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
	wait(`document.body.innerText.includes('journey-resume.pdf') && document.querySelector('input[name=workspace-resume]:checked')?.parentElement.innerText.includes('journey-resume.pdf') && document.querySelector('#letter-history')?.value==='1' && document.querySelector('#letter-body')?.value.includes('synthetic support role')`)
	click("Questions")
	wait(`document.body.innerText.includes('Delayed question')`)
	run(`(()=>{const e=document.querySelector('[aria-label="Question navigation"] select');e.value=Array.from(e.options).find(x=>x.textContent.includes('Delayed question')).value;e.dispatchEvent(new Event('change',{bubbles:true}))})()`)
	wait(fmt.Sprintf(`document.querySelector('#writing-answer')?.value === %q`, updatedAnswer))
	click("Apply")
	wait(`document.body.innerText.includes('Browser closed') || document.body.innerText.includes('saved materials are unchanged') || document.body.innerText.includes('disconnected')`)
	click("Submission history")
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
