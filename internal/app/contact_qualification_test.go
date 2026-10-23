//go:build qualification

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// This adapter changes only the test observation's origin. Actual mutations
// still use Chromium's private loopback owner and the live local form.
type contactFixtureSession struct {
	*chromium.Session
	fixture, saved string
	observes       atomic.Int64
}

func (s *contactFixtureSession) Observe(ctx context.Context) (browser.Observation, error) {
	s.observes.Add(1)
	o, err := s.Session.Observe(ctx)
	if err == nil {
		o.URL = strings.Replace(o.URL, s.fixture, s.saved, 1)
	}
	return o, err
}

func TestControlledContactJourney(t *testing.T) {
	controlledContactJourney(t, false)
}

func TestControlledCombinedContinuation(t *testing.T) {
	controlledContactJourney(t, true)
}

func controlledContactJourney(t *testing.T, combined bool) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled Chromium qualification")
	}
	var mu sync.Mutex
	uploads := 0
	uploadHash := ""
	additionalQuestion := ""
	if combined {
		additionalQuestion = `<label>Describe a difficult support incident you resolved<textarea name="incident-story"></textarea></label>`
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, "multipart", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			file, _, err := r.FormFile("resume")
			if err != nil {
				http.Error(w, "file", 400)
				return
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "read", 400)
				return
			}
			sum := sha256.Sum256(data)
			mu.Lock()
			uploads++
			uploadHash = hex.EncodeToString(sum[:])
			mu.Unlock()
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!doctype html><h1>Synthetic Ashby application</h1><form class="ashby-application-form">
<label>First name<input name="first_name" autocomplete="given-name"></label>
<label>Last name<input name="last_name" autocomplete="family-name"></label>
<label>Email<input name="email" type="email" autocomplete="email"></label>
<label>Phone<input name="phone" type="tel" autocomplete="tel"></label>
<label>Manager email<input name="manager_email" type="email"></label>
<label>Work authorization<input name="sponsorship"></label>
<section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input type="file" name="resume" accept=".pdf"></section>
<section data-field-entry-id="cover"><div class="ashby-application-form-question-title">Cover Letter</div><input type="file" name="cover"></section>
<label>Why this team?<textarea name="motivation"></textarea></label>
<!-- CHECKPOINT1_ADDITIONAL_QUESTION -->
<label>Preferred working pattern<select name="working-pattern"><option value="">Choose</option><option value="remote">Remote</option><option value="office">Office</option></select></label>
<label>Available for occasional travel<input name="travel" type="checkbox"></label>
<fieldset><legend>Preferred interview time</legend><label>Morning<input name="interview" type="radio" value="am"></label><label>Afternoon<input name="interview" type="radio" value="pm"></label></fieldset>
<button type="submit">Submit application</button></form>
<script>
document.querySelector('[name=motivation]').addEventListener('input',e=>{const old=e.target;setTimeout(()=>{const next=old.cloneNode();next.value=old.value;old.replaceWith(next)},50)},{once:true});
document.querySelector('[name=working-pattern]').addEventListener('change',()=>{if(!document.querySelector('[name=remote-tools]')){const label=document.createElement('label');label.textContent='Which remote support tools have you used?';const input=document.createElement('textarea');input.name='remote-tools';label.append(input);document.querySelector('form').append(label)}});
window.contactEvents={input:0,change:0,submitted:0};
document.querySelector('form').addEventListener('submit',e=>{e.preventDefault();contactEvents.submitted++});
for(const type of ['input','change']) document.addEventListener(type,e=>{if(['first_name','last_name','email','phone'].includes(e.target.name)) contactEvents[type]++});
document.querySelector('[name=first_name]').addEventListener('input',e=>{const old=e.target;setTimeout(()=>{const fresh=old.cloneNode(true);fresh.value=old.value;old.replaceWith(fresh)},50)},{once:true});
document.querySelector('[name=resume]').addEventListener('change',async e=>{const body=new FormData();body.append('resume',e.target.files[0]);await fetch('/upload',{method:'POST',body})});
</script>`
		html = strings.Replace(html, "<!-- CHECKPOINT1_ADDITIONAL_QUESTION -->", additionalQuestion, 1)
		fmt.Fprint(w, html)
	}))
	defer fixture.Close()
	base := fixture.URL + "/fixture/job"
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	employer := base + "/application"
	a, _ := testWorkspace(t)
	requireQualificationStorage(t, a)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic contact qualification", Description: "Support customers through production incidents"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("approved-resume.pdf", bytes.NewReader(mustQualificationPDF(t)))
	if err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Managed production incidents")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	var generatedAnswerCalls atomic.Int32
	const generatedIncidentAnswer = "I reviewed the incident timeline, coordinated the response, and documented follow-up steps."
	if combined {
		addCareerEvidenceForSearch(t, a, careerEvidenceContent{Title: "Synthetic support incident", Situation: "A production customer workflow was interrupted.", Actions: "I reviewed the logs, coordinated the investigation, and documented follow-up.", OutcomeState: "unknown"}, true)
		a.applicationAnswerGenerate = func(context.Context, string, string) (string, error) {
			generatedAnswerCalls.Add(1)
			return generatedIncidentAnswer, nil
		}
	}
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		return "Dear Hiring Team, I bring verified technical support experience to this role.", nil
	}
	p := CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Phone: "202-555-0104", Approved: true}
	if err := a.saveCandidateProfile(p); err != nil {
		t.Fatal(err)
	}
	a.browsers.pollEvery = 80 * time.Millisecond
	fixtureSession := &contactFixtureSession{Session: chrome, fixture: base, saved: testAshbyPosting}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fixtureSession, nil }
	defer a.browsers.closeAll()
	app := httptest.NewServer(newMux(a))
	defer app.Close()
	ui := app.URL + "/jobs/1"
	evaluate := func(page, expression string, out any) error {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &reply); err != nil {
			return err
		}
		if len(reply.Exception) > 0 {
			return fmt.Errorf("fixture JavaScript: %s", reply.Exception)
		}
		if out != nil {
			return json.Unmarshal(reply.Result.Value, out)
		}
		return nil
	}
	// Saves now return to an exact current question; keep the fixture tab target
	// exact as well. This helper changes no transfer or content assertions.
	questionPage := func(label string) string {
		t.Helper()
		job, err := a.jobByID(1)
		if err != nil {
			t.Fatal(err)
		}
		profile, err := a.candidateProfile()
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := a.prepareQuestions(job, profile)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range prepared.Questions {
			if q.Field.Label == label {
				return app.URL + "/jobs/1?question=" + q.Key
			}
		}
		t.Fatalf("missing observed question %q", label)
		return ""
	}

	wait := func(description string, fn func() bool) {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			if fn() {
				return
			}
			time.Sleep(60 * time.Millisecond)
		}
		var diagnostic string
		_ = evaluate(ui, `JSON.stringify({url:location.href,body:document.body.innerText.slice(0,1000),questions:[...document.querySelectorAll('.application-question-form')].map(f=>({label:f.querySelector('legend,h3,h4')?.textContent||f.textContent.slice(0,120),ai:!!f.querySelector('[data-answer-ai]')}))})`, &diagnostic)
		t.Fatalf("timed out: %s; UI=%s", description, diagnostic)
	}
	wait("employer form", func() bool {
		var ready bool
		return evaluate(employer, "!!document.querySelector('[name=resume]')", &ready) == nil && ready
	})
	if err := chrome.FixtureCall(ctx, employer, "Target.createTarget", map[string]any{"url": ui + "#prepare"}, nil); err != nil {
		t.Fatal(err)
	}
	wait("YourMove UI", func() bool {
		var ready bool
		return evaluate(ui, "!!document.querySelector('#resume_id')", &ready) == nil && ready
	})
	// Normal UI actions: choose a resume, save it, return to Prepare, approve, transfer.
	if err := evaluate(ui, `(()=>{const s=document.querySelector('#resume_id');s.value='`+strconv.FormatInt(resume.ID, 10)+`';s.dispatchEvent(new Event('change',{bubbles:true}));s.form.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	wait("saved resume and displayed approved contacts", func() bool {
		var ready bool
		return evaluate(ui, `!!document.querySelector('.g55-handoff-form input[name=transfer_contacts]') && document.body.textContent.includes('casey@example.test')`, &ready) == nil && ready
	})
	if err := evaluate(ui, `document.querySelector('.workspace-tab[data-tab-target=prepare]').click();true`, nil); err != nil {
		t.Fatal(err)
	}
	capture := func(width int, name string) {
		t.Helper()
		dir := os.Getenv("YOURMOVE_EVIDENCE_DIR")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		shot, err := chrome.FixtureScreenshot(ctx, ui, width)
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	capture(1280, "prepare-1280.png")
	capture(390, "prepare-390.png")
	start := time.Now()
	if err := evaluate(ui, `(()=>{const f=document.querySelector('.g55-handoff-form');f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	wait("contact transfer and resume selection", func() bool {
		s := a.browsers.state(1)
		return s.ContactStatus == "complete" && s.ResumeStatus == "selected"
	})
	transferTime := time.Since(start)
	wait("result visible in Prepare", func() bool {
		var ready bool
		return evaluate(ui, `location.hash==='#prepare' && document.body.textContent.includes('4 contact fields filled.')`, &ready) == nil && ready
	})
	uiTime := time.Since(start)
	var values struct {
		First, Last, Email, Phone, Manager, Sensitive string
		Input, Change, Submitted                      int
		Cover                                         int
	}
	expression := `({First:document.querySelector('[name=first_name]').value,Last:document.querySelector('[name=last_name]').value,Email:document.querySelector('[name=email]').value,Phone:document.querySelector('[name=phone]').value,Manager:document.querySelector('[name=manager_email]').value,Sensitive:document.querySelector('[name=sponsorship]').value,Input:contactEvents.input,Change:contactEvents.change,Submitted:contactEvents.submitted,Cover:document.querySelector('[name=cover]').files.length})`
	if err := evaluate(employer, expression, &values); err != nil {
		t.Fatal(err)
	}
	if values.First != p.FirstName || values.Last != p.LastName || values.Email != p.Email || values.Phone != p.Phone || values.Manager != "" || values.Sensitive != "" || values.Input != 4 || values.Change != 4 || values.Submitted != 0 || values.Cover != 0 {
		t.Fatalf("incorrect transfer or events: %+v", values)
	}
	mu.Lock()
	count, hash := uploads, uploadHash
	mu.Unlock()
	if count != 1 || hash != resume.SHA256 {
		t.Fatalf("resume delivery=%d hash=%s", count, hash)
	}
	if err := evaluate(employer, `(()=>{const e=document.querySelector('[name=last_name]');e.value='Manual correction';e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	before := fixtureSession.observes.Load()
	response := postApp(newMux(a), "/jobs/1/browser/observe", nil, "", "")
	if response.Code != http.StatusSeeOther {
		t.Fatal(response.Code)
	}
	wait("normal monitoring continuation", func() bool { return fixtureSession.observes.Load() >= before+2 })
	if err := evaluate(employer, expression, &values); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	count = uploads
	mu.Unlock()
	if values.Last != "Manual correction" || values.Input != 5 || count != 1 {
		t.Fatalf("manual work replayed or duplicate attachment: %+v uploads=%d", values, count)
	}
	capture(1280, "result-1280.png")
	// B/C normal workspace continuation, with no new browser or resume handoff.
	letterBody := "Dear Hiring Team, I bring verified technical support experience to this role. I value careful customer communication."
	if err := evaluate(ui, `document.querySelector('.prepare-writer > summary').click();true`, nil); err != nil {
		t.Fatal(err)
	}
	if err := evaluate(ui, `(()=>{const f=document.querySelector('form[action$="/cover-letter/generate"]');f.querySelector('[name=confirm_external]').click();f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	ui = app.URL + "/jobs/1?cover=generated"
	wait("generated letter through normal navigation", func() bool {
		var ready bool
		return evaluate(ui, `!!document.querySelector('form[action$="/cover-letter/save"]')`, &ready) == nil && ready
	})
	if err := evaluate(ui, `(()=>{const f=document.querySelector('form[action$="/cover-letter/save"]');const e=f.querySelector('textarea');e.value='`+letterBody+`';e.dispatchEvent(new Event('input',{bubbles:true}));f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	ui = app.URL + "/jobs/1?cover=saved&draft=1"
	wait("saved letter through real save", func() bool {
		var ready bool
		return evaluate(ui, `document.body.textContent.includes('Your edits were saved')`, &ready) == nil && ready
	})
	wait("save-return focus after the load frame", func() bool {
		var ready bool
		return evaluate(ui, `(()=>{const action=document.querySelector('#application-action');const bounds=action.getBoundingClientRect();return bounds.top>=0&&bounds.top<innerHeight})()`, &ready) == nil && ready
	})
	var returnedToScope bool
	if err := evaluate(ui, `(()=>{const action=document.querySelector('#application-action');const form=action.querySelector('form[action$="/continue"]');const body=form.querySelector('.transfer-letter-scope');return location.hash==='#prepare'&&!document.querySelector('.prepare-writer').open&&action.getBoundingClientRect().top>=0&&action.getBoundingClientRect().top<innerHeight&&body.textContent===`+strconv.Quote(letterBody)+`&&!!form.querySelector('button')})()`, &returnedToScope); err != nil || !returnedToScope {
		t.Fatal("successful save did not return directly to exact scope", err)
	}
	t.Log("save → Prepare: zero navigation or disclosure actions to locate current exact scope; authorization remains a separate button")
	if dir := os.Getenv("YOURMOVE_SCOPE_EVIDENCE"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{1280, 1050, 800, 700, 390} {
			for _, theme := range []string{"light", "dark"} {
				shot, err := chrome.FixtureScreenshotAt(ctx, ui, width, "#prepare", theme)
				if err != nil {
					t.Fatal(err)
				}
				data, err := base64.StdEncoding.DecodeString(shot)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("saved-%d-%s.png", width, theme)), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := evaluate(ui, `location.reload();true`, nil); err != nil {
		t.Fatal(err)
	}
	wait("saved letter and discovered question editor", func() bool {
		var ready bool
		return evaluate(ui, `!!document.querySelector('form[action$="/continue"]')&&[...document.querySelectorAll('form[action$="/answers/save"]')].some(f=>f.textContent.includes('Why this team?'))`, &ready) == nil && ready
	})
	const combinedAnswer = "I enjoy resolving difficult incidents with a collaborative support team."
	expectedAction := "Continue application"
	if combined {
		if err := evaluate(ui, `(()=>{const f=[...document.querySelectorAll('form[action$="/answers/save"]')].find(f=>f.textContent.includes('Why this team?'));f.querySelector('textarea').value='`+combinedAnswer+`';f.querySelector('button').click();return true})()`, nil); err != nil {
			t.Fatal(err)
		}
		ui = questionPage("Why this team?")
		wait("new answer and letter share one scoped action", func() bool {
			var ready bool
			return evaluate(ui, `document.querySelector('form[action$="/continue"] button')?.textContent==='Continue application'`, &ready) == nil && ready
		})
		var untouched bool
		if err := evaluate(employer, `document.querySelector('[name=motivation]').value===''`, &untouched); err != nil || !untouched {
			t.Fatal("saving answer transmitted data", err)
		}
		aiPage := questionPage("Describe a difficult support incident you resolved")
		if err := chrome.FixtureCall(ctx, ui, "Page.navigate", map[string]any{"url": aiPage}, nil); err != nil {
			t.Fatal(err)
		}
		ui = aiPage
		wait("narrative question help disclosure", func() bool {
			var ready bool
			return evaluate(ui, `!!document.querySelector('.question-panel:not([hidden]) [data-question-help]')`, &ready) == nil && ready
		})
		if err := evaluate(ui, `document.querySelector('.question-panel:not([hidden]) [data-question-help]').open=true;true`, nil); err != nil {
			t.Fatal(err)
		}
		wait("supported narrative question and AI controls", func() bool {
			var ready bool
			return evaluate(ui, `!!document.querySelector('.question-panel:not([hidden]) [data-answer-ai] [name="provider"]')&&!!document.querySelector('.question-panel:not([hidden]) [data-answer-ai-controls]:not([hidden])')`, &ready) == nil && ready
		})
		var providerModel struct{ Provider, Model string }
		if err := evaluate(ui, `({Provider:document.querySelector('.question-panel:not([hidden]) [data-answer-ai] [name="provider"]').value,Model:document.querySelector('.question-panel:not([hidden]) [data-answer-ai] [name="model"]').value})`, &providerModel); err != nil {
			t.Fatal(err)
		}
		if providerModel.Provider != "ollama" || providerModel.Model == "" {
			t.Fatalf("AI provider/model controls did not initialize: %+v", providerModel)
		}
		if err := evaluate(ui, `(()=>{const ai=document.querySelector('.question-panel:not([hidden]) [data-answer-ai]');ai.open=true;ai.querySelector('[data-answer-ai-another]').open=true;ai.querySelector('[name="career_evidence"]').checked=true;ai.querySelector('[data-answer-ai-review]').click();return true})()`, nil); err != nil {
			t.Fatal(err)
		}
		wait("AI source review completed locally", func() bool {
			var ready bool
			return evaluate(ui, `!document.querySelector('.question-panel:not([hidden]) [data-answer-ai-review-panel]').hidden`, &ready) == nil && ready
		})
		if generatedAnswerCalls.Load() != 0 {
			t.Fatal("AI source review called the injected generator")
		}
		var exactReview struct{ Question, Payload string }
		if err := evaluate(ui, `({Question:document.querySelector('.question-panel:not([hidden])')?.getAttribute('aria-label')||'',Payload:document.querySelector('.question-panel:not([hidden]) [data-answer-ai-payload]').textContent})`, &exactReview); err != nil {
			t.Fatal(err)
		}
		if exactReview.Question != "Describe a difficult support incident you resolved" || !strings.Contains(exactReview.Payload, "Synthetic support incident") || !strings.Contains(exactReview.Payload, "A production customer workflow was interrupted") {
			t.Fatalf("AI review omitted the selected factual source: %+v", exactReview)
		}
		if err := evaluate(ui, `(()=>{const ai=document.querySelector('.question-panel:not([hidden]) [data-answer-ai]');ai.querySelector('[data-answer-ai-authorize]').checked=true;ai.querySelector('[data-answer-ai-external]').checked=true;ai.querySelector('[data-answer-ai-generate]').click();return true})()`, nil); err != nil {
			t.Fatal(err)
		}
		wait("fake AI preview completed", func() bool {
			var ready bool
			return evaluate(ui, `!document.querySelector('.question-panel:not([hidden]) [data-answer-ai-preview]').hidden&&document.querySelector('.question-panel:not([hidden]) [data-answer-ai-text]').value.includes('incident timeline')`, &ready) == nil && ready
		})
		if generatedAnswerCalls.Load() != 1 {
			t.Fatalf("explicitly authorized AI request count=%d", generatedAnswerCalls.Load())
		}
		if err := evaluate(ui, `document.querySelector('.question-panel:not([hidden]) [data-answer-ai-adopt]').click();true`, nil); err != nil {
			t.Fatal(err)
		}
		wait("explicit adoption into unsaved editor", func() bool {
			var ready bool
			return evaluate(ui, `document.querySelector('.question-panel:not([hidden]) .application-question-form textarea[name="answer"]').value==='`+generatedIncidentAnswer+`'&&document.querySelector('.question-panel:not([hidden]) .application-question-form').dataset.dirty==='true'`, &ready) == nil && ready
		})
		var savedCount int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=1 AND answer=?`, generatedIncidentAnswer).Scan(&savedCount); err != nil || savedCount != 0 {
			t.Fatalf("AI adoption saved without the local Save action: count=%d err=%v", savedCount, err)
		}
		if err := evaluate(ui, `document.querySelector('.question-panel:not([hidden]) .application-question-form button[type="submit"]').click();true`, nil); err != nil {
			t.Fatal(err)
		}
		wait("adopted answer saved locally", func() bool {
			var ready bool
			return evaluate(ui, `document.querySelector('.question-panel:not([hidden]) .application-question-form [name="revision"]').value==='1'&&document.querySelector('.question-panel:not([hidden]) .application-question-form textarea[name="answer"]').value==='`+generatedIncidentAnswer+`'`, &ready) == nil && ready
		})
		var savedAIAnswer string
		var savedAIRevision int64
		if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=1 AND label=?`, "Describe a difficult support incident you resolved").Scan(&savedAIAnswer, &savedAIRevision); err != nil || savedAIAnswer != generatedIncidentAnswer || savedAIRevision != 1 {
			t.Fatalf("adopted answer did not persist at revision 1: answer=%q revision=%d err=%v", savedAIAnswer, savedAIRevision, err)
		}
		whyPage := questionPage("Why this team?")
		if err := chrome.FixtureCall(ctx, ui, "Page.navigate", map[string]any{"url": whyPage}, nil); err != nil {
			t.Fatal(err)
		}
		ui = whyPage
		expectedAction = "Continue application"
		wait("saved answer and letter action after returning to the first question", func() bool {
			var ready bool
			return evaluate(ui, `document.querySelector('form[action$="/continue"] button')?.textContent==='Continue application'`, &ready) == nil && ready
		})
	}
	var scoped bool
	if err := evaluate(ui, `document.querySelector('form[action$="/continue"] button').textContent==='`+expectedAction+`'&&!document.querySelector('form[action$="/continue"] [name=confirm]')`, &scoped); err != nil || !scoped {
		t.Fatal("missing exact letter action", err)
	}
	var noSilentTransfer bool
	if err := evaluate(employer, `document.querySelector('[name=cover]').files.length===0&&contactEvents.input===5`, &noSilentTransfer); err != nil || !noSilentTransfer {
		t.Fatal("save transferred data", err)
	}
	letterStart := time.Now()
	if err := evaluate(ui, `(()=>{const f=document.querySelector('form[action$="/continue"]');f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	wait("cover letter and any included answer observed", func() bool {
		state := a.browsers.state(1)
		return state.LetterStatus == "complete" && (!combined || state.AnswerStatus == "complete")
	})
	letterTime := time.Since(letterStart)
	var documents struct {
		Resume, Cover, Motivation, Incident string
		Submitted                           int
	}
	if err := evaluate(employer, `({Resume:document.querySelector('[name=resume]').files[0]?.name,Cover:document.querySelector('[name=cover]').files[0]?.name,Motivation:document.querySelector('[name=motivation]').value,Incident:document.querySelector('[name=incident-story]')?.value||'',Submitted:contactEvents.submitted})`, &documents); err != nil {
		t.Fatal(err)
	}
	if documents.Resume != "Casey_Fixture_Resume_Synthetic_contact_qualification.pdf" || documents.Cover != "Casey_Fixture_Cover_Letter_Synthetic_contact_qualification.pdf" || documents.Submitted != 0 {
		t.Fatalf("incorrect continuation: %+v", documents)
	}
	if combined && (documents.Motivation != combinedAnswer || documents.Incident != generatedIncidentAnswer) {
		t.Fatalf("authorized saved answers did not reach the controlled employer form: %+v", documents)
	}
	mu.Lock()
	count = uploads
	mu.Unlock()
	if count != 1 {
		t.Fatal("resume replayed during letter transfer")
	}
	ui = app.URL + "/jobs/1?draft=1"
	wait("return to exact selected letter draft", func() bool {
		var ready bool
		return evaluate(ui, `!!document.querySelector('form[action$="/answers/save"]')`, &ready) == nil && ready
	})
	capture(1280, "questions-letter-1280.png")
	capture(390, "questions-letter-390.png")

	const answer = "I enjoy resolving difficult incidents with a collaborative support team."
	if err := evaluate(ui, `(()=>{const f=[...document.querySelectorAll('form[action$="/answers/save"]')].find(f=>f.textContent.includes('Why this team?'));f.querySelector('textarea').value='`+answer+`';f.querySelector('[name=reusable]').click();f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	ui = questionPage("Why this team?")
	wait("saved question answer on return", func() bool {
		var ready bool
		return evaluate(ui, `[...document.querySelectorAll('form[action$="/answers/save"] textarea')].some(e=>e.value==='`+answer+`')`, &ready) == nil && ready
	})
	var employerAnswer string
	expectedEmployerAnswer := ""
	if combined {
		expectedEmployerAnswer = combinedAnswer
	}
	if err := evaluate(employer, `document.querySelector('[name=motivation]').value`, &employerAnswer); err != nil || employerAnswer != expectedEmployerAnswer {
		t.Fatal("local save changed employer form", err)
	}
	if err := evaluate(ui, `location.href='/jobs';true`, nil); err != nil {
		t.Fatal(err)
	}
	wait("opportunities navigation", func() bool {
		var ready bool
		return evaluate(app.URL+"/jobs", `location.pathname==='/jobs'`, &ready) == nil && ready
	})
	if err := evaluate(app.URL+"/jobs", `location.href='/jobs/1#prepare';true`, nil); err != nil {
		t.Fatal(err)
	}
	ui = app.URL + "/jobs/1"
	wait("answer persisted across leaving Prepare", func() bool {
		var ready bool
		return evaluate(ui, `[...document.querySelectorAll('form[action$="/answers/save"] textarea')].some(e=>e.value==='`+answer+`')`, &ready) == nil && ready
	})

	// D: enter native choices through the same authoritative local editors.
	for label, value := range map[string]string{"Preferred working pattern": "Remote", "Available for occasional travel": "true", "Preferred interview time": "Morning"} {
		labelJSON, _ := json.Marshal(label)
		valueJSON, _ := json.Marshal(value)
		js := `(()=>{const f=[...document.querySelectorAll('form[action$="/answers/save"]')].find(f=>f.textContent.includes(` + string(labelJSON) + `));if(!f)throw Error('missing editor');f.querySelector('select').value=` + string(valueJSON) + `;f.querySelector('button').click();return true})()`
		if err := evaluate(ui, js, nil); err != nil {
			t.Fatal(err)
		}
		ui = questionPage(label)
		wait("saved choice "+label, func() bool {
			var ready bool
			return evaluate(ui, `[...document.querySelectorAll('form[action$="/answers/save"]')].some(f=>f.textContent.includes(`+string(labelJSON)+`)&&f.querySelector('select')?.value===`+string(valueJSON)+`)`, &ready) == nil && ready
		})
	}
	answerStart := time.Now()
	if err := evaluate(ui, `(()=>{const f=document.querySelector('form[action$="/continue"]');f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}

	ui = app.URL + "/jobs/1"
	wait("answer transfer page while working", func() bool {
		var ready bool
		return evaluate(ui, `['waiting','checking'].includes(document.querySelector('[data-answer-status]')?.dataset.answerStatus)`, &ready) == nil && ready
	})
	if err := evaluate(ui, `(()=>{const e=[...document.querySelectorAll('form[action$="/answers/save"] textarea')].find(e=>e.value==='`+answer+`');e.value='Unsaved local draft';e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	wait("answer transfer processing finished", func() bool { return a.browsers.state(1).AnswerStatus == "complete" })
	transferState := a.browsers.state(1)
	t.Logf("observed per-field answer outcomes: %+v", transferState.AnswerOutcomes)

	answerTime := time.Since(answerStart)
	wait("status refresh preserved unsaved edits", func() bool {
		var ready bool
		return evaluate(ui, `!!document.querySelector('[data-new-questions]')&&[...document.querySelectorAll('form[action$="/answers/save"] textarea')].some(e=>e.value==='Unsaved local draft')`, &ready) == nil && ready
	})
	if err := evaluate(ui, `location.reload();true`, nil); err != nil {
		t.Fatal(err)
	}
	wait("saved answer remains after explicitly discarding unsaved edit", func() bool {
		var ready bool
		return evaluate(ui, `[...document.querySelectorAll('form[action$="/answers/save"] textarea')].some(e=>e.value==='`+answer+`')`, &ready) == nil && ready
	})
	var answered struct {
		Text, Pattern, Radio, Last string
		Travel                     bool
		Submitted                  int
	}
	if err := evaluate(employer, `({Text:document.querySelector('[name=motivation]').value,Pattern:document.querySelector('[name=working-pattern]').value,Travel:document.querySelector('[name=travel]').checked,Radio:document.querySelector('[name=interview]:checked')?.value,Last:document.querySelector('[name=last_name]').value,Submitted:contactEvents.submitted})`, &answered); err != nil {
		t.Fatal(err)
	}
	if answered.Text != answer || answered.Pattern != "remote" || !answered.Travel || answered.Radio != "am" || answered.Last != "Manual correction" || answered.Submitted != 0 {
		t.Fatalf("answer transfer incorrect: %+v; outcomes=%+v", answered, a.browsers.state(1).AnswerOutcomes)
	}
	mu.Lock()
	count = uploads
	mu.Unlock()
	if count != 1 {
		t.Fatal("answer transfer repeated resume upload")
	}

	// F: answer a newly exposed conditional question without restarting.
	wait("conditional question captured", func() bool {
		job, _ := a.jobByID(1)
		p, _ := a.candidateProfile()
		prep, e := a.prepareQuestions(job, p)
		if e != nil {
			return false
		}
		for _, q := range prep.Questions {
			if q.Field.ID == "browser:remote-tools" {
				return true
			}
		}
		return false
	})
	if err := evaluate(ui, `location.reload();true`, nil); err != nil {
		t.Fatal(err)
	}
	wait("conditional question editor", func() bool {
		var ready bool
		return evaluate(ui, `[...document.querySelectorAll('form[action$="/answers/save"]')].some(f=>f.textContent.includes('Which remote support tools'))`, &ready) == nil && ready
	})
	if err := evaluate(ui, `(()=>{const f=[...document.querySelectorAll('form[action$="/answers/save"]')].find(f=>f.textContent.includes('Which remote support tools'));f.querySelector('textarea').value='SSH and shared incident notes';f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	ui = questionPage("Which remote support tools have you used?")
	wait("conditional answer saved", func() bool {
		var ready bool
		return evaluate(ui, `[...document.querySelectorAll('form[action$="/answers/save"] textarea')].some(e=>e.value==='SSH and shared incident notes')`, &ready) == nil && ready
	})
	// The employer changes this field's identity after the local save. Recheck
	// the live form and make sure the old answer is preserved for review instead
	// of being sent to the newly identified control.
	if err := evaluate(employer, `(()=>{const e=document.querySelector('[name="remote-tools"]');e.name='remote-tools-v2';return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	if err := evaluate(ui, `document.querySelector('form[action$="/browser/observe"] button').click();true`, nil); err != nil {
		t.Fatal(err)
	}
	ui = app.URL + "/jobs/1"
	wait("changed employer question identity captured", func() bool {
		job, _ := a.jobByID(1)
		profile, _ := a.candidateProfile()
		prep, err := a.prepareQuestions(job, profile)
		if err != nil {
			return false
		}
		changed := false
		previous := false
		for _, question := range prep.Questions {
			if question.Field.ID == "browser:remote-tools-v2" {
				changed = true
			}
		}
		for _, answer := range prep.Previous {
			if answer.Label == "Which remote support tools have you used?" && answer.Value == "SSH and shared incident notes" {
				previous = true
			}
		}
		return changed && previous
	})
	changedQuestionPage := questionPage("Which remote support tools have you used?")
	if err := chrome.FixtureCall(ctx, ui, "Page.navigate", map[string]any{"url": changedQuestionPage}, nil); err != nil {
		t.Fatal(err)
	}
	ui = changedQuestionPage
	wait("old identity answer shown for manual review", func() bool {
		var ready bool
		return evaluate(ui, `document.body.textContent.includes('Earlier saved answers needing review')&&document.body.textContent.includes('SSH and shared incident notes')&&document.querySelector('.question-panel:not([hidden]) textarea[name="answer"]')?.value===''`, &ready) == nil && ready
	})
	const changedIdentityAnswer = "SSH and shared incident notes, reviewed for the changed employer field."
	if err := evaluate(ui, `(()=>{const f=document.querySelector('.question-panel:not([hidden]) form[action$="/answers/save"]');const e=f.querySelector('textarea[name="answer"]');e.value='`+changedIdentityAnswer+`';e.dispatchEvent(new Event('input',{bubbles:true}));f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	ui = changedQuestionPage
	wait("new identity answer saved locally", func() bool {
		var ready bool
		return evaluate(ui, `document.readyState==='complete'&&document.querySelector('.question-panel:not([hidden]) form[action$="/answers/save"] [name="revision"]')?.value==='1'&&document.querySelector('.question-panel:not([hidden]) form[action$="/answers/save"] textarea[name="answer"]')?.value==='`+changedIdentityAnswer+`'`, &ready) == nil && ready
	})
	const employerManualAnswer = "Applicant-entered manually before authorized transfer"
	if err := evaluate(employer, `(()=>{const e=document.querySelector('[name="remote-tools-v2"]');e.value='`+employerManualAnswer+`';e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	nextPrep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	nextState := a.browsers.state(1)
	nextPrep.Questions = pendingAnswers(nextPrep.ReadyAnswers(), nextState.AnswerAttempted, nextState.ContactStatus != "")
	if err := evaluate(ui, `(()=>{const f=document.querySelector('form[action$="/continue"]');f.querySelector('button').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	ui = app.URL + "/jobs/1"
	wait("conditional answer transfer", func() bool {
		s := a.browsers.state(1)
		return s.AnswerStatus == "complete" && s.AnswerReviewHash == nextPrep.ReviewHash()
	})
	var continued struct {
		Answer, Last, Resume, Cover string
		Submitted                   int
	}
	if err := evaluate(employer, `({Answer:document.querySelector('[name=remote-tools-v2]').value,Last:document.querySelector('[name=last_name]').value,Resume:document.querySelector('[name=resume]').files[0]?.name,Cover:document.querySelector('[name=cover]').files[0]?.name,Submitted:contactEvents.submitted})`, &continued); err != nil {
		t.Fatal(err)
	}
	if continued.Answer != employerManualAnswer || continued.Last != "Manual correction" || continued.Resume != "Casey_Fixture_Resume_Synthetic_contact_qualification.pdf" || continued.Cover != "Casey_Fixture_Cover_Letter_Synthetic_contact_qualification.pdf" || continued.Submitted != 0 {
		t.Fatalf("conditional continuation damaged: %+v", continued)
	}
	manualOutcome := false
	for _, outcome := range a.browsers.state(1).AnswerOutcomes {
		if outcome.Label == "Which remote support tools have you used?" && outcome.Status == "manual" {
			manualOutcome = true
		}
	}
	if !manualOutcome {
		t.Fatalf("manual employer value was not reported per field: %+v", a.browsers.state(1).AnswerOutcomes)
	}
	wait("final answer results visible", func() bool {
		var ready bool
		return evaluate(ui, `document.body.textContent.includes('Answer transfer · processing finished')&&document.body.textContent.includes('1 needs manual completion')&&document.body.textContent.includes('Manual completion required in Chromium')&&!!document.querySelector('.observed-results details[open]')`, &ready) == nil && ready
	})
	if dir := os.Getenv("YOURMOVE_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		shot, err := chrome.FixtureScreenshotAt(ctx, ui, 1280, "#observed-results-title", "")
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "answer-outcomes-1280.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	capture(1280, "continued-1280.png")
	capture(390, "continued-390.png")
	if combined {
		t.Log("One Continue application action transferred the exact displayed letter and saved answer; no contact replay, resume reupload, or browser restart")
	}
	t.Logf("D: 4 saved question answers observed (rerendered textarea, native select, checkbox, radio); prior manual contact preserved; %s for answer transfer; no submission or duplicate resume upload", answerTime.Round(time.Millisecond))
	t.Logf("B/C continuation: question entered, marked reusable, and saved in 3 actions; answer survived navigation; local saving did not mutate the employer answer; letter transferred with one scoped continuation action, observed in %s; original resume preserved; zero duplicate resume uploads or submissions", letterTime.Round(time.Millisecond))

	t.Logf("UI journey: 4 actions (select, save, Prepare, Fill application); transfer verified in %s; result visible in %s; 4 correct contact values, 4 input + 4 change events, 1 exact resume upload, 0 duplicate uploads, 0 unexpected overwrites, 0 submissions; manual correction preserved", transferTime.Round(time.Millisecond), uiTime.Round(time.Millisecond))

	// E: a future opportunity at the same employer exposes the exact provider question.
	nextURL := strings.Replace(testAshbyPosting, "e1f2d6a2-b9e2-4f16-b863-3ca228c83e75", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 1)
	if err := a.saveJob(Job{URL: nextURL, Host: "jobs.ashbyhq.com", Title: "Second synthetic opportunity"}); err != nil {
		t.Fatal(err)
	}
	observed, err := fixtureSession.Observe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed.URL = nextURL + "/application"
	nextResult, err := browser.AsResult(nextURL, observed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(2, nextResult); err != nil {
		t.Fatal(err)
	}
	var beforeNext struct {
		URL        string
		DirtyForms []string
	}
	if err := evaluate(ui, `({URL:location.href,DirtyForms:[...document.querySelectorAll('form[data-dirty="true"]')].map(f=>f.action)})`, &beforeNext); err != nil {
		t.Fatal(err)
	}
	if len(beforeNext.DirtyForms) > 0 {
		t.Fatalf("a saved answer or letter remained dirty before changing opportunities: %+v", beforeNext)
	}
	var nextNavigation struct {
		FrameID, LoaderID, ErrorText string
	}
	if err := chrome.FixtureCall(ctx, ui, "Page.navigate", map[string]any{"url": app.URL + "/jobs/2#prepare"}, &nextNavigation); err != nil {
		t.Fatal(err)
	}
	t.Logf("next opportunity browser navigation: %+v", nextNavigation)
	nextUI := app.URL + "/jobs/2"
	deadline := time.Now().Add(12 * time.Second)
	var lastNavigationErr error
	var reusableVisible bool
	for time.Now().Before(deadline) {
		var ready bool
		lastNavigationErr = evaluate(nextUI, `[...document.querySelectorAll('form[action$="/answers/save"]')].some(f=>f.textContent.includes('Why this team?')&&f.textContent.includes('Answer available locally; transfer requires your authorization')&&f.querySelector('textarea')?.value==='`+answer+`')`, &ready)
		if lastNavigationErr == nil && ready {
			reusableVisible = true
			break
		}
		time.Sleep(60 * time.Millisecond)
	}
	if !reusableVisible {
		response, requestErr := http.Get(nextUI)
		status := ""
		responseText := ""
		if requestErr == nil {
			status = response.Status
			body, _ := io.ReadAll(response.Body)
			responseText = string(body)
			response.Body.Close()
		}
		job, _ := a.jobByID(2)
		profile, _ := a.candidateProfile()
		prepared, prepareErr := a.prepareQuestions(job, profile)
		var answerState []string
		for _, question := range prepared.Questions {
			if strings.Contains(question.Field.Label, "Why this team") {
				answerState = append(answerState, fmt.Sprintf("source=%q value=%q reusable=%t suggestions=%d", question.Source, question.Value, question.ReusableKey != "", len(question.LibrarySuggestions)))
			}
		}
		t.Fatalf("qualified reusable answer not visible without retyping: browser navigation=%+v browser error=%v direct GET=%s (%v) prepared=%v prepare err=%v body has answer=%t reusable-copy=%t", nextNavigation, lastNavigationErr, status, requestErr, answerState, prepareErr, strings.Contains(responseText, answer), strings.Contains(responseText, "Reusable answer approved"))
	}
	t.Log("E: optional reuse selected in the authoritative editor; second saved opportunity displayed the exact qualified answer without retyping; different-employer and local-override boundaries covered in Go storage tests")

	liveObservation, err := chrome.Observe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var motivation, pattern browser.AnswerValue
	for _, c := range liveObservation.Controls {
		field := schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required, Sensitive: c.Sensitive, Options: c.Options, Group: "rendered form"}
		if c.ID == "motivation" {
			motivation = browser.AnswerValue{Field: field, Value: answer}
		}
		if c.ID == "working-pattern" {
			pattern = browser.AnswerValue{Field: field, Value: "Remote"}
		}
	}
	if err := evaluate(employer, `document.querySelector('[name=working-pattern]').options[1].text='Hybrid';document.querySelector('[name=motivation]').value='Authoritative manual narrative';true`, nil); err != nil {
		t.Fatal(err)
	}
	changed, err := chrome.FillAnswer(ctx, pattern)
	if err != nil || changed.Status != "manual" {
		t.Fatalf("changed options accepted: %+v %v", changed, err)
	}
	manual, err := chrome.FillAnswer(ctx, motivation)
	if err != nil || manual.Status != "manual" {
		t.Fatalf("manual narrative overwritten: %+v %v", manual, err)
	}
	if err := evaluate(employer, `document.querySelector('[name=motivation]').value='';const original=document.querySelector('[name=motivation]');original.after(original.cloneNode());true`, nil); err != nil {
		t.Fatal(err)
	}
	ambiguousAnswer, err := chrome.FillAnswer(ctx, motivation)
	if err != nil || ambiguousAnswer.Status != "manual" {
		t.Fatalf("ambiguous answer destination accepted: %+v %v", ambiguousAnswer, err)
	}
	var noAnswerWrites bool
	if err := evaluate(employer, `[...document.querySelectorAll('[name=motivation]')].every(e=>e.value==='')&&document.querySelector('[name=working-pattern]').value==='remote'`, &noAnswerWrites); err != nil || !noAnswerWrites {
		t.Fatal("abstention changed browser values", err)
	}
	t.Log("D negative cases: changed native options, existing manual narrative, and ambiguous question controls preserved without writes")
	// Qualify concrete abstention behavior against live controls, independently of the host's no-replay rule.
	approved := browser.ContactValues{FirstName: p.FirstName, LastName: p.LastName, Email: p.Email, Phone: p.Phone}
	if err := evaluate(employer, `document.querySelector('form').innerHTML='<label>Email<input name="email"></label><label>Email<input name="email"></label><label>First name<input name="unidentified-0"></label>';true`, nil); err != nil {
		t.Fatal(err)
	}
	result, err := chrome.FillContacts(ctx, approved)
	if err != nil {
		t.Fatal(err)
	}
	ambiguous := false
	for _, field := range result.Fields {
		if field.Key == "email" && field.Status == "ambiguous" {
			ambiguous = true
		}
	}
	var untouched bool
	_ = evaluate(employer, `[...document.querySelectorAll('input')].every(e=>e.value==='')`, &untouched)
	if !ambiguous || !untouched {
		t.Fatalf("ambiguous or positional control filled: %+v", result)
	}
	if err := evaluate(employer, `document.querySelector('form').innerHTML='<label>Name<input name="_systemfield_name" value="User correction"></label><label>Email<input name="_systemfield_email" type="email" value="manual@example.test"></label><label>Phone<input name="_systemfield_phone" type="tel" value="202-555-0104"></label>';true`, nil); err != nil {
		t.Fatal(err)
	}
	result, err = chrome.FillContacts(ctx, approved)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fields) != 3 || result.Fields[0].Status != "manual" || result.Fields[1].Status != "manual" || result.Fields[2].Status != "correct" {
		t.Fatalf("existing user values not preserved: %+v", result)
	}

	if err := evaluate(employer, `document.querySelectorAll('input').forEach(e=>e.value='');true`, nil); err != nil {
		t.Fatal(err)
	}
	result, err = chrome.FillContacts(ctx, approved)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fields) != 3 {
		t.Fatalf("full-name layout: %+v", result)
	}
	for _, field := range result.Fields {
		if field.Status != "filled" {
			t.Fatalf("full-name transfer: %+v", result)
		}
	}
	var fullName string
	if err := evaluate(employer, `document.querySelector('[name=_systemfield_name]').value`, &fullName); err != nil || fullName != "Casey Fixture" {
		t.Fatalf("full name=%q err=%v", fullName, err)
	}
	// Real Ashby uses a scoped div container and a custom phone ID.
	if err := evaluate(employer, `document.querySelector('form').outerHTML='<div class="ashby-application-form-container"><label>Full Legal Name<input name="ff5bb721-1230-4753-bb72-2f8324c0bf26__systemfield_name"></label><label>Email<input name="ff5bb721-1230-4753-bb72-2f8324c0bf26__systemfield_email" type="email"></label><label>Phone Number<input id="ff5bb721-1230-4753-bb72-2f8324c0bf26_106ba651-5f51-4048-af0a-ef3195a66c55" type="tel"></label></div>';true`, nil); err != nil {
		t.Fatal(err)
	}
	result, err = chrome.FillContacts(ctx, approved)
	if err != nil || !result.Ready || len(result.Fields) != 3 {
		t.Fatalf("Ashby div layout: %+v %v", result, err)
	}
	for _, field := range result.Fields {
		if field.Status != "filled" {
			t.Fatalf("Ashby div transfer: %+v", result)
		}
	}
	if err := evaluate(employer, `document.querySelector('.ashby-application-form-container').outerHTML='<form class="ashby-application-form"></form>';true`, nil); err != nil {
		t.Fatal(err)
	}
	if err := evaluate(employer, `document.querySelector('form').innerHTML='<label>Email<input name="email"></label>';document.querySelector('[name=email]').addEventListener('input',e=>{const old=e.target;setTimeout(()=>{const fresh=old.cloneNode();fresh.value='';old.replaceWith(fresh)},50)});true`, nil); err != nil {
		t.Fatal(err)
	}
	result, err = chrome.FillContacts(ctx, approved)
	if err != nil {
		t.Fatal(err)
	}
	uncertain := false
	for _, field := range result.Fields {
		if field.Key == "email" && field.Status == "uncertain" {
			uncertain = true
		}
	}
	var blank bool
	_ = evaluate(employer, `document.querySelector('[name=email]').value===''`, &blank)
	if !uncertain || !blank {
		t.Fatalf("rerender loss falsely verified or replayed: %+v", result)
	}
	if err := evaluate(employer, `document.querySelector('form').innerHTML='<label>Work authorization<input name="email" autocomplete="email"></label>';true`, nil); err != nil {
		t.Fatal(err)
	}
	result, err = chrome.FillContacts(ctx, approved)
	if err != nil {
		t.Fatal(err)
	}
	_ = evaluate(employer, `document.querySelector('[name=email]').value===''`, &blank)
	if !blank {
		t.Fatal("contact data pasted into a sensitive question")
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<form class="ashby-application-form"><input name="email"></form>`)
	}))
	defer other.Close()
	if err := chrome.FixtureCall(ctx, employer, "Page.navigate", map[string]any{"url": other.URL}, nil); err != nil {
		t.Fatal(err)
	}
	wait("foreign navigation", func() bool {
		var current string
		return evaluate(other.URL+"/", "location.origin", &current) == nil && current == other.URL
	})
	if _, err := chrome.FillContacts(ctx, approved); err == nil {
		t.Fatal("contact transfer allowed a foreign application")
	}
	t.Log("Additional live-CDP cases passed: ambiguous controls, positional-ID abstention, conflicting user values, already-correct value, Ashby full-name controls, rerender loss, sensitive-label abstention, foreign session ownership")

	if combined {
		// Finish the same synthetic opportunity in the rendered YourMove UI.
		// The employer form's submit handler prevents external submission; this
		// records only the applicant's explicit, simulated confirmation.
		// YourMove's application window and managed employer browser are
		// separate Chromium processes in normal desktop operation. Keep that
		// boundary here because Next job deliberately closes the employer one.
		appUI, err := chromium.LaunchFixture(ctx, app.URL+"/jobs/1#prepare")
		if err != nil {
			t.Fatal(err)
		}
		defer appUI.Close()
		appPage := app.URL + "/jobs/1"
		evaluateAppUI := func(page, expression string, out any) error {
			var reply struct {
				Result struct {
					Value json.RawMessage `json:"value"`
				} `json:"result"`
				Exception json.RawMessage `json:"exceptionDetails"`
			}
			if err := appUI.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &reply); err != nil {
				return err
			}
			if len(reply.Exception) > 0 {
				return fmt.Errorf("YourMove UI JavaScript: %s", reply.Exception)
			}
			if out != nil {
				return json.Unmarshal(reply.Result.Value, out)
			}
			return nil
		}
		waitAppUI := func(description, page, expression string) {
			t.Helper()
			deadline := time.Now().Add(12 * time.Second)
			for time.Now().Before(deadline) {
				var ready bool
				if evaluateAppUI(page, expression, &ready) == nil && ready {
					return
				}
				time.Sleep(60 * time.Millisecond)
			}
			var diagnostic string
			_ = evaluateAppUI(page, `JSON.stringify({url:location.href,body:document.body.innerText.slice(0,500)})`, &diagnostic)
			t.Fatalf("timed out: %s; YourMove UI=%s", description, diagnostic)
		}
		waitAppUI("recording action after the controlled application journey", appPage, `document.readyState==='complete'&&!!document.querySelector('[data-record-completion]')&&!!document.querySelector('[data-record-completion] [name="submission_token"]')`)
		const completionNote = "Synthetic applicant-confirmed submission"
		if err := evaluateAppUI(appPage, `(()=>{const d=document.querySelector('#record-submission');d.open=true;const n=d.querySelector('[name="notes"]');n.focus();n.value=`+strconv.Quote(completionNote)+`;n.dispatchEvent(new Event('input',{bubbles:true}));const c=d.querySelector('[name="confirm"]');c.click();return d.open&&c.checked})()`, nil); err != nil {
			t.Fatal(err)
		}
		if err := evaluateAppUI(appPage, `document.querySelector('[data-record-completion] button[type="submit"]').click();true`, nil); err != nil {
			t.Fatal(err)
		}
		waitAppUI("completion confirmation and persisted History rendered", appPage, `document.body.textContent.includes('Application recorded as Applied.')&&document.querySelector('#history')?.textContent.includes('Synthetic applicant-confirmed submission')`)
		job, err := a.jobByID(1)
		if err != nil || job.Stage != StageApplied {
			t.Fatalf("integrated application did not reach Applied: %+v %v", job, err)
		}
		history, err := a.jobSubmissions(1)
		if err != nil || len(history) != 1 || history[0].ResumeID != resume.ID || history[0].Notes != completionNote {
			t.Fatalf("integrated completion record mismatch: %+v %v", history, err)
		}
		var historyResume bool
		if err := evaluateAppUI(appPage, `!!document.querySelector('#history form[action="/jobs/1/next"] button')&&document.querySelector('#history').textContent.includes('approved-resume.pdf')`, &historyResume); err != nil || !historyResume {
			t.Fatalf("History did not show the resume association and next action: %v", err)
		}
		if dir := os.Getenv("YOURMOVE_EVIDENCE_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			shot, err := appUI.FixtureScreenshotAt(ctx, appPage, 1280, "#history", "")
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(shot)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "integrated-applied-history.png"), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := evaluateAppUI(appPage, `document.querySelector('#history form[action="/jobs/1/next"] button').click();true`, nil); err != nil {
			t.Fatal(err)
		}
		nextPage := app.URL + "/jobs/2"
		waitAppUI("next opportunity after integrated completion", nextPage, `location.pathname==='/jobs/2'&&document.readyState==='complete'`)
		history, err = a.jobSubmissions(1)
		if err != nil || len(history) != 1 {
			t.Fatalf("advancing lost or duplicated the completed application: %+v %v", history, err)
		}
		t.Log("Integrated Chromium journey passed: AI preview → explicit adoption → local save → authorized employer fill → rendered Applied/History → Next job; one persisted submission, no employer submit")
	}
}

func mustQualificationPDF(t *testing.T) []byte {
	t.Helper()
	data, err := coverLetterPDF("Verified technical support experience and production troubleshooting.")
	if err != nil {
		t.Fatal(err)
	}
	return data
}
