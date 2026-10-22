//go:build qualification

package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// This opt-in qualification uses synthetic jobs, temporary storage, and a
// localhost Chromium fixture. The employer URL is stored as identity only.
func TestControlledCareerEvidenceWorkbench(t *testing.T) {
	if testing.Short() || !strings.EqualFold(strings.TrimSpace(os.Getenv("YOURMOVE_QUALIFY_BROWSER")), "1") {
		t.Skip("opt-in isolated Chromium Career Evidence workbench qualification")
	}
	a, dir := testWorkspace(t)
	source := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic Source Engineer", Company: "Fixture Systems"}
	target := Job{URL: "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000002", Host: "jobs.ashbyhq.com", Title: "Synthetic Destination Engineer", Company: "Second Fixture"}
	for _, job := range []Job{source, target} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}
	source, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	target, err = a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:production-story", Label: answerLibraryStory, Kind: "textarea", Required: true}
	for _, job := range []Job{source, target} {
		if _, err := a.saveInspection(job.ID, schema.Result{
			Platform: "ashby", Source: "browser", Completeness: "rendered_partial",
			ApplicationURL: job.URL + "/application", Fields: []schema.Field{field},
		}); err != nil {
			t.Fatal(err)
		}
	}
	sourceKey := questionKey(source.URL, "ashby", field)
	sourceWriting := "I reviewed synthetic slow-query logs and compared MySQL EXPLAIN plans."
	saveLibrarySourceAnswer(t, a, source, sourceKey, sourceWriting, 0)
	if promoted := promoteLibrarySource(t, a, source.ID, sourceKey, 1); promoted.Code != http.StatusSeeOther {
		t.Fatalf("promote synthetic previous writing: %d %s", promoted.Code, promoted.Body.String())
	}
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("load previous writing: %d %v", len(entries), err)
	}
	evidenceID, err := a.createCareerEvidenceFromLibrary(entries[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	evidenceContent := careerEvidenceContent{
		Title:           "Investigating production MySQL performance degradation",
		Situation:       "A customer-facing application had slow MySQL operations.\nThe service remained available while the team investigated.",
		Actions:         "Reviewed synthetic slow-query logs and EXPLAIN plans. Recommended indexing changes.",
		EmployerContext: "Fixture Systems", RoleContext: "Support engineer", Technologies: "Linux, MySQL, SSH",
		OutcomeState: "unknown", SourceNotes: "Synthetic fixture notes retained for inspection.",
	}
	if err := a.updateCareerEvidence(evidenceID, 1, evidenceContent, false); err != nil {
		t.Fatal(err)
	}
	if err := a.updateCareerEvidence(evidenceID, 2, evidenceContent, true); err != nil {
		t.Fatal(err)
	}
	draft := addCareerEvidenceForSearch(t, a, careerEvidenceContent{
		Title: "Production incident response draft", Situation: "A customer-facing application had slow MySQL operations.",
		Actions: "Reviewed synthetic logs and query plans.", OutcomeState: "not_recorded",
	}, false)
	key := questionKey(target.URL, "ashby", field)

	appHandler := newMux(a)
	localPage := httptest.NewRecorder()
	localRequest := httptest.NewRequest(http.MethodGet, "http://localhost/jobs/2", nil)
	localRequest.Host = "localhost"
	appHandler.ServeHTTP(localPage, localRequest)
	if localPage.Code != http.StatusOK || !strings.Contains(localPage.Body.String(), "application-question-form") {
		t.Fatalf("synthetic workbench fixture did not render before Chromium: %d %s", localPage.Code, localPage.Body.String())
	}
	var appScripts atomic.Int32
	router := http.NewServeMux()
	router.HandleFunc("GET /jobs/2/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/2", http.StatusSeeOther)
	})
	for _, script := range []string{"/static/ui.js", "/static/inspection.js"} {
		router.HandleFunc("GET "+script, func(w http.ResponseWriter, r *http.Request) {
			appScripts.Add(1)
			appHandler.ServeHTTP(w, r)
		})
	}
	router.Handle("/", appHandler)
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, server.URL+"/jobs/2")
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()
	page := server.URL + "/jobs/2"
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
		var lastErr error
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			var ready bool
			lastErr = evaluate(script, &ready)
			if lastErr == nil && ready {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		var state string
		_ = evaluate(`JSON.stringify({url:location.href,title:document.title,body:document.body?.innerText?.slice(0,1200),help:document.querySelector('[data-question-help]')?.outerHTML,forms:[...document.forms].map(f=>f.action)})`, &state)
		t.Fatalf("Career Evidence workbench browser condition timed out: %s (%s, %v)", script, state, lastErr)
	}
	assert := func(script string) {
		t.Helper()
		var ok bool
		if err := evaluate(script, &ok); err != nil || !ok {
			t.Fatalf("workbench assertion failed: %s (%v)", script, err)
		}
	}
	wait(`document.readyState==='complete' && !!document.querySelector('.application-question-form')`)
	if appScripts.Load() < 2 {
		t.Fatalf("workbench did not request both optional app scripts: %d", appScripts.Load())
	}
	if err := evaluate(`document.querySelector('[data-question-help] summary').click()`, nil); err != nil {
		t.Fatal(err)
	}
	wait(`document.querySelector('[data-question-help]').dataset.loaded==='true'`)
	assert(`document.querySelectorAll('.related-previous-writing').length===1 && document.querySelectorAll('.related-career-evidence').length===1`)
	assert(`document.querySelector('[data-question-panel]').dataset.questionLabel===` + fmt.Sprintf("%q", field.Label))
	assert(`document.querySelector('.related-career-evidence').textContent.includes("` + evidenceContent.Title + `")`)
	assert(`!document.querySelector('.related-career-evidence').textContent.includes("` + draft.Title + `")`)
	assert(`document.querySelector('.related-previous-writing').textContent.includes('Read the complete answer')`)
	assert(`document.querySelector('#answer-` + key + `').value===''`)
	assert(`!document.querySelector('form[action$="/answers/transfer"]') && !!document.querySelector('form[action$="/submit"] input[name="confirm"][required]:not(:checked)')`)
	if a.browsers.state(target.ID).Active {
		t.Fatal("rendering reference material started an employer browser")
	}
	var count int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=?`, target.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("evidence was stored as an answer before the applicant saved it: count=%d err=%v", count, err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=?`, target.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("reference display recorded a submission: count=%d err=%v", count, err)
	}

	const dirtyAnswer = "My own unsaved answer remains local."
	if err := evaluate(`(()=>{const e=document.querySelector('#answer-`+key+`');e.value=`+fmt.Sprintf("%q", dirtyAnswer)+`;e.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('.career-evidence-full summary').click();return true})()`, nil); err != nil {
		t.Fatal(err)
	}
	assert(`document.querySelector('.career-evidence-full').open`)
	assert(`document.querySelector('.career-evidence-full').textContent.includes('A customer-facing application had slow MySQL operations.')`)
	assert(`document.querySelector('.career-evidence-full').textContent.includes('Synthetic fixture notes retained for inspection.')`)
	assert(`document.querySelector('.career-evidence-full').textContent.includes('I reviewed synthetic slow-query logs and compared MySQL EXPLAIN plans.')`)
	assert(`document.querySelector('#answer-` + key + `').value===` + fmt.Sprintf("%q", dirtyAnswer))
	assert(`document.querySelector('[data-question-panel]').dataset.questionLabel===` + fmt.Sprintf("%q", field.Label))
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers WHERE job_id=?`, target.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("inspecting evidence saved the dirty editor: count=%d err=%v", count, err)
	}

	shot, err := chrome.FixtureScreenshotAt(ctx, page, 390, ".application-questions", "light")
	if err != nil || shot == "" {
		t.Fatalf("narrow workbench screenshot: %t %v", shot != "", err)
	}
	if image, decodeErr := base64.StdEncoding.DecodeString(shot); decodeErr != nil {
		t.Fatal(decodeErr)
	} else if outputDir := os.Getenv("YOURMOVE_QUALIFY_SCREENSHOT_DIR"); outputDir != "" {
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outputDir, "career-evidence-workbench-390.png"), image, 0600); err != nil {
			t.Fatal(err)
		}
	}
	assert(`document.documentElement.scrollWidth<=window.innerWidth`)

	if err := evaluate(`document.querySelector('.application-question-form button[type="submit"]').click()`, nil); err != nil {
		t.Fatal(err)
	}
	var savedAnswer string
	var answerRevision int64
	for deadline := time.Now().Add(5 * time.Second); ; {
		err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=? AND question_key=?`, target.ID, key).Scan(&savedAnswer, &answerRevision)
		if err == nil {
			break
		}
		if !errors.Is(err, sql.ErrNoRows) || time.Now().After(deadline) {
			t.Fatalf("explicit local Save answer did not persist: %q revision=%d err=%v", savedAnswer, answerRevision, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	if savedAnswer != dirtyAnswer || answerRevision != 1 {
		t.Fatalf("explicit local Save answer persisted unexpected content: %q revision=%d", savedAnswer, answerRevision)
	}
	if a.browsers.state(target.ID).Active {
		t.Fatal("local answer save armed an employer browser transfer")
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=?`, target.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("local answer save recorded employer submission: count=%d err=%v", count, err)
	}

	if err := chrome.Close(); err != nil {
		t.Fatal(err)
	}
	server.Close()
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	restartedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, restartedDB, filepath.Join(dir, "resumes"))
	restartedJob, err := restarted.jobByID(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := restarted.prepareQuestions(restartedJob, CandidateProfile{})
	if err != nil || len(prepared.Questions) != 1 || prepared.Questions[0].Value != dirtyAnswer {
		t.Fatalf("restart did not preserve answer: %+v %v", prepared, err)
	}
	suggestions, err := restarted.careerEvidenceSuggestionsForQuestions(prepared.Questions)
	if err != nil || len(suggestions[prepared.Questions[0].Key]) != 1 || suggestions[prepared.Questions[0].Key][0].Evidence.ID != evidenceID {
		t.Fatalf("restart did not preserve on-demand reviewed evidence retrieval: %+v %v", suggestions, err)
	}
	t.Log("Rendered question-specific previous writing and reviewed evidence after opening Help me answer; hid the relevant draft; inspected full evidence while preserving a dirty editor; saved only through the explicit local form; confirmed no transfer/submission; reloaded the evidence after SQLite restart.")
}
