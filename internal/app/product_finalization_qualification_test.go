//go:build qualification

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// These are production HTML/forms/scripts in an isolated Chromium process.
// No live employer URL is opened. The default-browser callback is a recording
// test double; it does not qualify the native browser handoff or file dialogs.
type finalizationBrowser struct {
	t            *testing.T
	ctx          context.Context
	chrome       *chromium.Session
	server       *httptest.Server
	page         string
	dir          string
	measurements []map[string]any
}

func newFinalizationBrowser(t *testing.T, a *App, name string) *finalizationBrowser {
	t.Helper()
	if os.Getenv("YOURMOVE_QUALIFY_FINALIZATION") != "1" {
		t.Skip("opt-in synthetic finalization journeys")
	}
	dir := os.Getenv("YOURMOVE_FINALIZATION_EVIDENCE")
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute YOURMOVE_FINALIZATION_EVIDENCE required")
	}
	dir = filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newMux(a))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	t.Cleanup(cancel)
	chrome, err := chromium.LaunchFixture(ctx, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { chrome.Close() })
	b := &finalizationBrowser{t: t, ctx: ctx, chrome: chrome, server: srv, page: srv.URL + "/jobs", dir: dir}
	b.wait(`document.readyState==='complete' && !!document.querySelector('.page-heading')`)
	t.Cleanup(func() {
		data, err := json.MarshalIndent(b.measurements, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "measurements.json"), data, 0600); err != nil {
			t.Error(err)
		}
	})
	return b
}

func (b *finalizationBrowser) run(script string) {
	b.t.Helper()
	if err := evalBriefJS(b.ctx, b.chrome, b.page, script, nil); err != nil {
		b.t.Fatal(err)
	}
}
func (b *finalizationBrowser) wait(script string) {
	b.t.Helper()
	waitBriefJS(b.t, b.ctx, b.chrome, b.page, script)
}
func (b *finalizationBrowser) check(script string) {
	b.t.Helper()
	assertBriefJS(b.t, b.ctx, b.chrome, b.page, script)
}
func (b *finalizationBrowser) click(selector string) {
	b.t.Helper()
	clickBriefUI(b.t, b.ctx, b.chrome, b.page, selector)
}
func (b *finalizationBrowser) press(key, code string, number int) {
	b.t.Helper()
	for _, kind := range []string{"keyDown", "keyUp"} {
		params := map[string]any{"type": kind, "key": key, "code": code, "windowsVirtualKeyCode": number}
		if key == "Enter" && kind == "keyDown" {
			params["text"] = "\r"
			params["unmodifiedText"] = "\r"
		}
		if err := b.chrome.FixtureCall(b.ctx, b.page, "Input.dispatchKeyEvent", params, nil); err != nil {
			b.t.Fatal(err)
		}
	}
}
func (b *finalizationBrowser) navigate(path, selector string) {
	b.t.Helper()
	b.run(`window.finalizationPreviousDocument=true`)
	if err := b.chrome.FixtureCall(b.ctx, b.page, "Page.navigate", map[string]any{"url": b.server.URL + path}, nil); err != nil {
		b.t.Fatal(err)
	}
	if b.server.URL+strings.Split(path, "#")[0] == b.page {
		if err := b.chrome.FixtureCall(b.ctx, b.page, "Page.reload", map[string]any{"ignoreCache": true}, nil); err != nil {
			b.t.Fatal(err)
		}
	}
	b.page = b.server.URL + strings.Split(path, "#")[0]
	b.wait(`!window.finalizationPreviousDocument && document.readyState==='complete' && !!document.querySelector(` + fmt.Sprintf("%q", selector) + `)`)
}
func (b *finalizationBrowser) submit(selector, target, ready string) {
	b.t.Helper()
	b.run(`window.finalizationPreviousDocument=true`)
	b.click(selector)
	b.page = b.server.URL + target
	b.wait(`!window.finalizationPreviousDocument && document.readyState==='complete' && (` + ready + `)`)
}
func (b *finalizationBrowser) capture(name, selector string, widths ...int) {
	b.t.Helper()
	for _, width := range widths {
		height := 800
		if width == 900 {
			height = 700
		}
		if width == 700 {
			height = 550
		}
		for _, theme := range []string{"light", "dark"} {
			connection, err := b.chrome.FixturePage(b.ctx, b.page)
			if err != nil {
				b.t.Fatal(err)
			}
			defer connection.Close()
			if err := connection.Call(b.ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
				b.t.Fatal(err)
			}
			var result struct {
				Result struct {
					Value map[string]any `json:"value"`
				} `json:"result"`
				Exception any `json:"exceptionDetails"`
			}
			script := `(async()=>{document.documentElement.dataset.theme=` + fmt.Sprintf("%q", theme) + `;document.documentElement.style.scrollBehavior='auto';document.querySelector(` + fmt.Sprintf("%q", selector) + `).scrollIntoView({block:'start',behavior:'instant'});await new Promise(requestAnimationFrame);await new Promise(requestAnimationFrame);return {innerWidth,innerHeight,scrollWidth:document.documentElement.scrollWidth,visibleRows:[...document.querySelectorAll('[data-job-row]:not([hidden])')].filter(e=>{const r=e.getBoundingClientRect();return r.top>=0&&r.bottom<=innerHeight}).length,firstJobTop:document.querySelector('[data-job-row]:not([hidden])')?.getBoundingClientRect().top??null}})()`
			if err := connection.Call(b.ctx, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true}, &result); err != nil {
				b.t.Fatal(err)
			}
			m := result.Result.Value
			if result.Exception != nil || m == nil {
				b.t.Fatalf("capture %s selector %s failed: %v", name, selector, result.Exception)
			}
			captureName := fmt.Sprintf("%s-%dx%d-%s", name, width, height, theme)
			m["capture"] = captureName
			b.measurements = append(b.measurements, m)
			if m["scrollWidth"].(float64) > m["innerWidth"].(float64) {
				b.t.Errorf("horizontal overflow %s: %v", captureName, m)
			}
			if m["innerWidth"].(float64) != float64(width) || m["innerHeight"].(float64) != float64(height) {
				b.t.Fatalf("viewport override did not apply %s: %v", captureName, m)
			}
			if name == "opportunities-25" && width == 900 && m["visibleRows"].(float64) < 2 {
				b.t.Errorf("900x700 list should expose two complete opportunities: %v", m)
			}
			var shot struct {
				Data string `json:"data"`
			}
			if err := connection.Call(b.ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &shot); err != nil {
				b.t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(shot.Data)
			if err != nil {
				b.t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(b.dir, captureName+".png"), data, 0600); err != nil {
				b.t.Fatal(err)
			}
			connection.Close()
		}
	}
}

func TestFinalizationManualApplicantJourney(t *testing.T) {
	a, workspace := testWorkspace(t)
	b := newFinalizationBrowser(t, a, "manual-journey")
	b.check(`document.querySelectorAll('form[action="/jobs"] button[type=submit]').length===1 && !!document.querySelector('#job_url').getClientRects().length`)
	b.capture("first-job", ".page-heading", 1100, 900, 700)
	b.run(`document.querySelector('#job_url').value='ftp://example.test/unsupported';document.querySelector('#job_title').value='Retain this title'`)
	b.submit(`form[action="/jobs"] button[type=submit]`, "/jobs", `document.querySelector('#job_url')?.getAttribute('aria-invalid')==='true'`)
	b.check(`document.querySelector('#job_url').value==='ftp://example.test/unsupported' && document.querySelector('#job_title').value==='Retain this title'`)
	b.capture("invalid-url-retained", ".page-heading", 1100)
	b.run(`document.querySelector('#job_url').value='https://careers.example.com/jobs/recovery-engineer';document.querySelector('#job_title').value='Recovery Engineer'`)
	b.submit(`form[action="/jobs"] button[type=submit]`, "/jobs/1", `!!document.querySelector('#description')`)
	b.check(`document.querySelector('#edit-details').open && !document.querySelector('#overview').hidden`)
	const description = "Responsibilities\nBuild dependable Go services and explain technical investigations.\n\nQualifications\nExperience with SQLite.\n• Communicate clearly."
	b.run(`(()=>{for(const [id,value] of Object.entries({description:` + fmt.Sprintf("%q", description) + `,title:'Recovery Engineer',company:'Synthetic Cooperative'})){const e=document.getElementById(id);e.value=value;e.dispatchEvent(new Event('input',{bubbles:true}))}})()`)
	b.submit(`.job-edit-form button[type=submit]`, "/jobs/1", `!!document.querySelector('[data-job-document]')`)
	job, err := a.jobByID(1)
	if err != nil || job.Title != "Recovery Engineer" || job.Company != "Synthetic Cooperative" || strings.ReplaceAll(job.Description, "\r\n", "\n") != description || !job.DescriptionManual {
		t.Fatalf("manual recovery persistence: %+v %v", job, err)
	}
	b.navigate("/jobs/1#prepare", "#prepare")
	b.click(`a[href="/candidate?return_job=1"]`)
	b.page = b.server.URL + "/candidate?return_job=1"
	b.wait(`document.readyState==='complete' && !!document.querySelector('#first_name')`)
	b.run(`(()=>{for(const [id,value]of Object.entries({first_name:'Alex',last_name:'Morgan',phone:'+1 202 555 0142',city:'Ann Arbor',region:'Michigan',country:'United States',linkedin:'https://linkedin.example.test/alex',github:'https://github.example.test/alex',portfolio:'https://portfolio.example.test/alex',pronouns:'They/them'}))document.getElementById(id).value=value;document.querySelector('[name=approve]').checked=true})()`)
	b.submit(`form[action="/candidate"] button[type=submit]`, "/candidate", `!!document.querySelector('[data-profile-validation-summary]')`)
	b.check(`document.getElementById('first_name').value==='Alex' && document.getElementById('last_name').value==='Morgan' && document.getElementById('city').value==='Ann Arbor' && document.getElementById('github').value==='https://github.example.test/alex' && document.getElementById('pronouns').value==='They/them' && document.querySelector('[name=approve]').checked && document.querySelector('#email').getAttribute('aria-invalid')==='true' && document.activeElement===document.querySelector('[data-profile-validation-summary]') && document.querySelector('[name=return_job]').value==='1'`)
	profile, err := a.candidateProfile()
	if err != nil || profile.Revision != 0 || profile.Approved {
		t.Fatalf("invalid approval changed saved facts: %+v %v", profile, err)
	}
	b.capture("profile-invalid-retained", ".page-heading", 1100, 700)
	b.run(`document.getElementById('email').value='alex@example.test'`)
	b.submit(`form[action="/candidate"] button[type=submit]`, "/candidate?notice=saved&return_job=1", `!document.querySelector('[data-profile-validation-summary]') && document.querySelector('#email')?.value==='alex@example.test'`)
	profile, err = a.candidateProfile()
	if err != nil || profile.Revision != 1 || !profile.Approved {
		t.Fatalf("valid profile approval: %+v %v", profile, err)
	}
	b.click(`a[href="/jobs/1#prepare"]`)
	b.page = b.server.URL + "/jobs/1"
	b.wait(`document.readyState==='complete' && !!document.querySelector('#resume_id') && !document.querySelector('#prepare').hidden`)
	b.click(`a[href="/resumes?return_job=1"]`)
	b.page = b.server.URL + "/resumes?return_job=1"
	b.wait(`document.readyState==='complete' && !!document.querySelector('input[type=file][name=resume]')`)
	resumeBytes := mustQualificationPDF(t)
	resumePath := filepath.Join(t.TempDir(), "Alex_Morgan_履歴書.pdf")
	if err = os.WriteFile(resumePath, resumeBytes, 0600); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	fileConnection, err := b.chrome.FixturePage(b.ctx, b.page)
	if err != nil {
		t.Fatal(err)
	}
	defer fileConnection.Close()
	if err = fileConnection.Call(b.ctx, "DOM.getDocument", map[string]any{}, &document); err != nil {
		t.Fatal(err)
	}
	var input struct {
		NodeID int `json:"nodeId"`
	}
	if err = fileConnection.Call(b.ctx, "DOM.querySelector", map[string]any{"nodeId": document.Root.NodeID, "selector": "input[type=file][name=resume]"}, &input); err != nil {
		t.Fatal(err)
	}
	if err = fileConnection.Call(b.ctx, "DOM.setFileInputFiles", map[string]any{"nodeId": input.NodeID, "files": []string{resumePath}}, nil); err != nil {
		t.Fatal(err)
	}
	fileConnection.Close()
	b.submit(`form[action="/resumes/import"] button[type=submit]`, "/resumes?notice=imported&return_job=1", `document.querySelector('.resume-info')?.textContent.includes('Alex_Morgan_履歴書.pdf')`)
	b.capture("document-imported", ".page-heading", 1100, 700)
	b.click(`a[href="/jobs/1#prepare"]`)
	b.page = b.server.URL + "/jobs/1"
	b.wait(`document.readyState==='complete' && !!document.querySelector('#resume_id option[value="1"]')`)
	b.run(`document.querySelector('#resume_id').value='1';document.querySelector('#resume_id').dispatchEvent(new Event('change',{bubbles:true}))`)
	b.submit(`form[action="/jobs/1/resume"] button[type=submit]`, "/jobs/1", `document.querySelector('#prepare').textContent.includes('Alex_Morgan_履歴書.pdf')`)
	job, _ = a.jobByID(1)
	if job.SelectedResumeID != 1 {
		t.Fatal("explicit selected resume not saved")
	}
	b.navigate("/jobs/1#cover-letter", "#cover-letter")
	const letter = "Dear hiring team,\n\nI investigate service incidents with careful reproduction, clear communication, and documented findings. This synthetic letter was written manually.\n\nAlex Morgan"
	b.run(`document.querySelector('#manual-letter-body').value=` + fmt.Sprintf("%q", letter) + `;document.querySelector('#manual-letter-body').dispatchEvent(new Event('input',{bubbles:true}))`)
	b.run(`window.finalizationManualEditor=document.querySelector('#manual-letter-body')`)
	b.check(`document.querySelector('form[action$="/cover-letter/manual"]').dataset.dirty==='true' && document.querySelector('form[action$="/cover-letter/manual"] [data-answer-status]').textContent==='Unsaved changes'`)
	b.click(`[data-tab-target="prepare"]`)
	b.click(`[data-tab-target="cover-letter"]`)
	b.check(`document.querySelector('#manual-letter-body')===window.finalizationManualEditor && finalizationManualEditor.value===` + fmt.Sprintf("%q", letter))
	b.click(`.nav-item[href="/candidate"]`)
	b.wait(`document.querySelector('.leave-materials-dialog')?.open`)
	b.check(`document.activeElement===document.querySelector('.leave-materials-dialog [data-stay]')`)
	b.capture("manual-letter-unsaved-dialog", ".leave-materials-dialog", 1100, 700)
	b.press("Escape", "Escape", 27)
	b.check(`!document.querySelector('.leave-materials-dialog').open && document.querySelector('#manual-letter-body').value===` + fmt.Sprintf("%q", letter))
	b.click(`.nav-item[href="/candidate"]`)
	b.wait(`document.querySelector('.leave-materials-dialog')?.open`)
	b.check(`document.activeElement===document.querySelector('.leave-materials-dialog [data-stay]')`)
	b.press("Enter", "Enter", 13)
	b.wait(`!document.querySelector('.leave-materials-dialog').open && location.pathname==='/jobs/1'`)
	b.check(`document.querySelector('#manual-letter-body').value===` + fmt.Sprintf("%q", letter))
	b.click(`.nav-item[href="/candidate"]`)
	b.wait(`document.querySelector('.leave-materials-dialog')?.open`)
	b.submit(`.leave-materials-dialog [data-leave]`, "/candidate", `!!document.querySelector('#first_name')`)
	var manualDrafts int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM cover_letters WHERE job_id=1`).Scan(&manualDrafts); err != nil || manualDrafts != 0 {
		t.Fatal("discarding unsaved first letter persisted a draft", err)
	}
	b.navigate("/jobs/1#cover-letter", "#cover-letter")
	b.check(`document.querySelector('#manual-letter-body').value===''`)
	b.run(`document.querySelector('#manual-letter-body').value=` + fmt.Sprintf("%q", letter) + `;document.querySelector('#manual-letter-body').dispatchEvent(new Event('input',{bubbles:true}))`)
	b.submit(`form[action="/jobs/1/cover-letter/manual"] button[type=submit]`, "/jobs/1?cover=saved&draft=1", `!!document.querySelector('#letter-body')`)
	savedLetter, err := a.coverLetterByID(1, 1)
	if err != nil || strings.ReplaceAll(savedLetter.Body, "\r\n", "\n") != letter {
		t.Fatalf("manual letter not persisted: %+v %v", savedLetter, err)
	}
	b.navigate("/jobs/1#prepare", "#prepare")
	b.capture("manual-prepared", "#prepare", 1100, 900, 700)
	// The production endpoint invokes only the saved-URL browser callback. The
	// test double records the request and deliberately opens no external site.
	opened := []string{}
	a.openPosting = func(target string) error { opened = append(opened, target); return nil }
	b.run(`(async()=>{const r=await fetch('/jobs/1/open',{method:'POST'});if(!r.ok)throw Error('manual opening failed')})()`)
	if len(opened) != 1 || opened[0] != job.URL {
		t.Fatalf("manual opening callback: %v", opened)
	}
	if len(a.browsers.sessions)+len(a.browsers.handoffs)+len(a.browsers.answers)+len(a.browsers.letters) != 0 {
		t.Fatal("local preparation/manual navigation armed employer transfer")
	}
	if err = a.saveJob(Job{URL: "https://careers.example.com/jobs/second-engineer", Host: "careers.example.com", Title: "Second Application Engineer", Company: "Second Synthetic Cooperative"}); err != nil {
		t.Fatal(err)
	}
	b.navigate("/jobs/1#prepare", "#prepare")
	b.click("#record-submission > summary")
	b.run(`document.querySelector('[data-record-completion] [name=notes]').value='Synthetic controlled manual completion; no live employer submission';document.querySelector('[data-record-completion] [name=confirm]').checked=true`)
	b.submit(`[data-record-completion] button[type=submit]`, "/jobs/1", `!!document.querySelector('#history .submission-success')`)
	b.capture("completion-recorded", "#history", 1100, 700)
	b.submit(`form[action="/jobs/1/next"] button[type=submit]`, "/jobs/2", `!document.querySelector('#prepare').hidden && document.querySelector('h1')?.textContent.includes('Second Application Engineer')`)
	b.check(`document.querySelector('#resume_id option[value="1"]').textContent.includes('Alex_Morgan_履歴書.pdf') && document.querySelector('#resume_id').value==='0'`)
	second, _ := a.jobByID(2)
	if second.SelectedResumeID != 0 {
		t.Fatal("new job implicitly assigned resume")
	}
	retained, _ := a.candidateProfile()
	if retained.Revision != profile.Revision || !retained.Approved || retained.Email != profile.Email {
		t.Fatal("shared approved profile lost on next job")
	}
	b.capture("second-job-existing-materials", "#prepare", 1100, 700)
	b.navigate("/candidate", "#first_name")
	b.check(`document.querySelector('#first_name').value==='Alex' && !document.querySelector('[data-return-job]') && !document.querySelector('[name=approve]').checked`)
	b.run(`document.querySelector('#first_name').value='';document.querySelector('[name=approve]').checked=true`)
	b.submit(`form[action="/candidate"] button[type=submit]`, "/candidate", `!!document.querySelector('[data-profile-validation-summary]')`)
	previous, _ := a.candidateProfile()
	if previous.Revision != profile.Revision || !previous.Approved || previous.FirstName != "Alex" {
		t.Fatal("failed approval changed previously approved revision")
	}
	b.run(`document.querySelector('[name=approve]').checked=false;document.querySelector('#email').value=''`)
	b.submit(`form[action="/candidate"] button[type=submit]`, "/candidate?notice=saved", `!document.querySelector('[data-profile-validation-summary]') && document.querySelector('#first_name')?.value===''`)
	draftProfile, _ := a.candidateProfile()
	if draftProfile.Approved || draftProfile.Revision != profile.Revision+1 || draftProfile.FirstName != "" || draftProfile.Email != "" {
		t.Fatal("incomplete unapproved draft did not save")
	}
	if len(a.browsers.sessions)+len(a.browsers.handoffs)+len(a.browsers.answers)+len(a.browsers.letters) != 0 {
		t.Fatal("completion or next job armed transfer")
	}
	// Close the source connection and reopen the same SQLite + document files.
	b.server.Close()
	if err = a.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(workspace, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reopened := newTestApp(t, db, a.resumeDir)
	job, err = reopened.jobByID(1)
	if err != nil || job.SelectedResumeID != 1 || job.SubmissionCount != 1 || job.Stage != StageApplied {
		t.Fatalf("reopened job association: %+v %v", job, err)
	}
	resume, err := reopened.resumeByID(1)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := reopened.readResumeBytes(resume)
	if err != nil || sha256.Sum256(exact) != sha256.Sum256(resumeBytes) {
		t.Fatal("reopened document bytes changed", err)
	}
	savedLetter, err = reopened.coverLetterByID(1, 1)
	if err != nil || strings.ReplaceAll(savedLetter.Body, "\r\n", "\n") != letter {
		t.Fatal("reopened letter changed", err)
	}
	t.Log("PASS controlled Chromium: first URL saved; manual description/correction retained; invalid approval preserves fields/consent and no saved revision; valid approval; contextual profile/Documents return; browser file-input import; explicit resume selection; manual letter; callback-only manual open; explicit local completion; Next; same profile/resume available without auto-assignment; SQLite/document reopen. Native picker, native browser launch and employer submission NOT VERIFIED.")
}

func TestFinalizationOpportunityStress(t *testing.T) {
	a, _ := testWorkspace(t)
	if os.Getenv("YOURMOVE_QUALIFY_FINALIZATION") != "1" {
		t.Skip("opt-in synthetic finalization stress")
	}
	titles := []string{"Support Engineer", "Customer Reliability Engineer", "Technical Implementation Engineer — Secure Clinical Data Exchange and International Partner Integrations", "データ連携エンジニア · Customer Success", "Senior Developer Advocate", "Solutions Engineer", "Customer Support Specialist", "Infrastructure Engineer"}
	companies := []string{"Cedar Systems", "Maple Harbor", "A Very Long Synthetic Employer Name for International Research and Public Interest Technology", "協同組合 Example", "Northstar"}
	for i := 0; i < 25; i++ {
		stage := StageReview
		if i%7 == 0 {
			stage = StageReady
		}
		if i%11 == 0 {
			stage = StageClosed
		}
		priority := PriorityNormal
		if i%4 == 0 {
			priority = PriorityHigh
		}
		if i%6 == 0 {
			priority = PriorityLow
		}
		if err := a.saveJob(Job{URL: fmt.Sprintf("https://example.test/roles/%d", i), Host: "example.test", Title: titles[i%len(titles)], Company: companies[i%len(companies)], Stage: stage, Priority: priority}); err != nil {
			t.Fatal(err)
		}
		if err := a.updateJob(Job{ID: int64(i + 1), Title: titles[i%len(titles)], Company: companies[i%len(companies)], Stage: stage, Priority: priority}); err != nil {
			t.Fatal(err)
		}
	}
	resume, err := a.importResume("Alex_Morgan_履歴書.pdf", bytes.NewReader(mustQualificationPDF(t)))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{2, 4, 7, 10, 13, 16, 19, 22} {
		if err := a.assignResume(id, resume.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int64{5, 9} {
		if err := a.recordSubmission(id, "Synthetic completed stress record", reviewedSubmissionToken(t, a, id)); err != nil {
			t.Fatal(err)
		}
	}
	b := newFinalizationBrowser(t, a, "opportunity-stress")
	b.check(`document.querySelectorAll('[data-job-row]').length===25`)
	b.capture("opportunities-25", ".page-heading", 1100, 900, 700)
	b.run(`const search=document.querySelector('[data-job-search]');search.value='zzzz-no-synthetic-match';search.dispatchEvent(new Event('input',{bubbles:true}))`)
	b.check(`!document.querySelector('#filtered-empty').hidden && [...document.querySelectorAll('[data-job-row]')].every(e=>e.hidden)`)
	b.capture("filtered-empty", ".page-heading", 1100, 700)
	b.click("[data-clear-filters]")
	b.check(`document.querySelector('[data-job-search]').value==='' && !!document.querySelector('[data-job-row]:not([hidden])')`)
	t.Log("PASS 25 varied synthetic opportunities: long/Unicode titles and employers, priorities/stages, three desktop browser viewports × two themes, no horizontal overflow; zero-result recovery and Clear filters.")
}
