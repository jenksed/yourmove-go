//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// TestControlledAnswerLibraryWorkbench covers the local UI lifecycle only:
// source promotion, completed-source retention, review, cancellation, draft
// insertion, explicit saving, and persistence after a restart.
func TestControlledAnswerLibraryWorkbench(t *testing.T) {
	if testing.Short() || !strings.EqualFold(strings.TrimSpace(os.Getenv("YOURMOVE_QUALIFY_BROWSER")), "1") {
		t.Skip("opt-in isolated Chromium Answer Library qualification")
	}
	a, dir := testWorkspace(t)
	resumeDir := a.resumeDir
	sourceJob := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Technical Support Engineer", Company: "Fixture Systems"}
	if err := a.saveJob(sourceJob); err != nil {
		t.Fatal(err)
	}
	sourceJob, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	sourceField := schema.Field{ID: "browser:incident-story", Label: answerLibraryStory, Kind: "textarea"}
	if _, err := a.saveInspection(sourceJob.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: sourceJob.URL + "/application", Fields: []schema.Field{sourceField}}); err != nil {
		t.Fatal(err)
	}
	sourceKey := questionKey(sourceJob.URL, "ashby", sourceField)
	sourceAnswer := "I investigated a challenging Linux incident using logs and database diagnostics.\nI explained the fix to the customer."
	saveLibrarySourceAnswer(t, a, sourceJob, sourceKey, sourceAnswer, 0)
	if promoted := promoteLibrarySource(t, a, sourceJob.ID, sourceKey, 1); promoted.Code != http.StatusSeeOther {
		t.Fatalf("promote source example: %d %s", promoted.Code, promoted.Body.String())
	}
	entry, err := a.answerLibraryEntries()
	if err != nil || len(entry) != 1 {
		t.Fatalf("library snapshot: %d %v", len(entry), err)
	}
	token := reviewedSubmissionToken(t, a, sourceJob.ID)
	completed := postApp(newMux(a), "/jobs/1/submit", url.Values{"confirm": {"yes"}, "submission_token": {token}}, "", "")
	if completed.Code != http.StatusSeeOther {
		t.Fatalf("record synthetic source completion: %d %s", completed.Code, completed.Body.String())
	}

	destination := Job{URL: "https://jobs.ashbyhq.com/render/00000000-0000-0000-0000-000000000001", Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Second Fixture"}
	if err := a.saveJob(destination); err != nil {
		t.Fatal(err)
	}
	destination, err = a.jobByID(2)
	if err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:challenge", Label: "Tell us about a challenging Linux incident you investigated", Kind: "textarea"}
	if _, err := a.saveInspection(destination.ID, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: destination.URL + "/application", Fields: []schema.Field{field}}); err != nil {
		t.Fatal(err)
	}
	key := questionKey(destination.URL, "ashby", field)
	initialAnswer := "An existing saved response that must be protected."
	saveLibrarySourceAnswer(t, a, destination, key, initialAnswer, 0)

	appHandler := newMux(a)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/jobs/2/application" {
			http.Redirect(w, r, "/jobs/2", http.StatusSeeOther)
			return
		}
		appHandler.ServeHTTP(w, r)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
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
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			var ready bool
			if evaluate(script, &ready) == nil && ready {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("browser workbench condition timed out:", script)
	}
	assert := func(script string) {
		t.Helper()
		var okay bool
		if err := evaluate(script, &okay); err != nil || !okay {
			t.Fatal("browser workbench assertion failed:", script, err)
		}
	}
	run := func(script string) {
		t.Helper()
		if err := evaluate(script, nil); err != nil {
			t.Fatal(err)
		}
	}
	wait(`document.readyState==='complete' && !!document.querySelector('[data-related-answer]')`)
	assert(`document.querySelector('[data-related-writing-example]').textContent.includes('Describe a production incident you resolved for a customer') && document.querySelector('[data-related-writing-example]').textContent.includes('Fixture Systems')`)
	// The no-JavaScript path is a real server link to a page that renders the
	// complete answer in escaped HTML for manual reading and copy/paste.
	noJS := httptestGet(t, appHandler, "/answer-library/1")
	if noJS.Code != http.StatusOK || !strings.Contains(noJS.Body.String(), "Read the complete saved answer") ||
		!strings.Contains(noJS.Body.String(), "I investigated a challenging Linux incident") || !strings.Contains(noJS.Body.String(), "I explained the fix to the customer.") {
		t.Fatalf("manual copy path did not render the complete source text: %d", noJS.Code)
	}
	run(`document.querySelector('[data-related-answer] summary').click()`)
	wait(`document.querySelector('[data-related-answer]').dataset.loaded==='true'`)
	assert(`document.querySelector('[data-related-content]').textContent.includes('I investigated a challenging Linux incident') && !document.querySelector('[data-use-library-answer]').hidden`)
	assert(`document.querySelector('textarea[name="answer"]').value===` + fmt.Sprintf("%q", initialAnswer))
	// Cancel with a previously saved, nonempty answer: it remains byte-for-byte.
	run(`(()=>{window.confirm=()=>false;document.querySelector('[data-use-library-answer]').click()})()`)
	assert(`document.querySelector('textarea[name="answer"]').value===` + fmt.Sprintf("%q", initialAnswer))
	// Cancel a second time after an unsaved editor change; the current draft and
	// dirty state must remain untouched.
	run(`(()=>{const e=document.querySelector('textarea[name="answer"]');e.value='unsaved work to preserve';e.dispatchEvent(new Event('input',{bubbles:true}));window.confirm=()=>false;document.querySelector('[data-use-library-answer]').click()})()`)
	assert(`document.querySelector('textarea[name="answer"]').value==='unsaved work to preserve' && document.querySelector('.application-question-form').dataset.dirty==='true'`)
	// Apply the example only after the explicit action. It must stay unsaved and
	// preserve the current observed question key and revision.
	run(`(()=>{window.confirm=()=>true;document.querySelector('[data-use-library-answer]').click()})()`)
	wait(`document.querySelector('textarea[name="answer"]').value.includes('I investigated a challenging Linux incident') && document.querySelector('[data-related-status]').textContent.includes('unsaved answer')`)
	assert(`document.querySelector('[name="question_key"]').value===` + fmt.Sprintf("%q", key) + ` && document.querySelector('[name="revision"]').value==='1' && document.querySelector('.application-question-form').dataset.dirty==='true'`)
	var stored string
	var storedRevision int64
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=2 AND question_key=?`, key).Scan(&stored, &storedRevision); err != nil || stored != initialAnswer || storedRevision != 1 {
		t.Fatalf("starting point wrote to SQLite: %q revision=%d err=%v", stored, storedRevision, err)
	}
	adapted := "I traced a Linux incident through service logs and database diagnostics, restored the customer workflow, and wrote a runbook for this team's escalation path."
	run(`(()=>{const e=document.querySelector('textarea[name="answer"]');e.value=` + fmt.Sprintf("%q", adapted) + `;e.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('.application-question-form [type="submit"]').click()})()`)
	page = server.URL + "/jobs/2?question=" + key + "#prepare"
	wait(`document.readyState==='complete' && document.querySelector('textarea[name="answer"]')?.value===` + fmt.Sprintf("%q", adapted))
	assert(`document.querySelector('[name="revision"]').value==='2' && document.querySelector('[data-answer-status]').textContent.includes('Saved locally')`)
	shot, err := chrome.FixtureScreenshotAt(ctx, page, 390, ".related-previous-writing", "")
	if err != nil || shot == "" {
		t.Fatalf("390px Question Workbench rendering: screenshot=%t err=%v", shot != "", err)
	}
	if err := a.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=2 AND question_key=?`, key).Scan(&stored, &storedRevision); err != nil || stored != adapted || storedRevision != 2 {
		t.Fatalf("explicit save result=%q revision=%d err=%v", stored, storedRevision, err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE job_id=2`).Scan(&storedRevision); err != nil || storedRevision != 0 {
		t.Fatalf("local answer use unexpectedly recorded an employer submission: %d %v", storedRevision, err)
	}
	retained, err := a.answerLibraryBySourceID(a.db, entry[0].ID)
	if err != nil || retained.Answer != sourceAnswer {
		t.Fatalf("adaptation changed the source library snapshot: %q %v", retained.Answer, err)
	}
	t.Log("Applied source → explicit Answer Library snapshot → different question → canceled replacements → unsaved starting point → adapted local save; no employer submission or transfer was armed")

	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, reopenedDB, resumeDir)
	reopened, err := restarted.answerLibraryBySourceID(restarted.db, entry[0].ID)
	if err != nil || reopened.Answer != sourceAnswer {
		t.Fatalf("restart changed the original curated example: %q %v", reopened.Answer, err)
	}
	if err := restarted.db.QueryRow(`SELECT answer,revision FROM application_answers WHERE job_id=2 AND question_key=?`, key).Scan(&stored, &storedRevision); err != nil || stored != adapted || storedRevision != 2 {
		t.Fatalf("restart lost the explicitly saved destination answer: %q revision=%d err=%v", stored, storedRevision, err)
	}
	if got, err := restarted.jobByID(1); err != nil || got.Stage != StageApplied {
		t.Fatalf("source application lost its completed state: %+v %v", got, err)
	}
}
