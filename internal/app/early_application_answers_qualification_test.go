//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

type greenhouseFixtureSession struct {
	*chromium.Session
	posting string
}

func (s *greenhouseFixtureSession) Observe(ctx context.Context) (browser.Observation, error) {
	observation, err := s.Session.Observe(ctx)
	if err == nil {
		observation.URL = s.posting
	}
	return observation, err
}

func TestControlledEarlyPreparationWorkbench(t *testing.T) {
	if testing.Short() || os.Getenv("YOURMOVE_QUALIFY_EARLY") != "1" {
		t.Skip("opt-in isolated Chromium early-preparation qualification")
	}
	a, workspaceDir := testWorkspace(t)
	job, result := earlyGreenhouseFixture(t, a)
	sourceJob := Job{URL: "https://job-boards.greenhouse.io/synthetic/jobs/67890", Host: "job-boards.greenhouse.io", Title: "Previous Synthetic Engineer", Company: "Previous Fixture Co"}
	if err := a.saveJob(sourceJob); err != nil {
		t.Fatal(err)
	}
	sourceJob, err := a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.saveInspection(sourceJob.ID, schema.Result{Platform: "greenhouse", Source: "browser", ApplicationURL: sourceJob.URL, Fields: []schema.Field{result.Fields[0]}}); err != nil {
		t.Fatal(err)
	}
	sourceKey := questionKey(sourceJob.URL, "greenhouse", result.Fields[0])
	previousAnswer := "I improved a synthetic system by reviewing its query plan and recommending an index."
	saveLibrarySourceAnswer(t, a, sourceJob, sourceKey, previousAnswer, 0)
	if promoted := promoteLibrarySource(t, a, sourceJob.ID, sourceKey, 1); promoted.Code != http.StatusSeeOther {
		t.Fatalf("promote early starting-point fixture: %d %s", promoted.Code, promoted.Body.String())
	}
	addCareerEvidenceForSearch(t, a, careerEvidenceContent{Title: "Synthetic performance improvement", Situation: "A dashboard query slowed down.", Actions: "I reviewed the query plan and recommended an index.", OutcomeState: "unknown"}, true)
	var inspectionCalls, generationCalls atomic.Int32
	var invalidProviderPrompt atomic.Bool
	a.inspectApplication = func(_ context.Context, raw string) (schema.Result, error) {
		inspectionCalls.Add(1)
		if raw != job.URL {
			return schema.Result{}, fmt.Errorf("discovery used a non-saved URL: %s", raw)
		}
		return result, nil
	}
	a.applicationAnswerGenerate = func(_ context.Context, prompt, _ string) (string, error) {
		generationCalls.Add(1)
		if !strings.Contains(prompt, "not yet verified on the employer form") {
			invalidProviderPrompt.Store(true)
		}
		return "I reviewed the query plan and recommended an index.", nil
	}
	var appMu sync.RWMutex
	activeApp := a
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appMu.RLock()
		app := activeApp
		appMu.RUnlock()
		newMux(app).ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	page := server.URL + "/jobs/1#prepare"
	evidenceRoot, err := os.MkdirTemp("/private/tmp", "yourmove-checkpoint2-greenhouse-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("isolated Greenhouse question-workbench screenshots: %s", evidenceRoot)
	chrome, err := chromium.LaunchFixture(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	evaluate := func(script string, out any) error {
		var response struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception any `json:"exceptionDetails"`
		}
		if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true, "awaitPromise": true}, &response); err != nil {
			return err
		}
		if response.Exception != nil {
			return fmt.Errorf("browser exception: %v", response.Exception)
		}
		if out != nil {
			return json.Unmarshal(response.Result.Value, out)
		}
		return nil
	}
	run := func(script string) {
		t.Helper()
		if err := evaluate(script, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait := func(script string) {
		t.Helper()
		for end := time.Now().Add(12 * time.Second); time.Now().Before(end); {
			var ok bool
			if evaluate("Boolean("+script+")", &ok) == nil && ok {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		var diagnostic string
		_ = evaluate(`JSON.stringify({url:location.href,ready:document.readyState,body:document.body.innerText.slice(-1800)})`, &diagnostic)
		t.Fatalf("browser condition failed: %s\n%s", script, diagnostic)
	}
	assert := func(script string) {
		t.Helper()
		var ok bool
		if err := evaluate("Boolean("+script+")", &ok); err != nil || !ok {
			t.Fatalf("browser assertion failed: %s (%v)", script, err)
		}
	}
	capture := func(name string, width int, selector, theme string) {
		t.Helper()
		shot, err := chrome.FixtureScreenshotAt(ctx, page, width, selector, theme)
		if err != nil {
			t.Fatalf("capture %s: %v", name, err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidenceRoot, name+".png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	wait(`document.readyState==='complete'&&!!document.querySelector('.early-application-questions form[action$="/inspect"] button')`)
	assert(`document.querySelector('[data-question-workbench]')===null||document.querySelectorAll('[data-question-workbench] [data-question-panel]').length===0`)
	run(`document.querySelector('.early-application-questions form[action$="/inspect"] button').click()`)
	wait(`document.readyState==='complete'&&!!document.querySelector('.early-application-questions [data-question-help]')`)
	capture("early-initial-1280-light", 1280, ".early-application-questions", "light")
	if inspectionCalls.Load() != 1 {
		t.Fatalf("expected one explicitly requested public inspection, got %d", inspectionCalls.Load())
	}
	run(`document.querySelector('.early-application-questions [data-question-help]').open=true;window.editor=document.querySelector('.early-application-questions textarea[name=answer]')`)
	wait(`!!document.querySelector('.early-application-questions [data-answer-ai]')`)
	run(`document.querySelector('.early-application-questions .help-tool:first-child > summary').click()`)
	wait(`!!document.querySelector('.early-application-questions [data-related-answer]')`)
	run(`document.querySelector('.early-application-questions [data-related-answer] > summary').click()`)
	wait(`document.querySelector('.early-application-questions [data-related-answer]').dataset.loaded==='true'`)
	assert(`document.querySelector('.early-application-questions [data-related-content]').textContent.includes(` + fmt.Sprintf("%q", previousAnswer) + `)`)
	assert(`document.querySelector('.early-application-questions [data-use-library-answer]').hidden===false`)
	run(`window.confirm=()=>false;editor.value='keep this early text';editor.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('.early-application-questions [data-use-library-answer]').click()`)
	assert(`editor.value==='keep this early text'`)
	run(`window.confirm=()=>true;document.querySelector('.early-application-questions [data-use-library-answer]').click()`)
	wait(`editor.value===` + fmt.Sprintf("%q", previousAnswer))
	var unsavedEarlyCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers WHERE job_id=?`, job.ID).Scan(&unsavedEarlyCount); err != nil || unsavedEarlyCount != 0 {
		t.Fatalf("using previous writing persisted the early answer: %d %v", unsavedEarlyCount, err)
	}
	run(`editor.value='';editor.dispatchEvent(new Event('input',{bubbles:true}))`)
	run(`document.querySelectorAll('.early-application-questions .help-tool')[1].open=true`)
	capture("early-career-evidence-1280-dark", 1280, ".early-application-questions", "dark")
	run(`document.querySelectorAll('.early-application-questions .help-tool')[1].open=false`)
	run(`window.ai=document.querySelector('.early-application-questions [data-answer-ai]');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']');window.editor=document.querySelector('.early-application-questions textarea[name=answer]');ai.open=true`)
	assert(`ai.dataset.early==='yes'&&editor.value===''&&ai.querySelector('[name=career_evidence]')&&!ai.querySelector('[name=career_evidence]').checked`)
	run(`find('another').open=true`)
	capture("early-ai-source-selection-390-light", 390, "[data-answer-ai-selection]", "light")
	run(`ai.querySelector('[name=career_evidence]').checked=true;ai.querySelector('[name=model]').value='synthetic-model';find('review').click()`)
	wait(`!find('review-panel').hidden`)
	capture("early-ai-review-1280-light", 1280, "[data-answer-ai-review-panel]", "light")
	assert(`find('execution').textContent.includes('Published question')`)
	assert(`find('authorize')&&!find('authorize').checked&&find('external')&&!find('external').checked`)
	run(`find('authorize').checked=true;find('external').checked=true;find('generate').click()`)
	wait(`!find('preview').hidden&&find('text').value.includes('query plan')`)
	capture("early-generated-390-dark", 390, "[data-answer-ai-preview]", "dark")
	if generationCalls.Load() != 1 {
		t.Fatalf("expected one explicitly authorized AI request, got %d", generationCalls.Load())
	}
	if invalidProviderPrompt.Load() {
		t.Fatal("early question was not identified as unverified in the model context")
	}
	var earlyCount, liveCount, browserCount, submissionCount int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&earlyCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=?`, job.ID).Scan(&liveCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_inspections WHERE job_id=? AND source='browser'`, job.ID).Scan(&browserCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=?`, job.ID).Scan(&submissionCount)
	if earlyCount != 0 || liveCount != 0 || browserCount != 0 || submissionCount != 0 {
		t.Fatalf("AI preview or public discovery crossed a live boundary: early=%d live=%d browser=%d submissions=%d", earlyCount, liveCount, browserCount, submissionCount)
	}
	run(`find('adopt').click()`)
	wait(`editor.value.includes('query plan')&&find('status').textContent.includes('Save early answer locally')`)
	run(`document.querySelector('.early-application-questions [data-question-help]').open=true`)
	wait(`!!document.querySelector('.early-application-questions [data-answer-ai]')`)
	assert(`document.querySelector('.early-application-questions [data-question-panel]:not([hidden]) [data-answer-ai]')`)
	run(`editor.value+=' I documented the recommendation.';editor.dispatchEvent(new Event('input',{bubbles:true}));editor.closest('form').querySelector('button[type=submit]').click()`)
	wait(`document.readyState==='complete'&&document.querySelector('.early-application-questions textarea[name=answer]')?.value.includes('documented the recommendation')`)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&earlyCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=?`, job.ID).Scan(&liveCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_inspections WHERE job_id=? AND source='browser'`, job.ID).Scan(&browserCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=?`, job.ID).Scan(&submissionCount)
	if earlyCount != 1 || liveCount != 0 || browserCount != 0 || submissionCount != 0 {
		t.Fatalf("saved early proposal crossed a live boundary: early=%d live=%d browser=%d submissions=%d", earlyCount, liveCount, browserCount, submissionCount)
	}
	if err := chrome.Close(); err != nil {
		t.Fatal(err)
	}

	// Restart YourMove between preparation and the employer application.
	oldDB := a.db
	a.Close()
	if err := oldDB.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(workspaceDir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	a = newTestApp(t, db, filepath.Join(workspaceDir, "resumes"))
	appMu.Lock()
	activeApp = a
	appMu.Unlock()
	recovered, err := a.prepareEarlyQuestions(job, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if err != nil || len(recovered.Questions) == 0 || recovered.Questions[0].Revision != 1 || !strings.Contains(recovered.Questions[0].Value, "documented the recommendation") {
		t.Fatalf("restart lost the prepared answer: %+v err=%v", recovered.Questions, err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/next-synthetic-opportunity", Host: "example.com", Title: "Next Synthetic Opportunity"}); err != nil {
		t.Fatal(err)
	}

	var employerSubmissions atomic.Int32
	employer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			employerSubmissions.Add(1)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/synthetic/application" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><head><title>Controlled Greenhouse application</title></head><body>
<h1>Controlled employer application</h1><form method="post" action="/synthetic/submission">
<label for="story">Describe a time you improved a system<textarea id="story" name="story" required></textarea></label>
<label>Resume <input id="resume" name="resume" type="file" accept=".pdf"></label>
<button type="button">Continue</button><button type="submit">Submit application</button>
</form></body></html>`)
	}))
	defer employer.Close()
	var managed *greenhouseFixtureSession
	a.browsers.pollEvery = 100 * time.Millisecond
	a.browsers.factory = func(browserCtx context.Context, destination string) (readOnlySession, error) {
		if destination != job.URL {
			return nil, fmt.Errorf("managed application opened a changed destination: %s", destination)
		}
		session, launchErr := chromium.LaunchFixture(browserCtx, employer.URL+"/synthetic")
		if launchErr != nil {
			return nil, launchErr
		}
		managed = &greenhouseFixtureSession{Session: session, posting: job.URL}
		return managed, nil
	}
	started := postApp(newMux(a), "/jobs/1/browser/start", url.Values{}, "", "")
	if started.Code != http.StatusSeeOther || managed == nil {
		t.Fatalf("open controlled employer application: %d %s", started.Code, started.Body.String())
	}
	var liveInspection Inspection
	for end := time.Now().Add(12 * time.Second); time.Now().Before(end); {
		liveInspection, err = a.latestInspectionBySource(job.ID, "browser")
		if err == nil && len(liveInspection.Fields) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil || liveInspection.Source != "browser" || liveInspection.Fields[0].ID != "browser:story" {
		t.Fatalf("controlled live form was not observed: %+v %v", liveInspection, err)
	}
	preparedEarly := earlyQuestionByID(t, a, job, "application:story", "")
	if preparedEarly.Reconciliation.Status != "Ready for explicit review" || !preparedEarly.Reconciliation.CanAccept {
		t.Fatalf("exact live question was not ready for review: %+v", preparedEarly.Reconciliation)
	}
	var liveValue string
	readEmployerValue := func() string {
		var response struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		if err := managed.FixtureCall(ctx, employer.URL+"/synthetic/application", "Runtime.evaluate", map[string]any{
			"expression": `document.querySelector('#story').value`, "returnByValue": true, "awaitPromise": true,
		}, &response); err != nil {
			t.Fatal(err)
		}
		return response.Result.Value
	}
	if liveValue = readEmployerValue(); liveValue != "" {
		t.Fatalf("prepared text reached the employer before explicit acceptance and transfer: %q", liveValue)
	}
	var applicationAnswerCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=?`, job.ID).Scan(&applicationAnswerCount); err != nil || applicationAnswerCount != 0 {
		t.Fatalf("public draft bypassed live acceptance: count=%d err=%v", applicationAnswerCount, err)
	}
	acceptValues := url.Values{
		"early_answer_id":             {fmt.Sprint(preparedEarly.EarlyAnswerID)},
		"early_revision":              {fmt.Sprint(preparedEarly.Revision)},
		"destination":                 {job.URL},
		"live_inspection_fingerprint": {preparedEarly.Reconciliation.LiveInspectionFingerprint},
		"live_question_key":           {preparedEarly.Reconciliation.LiveQuestionKey},
		"live_answer_revision":        {fmt.Sprint(preparedEarly.Reconciliation.LiveAnswerRevision)},
	}
	preparedPage := httptest.NewRecorder()
	newMux(a).ServeHTTP(preparedPage, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	for _, expected := range []string{"Current employer question", "Use prepared answer for this question", preparedEarly.Field.Label} {
		if !strings.Contains(preparedPage.Body.String(), expected) {
			t.Fatalf("Prepare did not present the reviewed mapping: missing %q", expected)
		}
	}
	accepted := postApp(newMux(a), "/jobs/1/answers/early-accept", acceptValues, "", "")
	if accepted.Code != http.StatusSeeOther || liveValue != "" {
		t.Fatalf("explicit acceptance unexpectedly filled employer field: %d %s", accepted.Code, accepted.Body.String())
	}
	profile, _ := a.candidateProfile()
	liveQuestions, err := a.prepareQuestions(job, profile)
	if err != nil || len(liveQuestions.ReadyAnswers()) != 1 {
		t.Fatalf("accepted answer was not saved under live validation: %+v %v", liveQuestions.Questions, err)
	}
	transferValues := url.Values{"destination": {job.URL}, "include_answers": {"yes"}, "review_hash": {liveQuestions.ReviewHash()}}
	transfer := postApp(newMux(a), "/jobs/1/continue", transferValues, "", "")
	if transfer.Code != http.StatusSeeOther {
		t.Fatalf("explicit transfer authorization failed: %d %s", transfer.Code, transfer.Body.String())
	}
	var outcomes []browser.AnswerOutcome
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end); {
		state := a.browsers.state(job.ID)
		if state.AnswerStatus == "complete" {
			outcomes = state.AnswerOutcomes
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(outcomes) != 1 || outcomes[0].Status != "filled" || outcomes[0].Label != preparedEarly.Field.Label {
		t.Fatalf("incorrect verified transfer outcome: %+v", outcomes)
	}
	if liveValue = readEmployerValue(); liveValue != preparedEarly.Value {
		t.Fatalf("employer field did not receive the explicitly authorized saved answer: %q", liveValue)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=?`, job.ID).Scan(&submissionCount); err != nil || submissionCount != 0 || employerSubmissions.Load() != 0 {
		t.Fatalf("answer filling submitted the employer application: local=%d employer events=%d err=%v", submissionCount, employerSubmissions.Load(), err)
	}
	resultPage := httptest.NewRecorder()
	newMux(a).ServeHTTP(resultPage, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if !strings.Contains(resultPage.Body.String(), "Filled and verified in Chromium") {
		t.Fatal("Prepare did not show the verified employer-field outcome")
	}
	token := submissionTokenFromPage(t, resultPage.Body.String())
	completed := postApp(newMux(a), "/jobs/1/submit", url.Values{"confirm": {"yes"}, "submission_token": {token}, "notes": {"Applicant submitted the controlled fixture personally"}}, "", "")
	if completed.Code != http.StatusSeeOther || completed.Header().Get("Location") != "/jobs/1#history" {
		t.Fatalf("local completion confirmation: %d %s", completed.Code, completed.Header().Get("Location"))
	}
	completedJob, err := a.jobByID(job.ID)
	if err != nil || completedJob.Stage != StageApplied || completedJob.SubmissionCount != 1 {
		t.Fatalf("local completion was not recorded: %+v %v", completedJob, err)
	}
	historyPage := httptest.NewRecorder()
	newMux(a).ServeHTTP(historyPage, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if !strings.Contains(historyPage.Body.String(), "Application recorded as Applied") || !strings.Contains(historyPage.Body.String(), "Applicant submitted the controlled fixture personally") || !strings.Contains(historyPage.Body.String(), "/jobs/1/next") {
		t.Fatal("Applied history or Next job action missing after local completion")
	}
	applicationsPage := httptest.NewRecorder()
	newMux(a).ServeHTTP(applicationsPage, httptest.NewRequest("GET", "http://localhost/applications", nil))
	if !strings.Contains(applicationsPage.Body.String(), "Synthetic Engineer") {
		t.Fatal("Applied history did not include the controlled Greenhouse opportunity")
	}
	next := postApp(newMux(a), "/jobs/1/next", url.Values{}, "", "")
	if next.Code != http.StatusSeeOther || next.Header().Get("Location") != "/jobs/3#prepare" || a.browsers.has(job.ID) {
		t.Fatalf("Next job transition failed: %d %s active=%v", next.Code, next.Header().Get("Location"), a.browsers.has(job.ID))
	}
	nextPage := httptest.NewRecorder()
	newMux(a).ServeHTTP(nextPage, httptest.NewRequest("GET", "http://localhost/jobs/3", nil))
	if !strings.Contains(nextPage.Body.String(), "Next Synthetic Opportunity") {
		t.Fatal("Next job did not open the next opportunity")
	}
	if employerSubmissions.Load() != 0 {
		t.Fatalf("controlled qualification generated %d employer submission events", employerSubmissions.Load())
	}
}
