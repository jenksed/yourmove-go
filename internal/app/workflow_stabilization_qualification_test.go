//go:build qualification

package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Real Chromium writes and readback against loopback only. The profile is
// deliberately absent; the unsaved editor never enters a transfer request.
func TestControlledUnapprovedProfileJourney(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_WORKFLOW") != "1" {
		t.Skip("isolated workflow qualification")
	}
	var uploads, submitted atomic.Int64
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" {
			uploads.Add(1)
			w.WriteHeader(204)
			return
		}
		if r.URL.Path == "/submit" {
			submitted.Add(1)
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><form class="ashby-application-form" action="/submit" method="POST"><label>Name<input name="name"></label><label>Why this team?<textarea name="motivation"></textarea></label><label>Website<input name="website"></label><section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume"></section><button>Submit application</button></form><script>document.querySelector('[name=resume]').onchange=()=>fetch('/upload',{method:'POST'});</script>`))
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	employer, err := chromium.LaunchFixture(ctx, fixture.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer employer.Close()
	a, _ := testWorkspace(t)
	if err = a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic support engineer"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("Synthetic.pdf", bytes.NewReader(mustQualificationPDF(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.saveFamily(RoleFamily{Name: "Synthetic support", TitleTerms: "support engineer"}); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResumeFamily(resume.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	live := schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{{ID: "browser:name", Label: "Name", Kind: "text"}, {ID: "browser:motivation", Label: "Why this team?", Kind: "textarea"}, {ID: "browser:website", Label: "Website", Kind: "text"}}}
	if _, err = a.saveInspection(1, live); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	for _, f := range live.Fields[1:] {
		if rec := postApp(mux, "/jobs/1/answers/save", mapValues("question_key", questionKey(testAshbyPosting, "ashby", f), "revision", "0", "destination", testAshbyPosting, "answer", "Synthetic answer for "+f.Label), "", ""); rec.Code != 303 {
			t.Fatal(rec.Code, rec.Body.String())
		}
	}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		return &contactFixtureSession{Session: employer, fixture: fixture.URL, saved: testAshbyPosting}, nil
	}
	defer a.browsers.closeAll()
	router := http.NewServeMux()
	router.HandleFunc("GET /jobs/1/application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1#prepare", 303) })
	router.Handle("/", mux)
	server := httptest.NewServer(router)
	defer server.Close()
	page := server.URL + "/jobs/1"
	ui, err := chromium.LaunchFixture(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Close()
	waitBriefPage(t, ctx, ui, page, "#prepare")
	if err = evalBriefJS(ctx, ui, page, `document.querySelector('.workspace-tab[data-tab-target=prepare]').click();const editor=document.querySelector('[name=answer]');editor.value='Unsaved Synthetic Name';editor.dispatchEvent(new Event('input',{bubbles:true}));true`, nil); err != nil {
		t.Fatal(err)
	}
	assertBriefJS(t, ctx, ui, page, `document.querySelector('[data-contact-exclusion]').textContent.includes('Contact filling disabled') && document.querySelectorAll('.pending-answer-scope dt').length===2 && document.querySelector('[data-transfer-unsaved]').textContent.includes('Name')`)
	if err = evalBriefJS(ctx, ui, page, `window.workflowBeforeFill=true;document.querySelector('form[action$="/continue"] button[type=submit]').click();true`, nil); err != nil {
		t.Fatal(err)
	}
	waitBriefJS(t, ctx, ui, page, `!window.workflowBeforeFill && document.querySelector('[name=answer]')?.value==='Unsaved Synthetic Name'`)
	for deadline := time.Now().Add(30 * time.Second); ; {
		state := a.browsers.state(1)
		if state.ResumeStatus == "selected" && state.AnswerStatus == "complete" {
			if len(state.AnswerOutcomes) != 2 || state.ContactStatus != "" {
				t.Fatalf("incorrect independent outcomes: %+v", state)
			}
			for _, out := range state.AnswerOutcomes {
				if out.Status != "filled" {
					t.Fatalf("not verified: %+v", out)
				}
			}
			break
		}
		if time.Now().After(deadline) {
			var diagnostic string
			_ = evalBriefJS(ctx, ui, page, `document.body.innerText`, &diagnostic)
			t.Fatalf("transfer did not finish: %+v UI=%s", state, diagnostic)
		}
		time.Sleep(100 * time.Millisecond)
	}
	assertBriefJS(t, ctx, employer, fixture.URL+"/application", `document.querySelector('[name=name]').value==='' && document.querySelector('[name=motivation]').value==='Synthetic answer for Why this team?' && document.querySelector('[name=website]').value==='Synthetic answer for Website' && document.querySelector('[name=resume]').files.length===1`)
	if uploads.Load() != 1 || submitted.Load() != 0 {
		t.Fatalf("uploads=%d submissions=%d", uploads.Load(), submitted.Load())
	}
	// A later profile approval offers its qualified name independently, without
	// re-uploading the resume or replacing a manually changed website answer.
	if err = evalBriefJS(ctx, employer, fixture.URL+"/application", `document.querySelector('[name=website]').value='Manual employer value';true`, nil); err != nil {
		t.Fatal(err)
	}
	if err = a.saveCandidateProfile(CandidateProfile{FirstName: "Approved", LastName: "Fixture", Email: "approved@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	if rec := postApp(mux, "/jobs/1/browser/observe", mapValues("return_to", "prepare"), "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	scope, form := renderedContinuation(t, a)
	if scope.Get("include_resume") != "" || scope.Get("include_answers") != "yes" || !strings.Contains(form, "Approved Fixture") {
		t.Fatal("profile continuation mixed document authority", scope, form)
	}
	if rec := postApp(mux, "/jobs/1/continue", scope, "", ""); rec.Code != 303 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	for deadline := time.Now().Add(15 * time.Second); ; {
		state := a.browsers.state(1)
		if state.AnswerStatus == "complete" && len(state.AnswerOutcomes) == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("approved fact did not finish independently: %+v", state)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("approved outcomes: %+v", a.browsers.state(1).AnswerOutcomes)
	assertBriefJS(t, ctx, employer, fixture.URL+"/application", `document.querySelector('[name=name]').value==='Approved Fixture' && document.querySelector('[name=website]').value==='Manual employer value'`)
	if uploads.Load() != 1 || submitted.Load() != 0 {
		t.Fatal("profile continuation replayed a file or submitted")
	}
	// Restore the original profile state for the independent navigation checks.
	if err = a.saveCandidateProfile(CandidateProfile{}); err != nil {
		t.Fatal(err)
	}

	var answers int
	if err = a.db.QueryRow("SELECT count(*) FROM application_answers").Scan(&answers); err != nil || answers != 2 {
		t.Fatal("unsaved answer persisted", answers, err)
	}
	// The exact dirty value survives independent filling, but navigation is an
	// explicit decision. Exercise the native dialog keyboard implementation too.
	if err = evalBriefJS(ctx, ui, page, `window.confirm=()=>{throw Error('native confirm forbidden')};document.querySelector('a[href="/candidate"]').click();true`, nil); err != nil {
		t.Fatal(err)
	}
	assertBriefJS(t, ctx, ui, page, `document.querySelector('dialog').open && document.activeElement.hasAttribute('data-stay')`)
	if err = evalBriefJS(ctx, ui, page, `document.querySelector("[data-stay]").click();true`, nil); err != nil {
		t.Fatal(err)
	}
	assertBriefJS(t, ctx, ui, page, `!document.querySelector('dialog').open && document.querySelector('[name=answer]').value==='Unsaved Synthetic Name'`)
	// Preserve a second dirty form and the letter together through Stay.
	if _, err = a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,'Synthetic saved letter','fixture','none',1,'master',1,'role','job','now')`); err != nil {
		t.Fatal(err)
	}
	// Leave goes to the original complete URL, suppressing beforeunload once.
	if err = evalBriefJS(ctx, ui, page, `const link=document.querySelector('a[href="/candidate"]');link.href='/candidate?from=workflow#profile';link.click();true`, nil); err != nil {
		t.Fatal(err)
	}
	if err = evalBriefJS(ctx, ui, page, `document.querySelector("[data-leave]").click();true`, nil); err != nil {
		t.Fatal(err)
	}
	page = server.URL + "/candidate?from=workflow"
	waitBriefJS(t, ctx, ui, page, `location.pathname==='/candidate' && location.search==='?from=workflow' && location.hash==='#profile' && !document.querySelector('dialog')?.open`)
	// Re-enter the real materials page with a saved letter and dirty three forms.
	if err = evalBriefJS(ctx, ui, page, `location.assign('/jobs/1#prepare');true`, nil); err != nil {
		t.Fatal(err)
	}
	page = server.URL + "/jobs/1"
	waitBriefPage(t, ctx, ui, page, "#prepare")
	if err = evalBriefJS(ctx, ui, page, `for(const editor of [document.querySelectorAll('[name=answer]')[0],document.querySelectorAll('[name=answer]')[1],document.querySelector('[name=body]')]){editor.value+=' unsaved fixture';editor.dispatchEvent(new Event('input',{bubbles:true}));}window.confirm=()=>{throw Error('native confirm forbidden')};true`, nil); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{"/candidate", "/resumes", "/answer-library", "/jobs"} {
		if err = evalBriefJS(ctx, ui, page, `document.querySelector('a[href="`+destination+`"]').click();true`, nil); err != nil {
			t.Fatal(err)
		}
		assertBriefJS(t, ctx, ui, page, `document.querySelector('dialog').open && document.querySelectorAll('form[data-dirty=true]').length===3`)
		if err = ui.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Tab", "code": "Tab", "windowsVirtualKeyCode": 9, "modifiers": 8}, nil); err != nil {
			t.Fatal(err)
		}
		assertBriefJS(t, ctx, ui, page, `document.activeElement.hasAttribute('data-leave')`)
		if err = ui.FixtureCall(ctx, page, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Escape", "code": "Escape", "windowsVirtualKeyCode": 27}, nil); err != nil {
			t.Fatal(err)
		}
		waitBriefJS(t, ctx, ui, page, `!document.querySelector('dialog').open && document.querySelectorAll('form[data-dirty=true]').length===3 && document.querySelector('[name=body]').value.endsWith(' unsaved fixture')`)
	}
	if err = evalBriefJS(ctx, ui, page, `document.querySelector('a[href="/resumes"]').click();true`, nil); err != nil {
		t.Fatal(err)
	}
	if err = evalBriefJS(ctx, ui, page, `document.querySelector("[data-leave]").click();true`, nil); err != nil {
		t.Fatal(err)
	}
	page = server.URL + "/resumes"
	waitBriefJS(t, ctx, ui, page, `location.pathname==='/resumes' && !document.querySelector('dialog')?.open`)

	// Saved records remain unchanged after acknowledged discard.
	profile, _ := a.candidateProfile()
	if profile.Approved {
		t.Fatal("profile silently approved")
	}
	var preserved string
	if err = a.db.QueryRow("SELECT group_concat(answer,'|') FROM (SELECT answer FROM application_answers ORDER BY answer)").Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if preserved != "Synthetic answer for Website|Synthetic answer for Why this team?" {
		t.Fatalf("discard changed persisted answers: %q", preserved)
	}
}
