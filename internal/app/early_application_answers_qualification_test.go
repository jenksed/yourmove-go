//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestControlledEarlyPreparationWorkbench(t *testing.T) {
	if testing.Short() || os.Getenv("YOURMOVE_QUALIFY_EARLY") != "1" {
		t.Skip("opt-in isolated Chromium early-preparation qualification")
	}
	a, _ := testWorkspace(t)
	job, result := earlyGreenhouseFixture(t, a)
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
	mux := newMux(a)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mux.ServeHTTP(w, r) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	page := server.URL + "/jobs/1#prepare"
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
	wait(`document.readyState==='complete'&&!!document.querySelector('.early-application-questions form[action$="/inspect"] button')`)
	assert(`document.querySelector('[data-question-workbench]')===null||document.querySelectorAll('[data-question-workbench] [data-question-panel]').length===0`)
	run(`document.querySelector('.early-application-questions form[action$="/inspect"] button').click()`)
	wait(`document.readyState==='complete'&&!!document.querySelector('.early-application-questions [data-answer-ai]')`)
	if inspectionCalls.Load() != 1 {
		t.Fatalf("expected one explicitly requested public inspection, got %d", inspectionCalls.Load())
	}
	run(`window.ai=document.querySelector('.early-application-questions [data-answer-ai]');window.find=n=>ai.querySelector('[data-answer-ai-'+n+']');window.editor=document.querySelector('.early-application-questions textarea[name=answer]');ai.open=true`)
	assert(`ai.dataset.early==='yes'&&editor.value===''&&ai.querySelector('[name=career_evidence]')&&!ai.querySelector('[name=career_evidence]').checked`)
	run(`ai.querySelector('[name=career_evidence]').checked=true;ai.querySelector('[name=model]').value='synthetic-model';find('review').click()`)
	wait(`!find('review-panel').hidden`)
	assert(`find('execution').textContent.includes('Published question')`)
	assert(`find('authorize')&&!find('authorize').checked&&find('external')&&!find('external').checked`)
	run(`find('authorize').checked=true;find('external').checked=true;find('generate').click()`)
	wait(`!find('preview').hidden&&find('text').value.includes('query plan')`)
	if generationCalls.Load() != 1 {
		t.Fatalf("expected one explicitly authorized AI request, got %d", generationCalls.Load())
	}
	if invalidProviderPrompt.Load() {
		t.Fatal("early question was not identified as unverified in the model context")
	}
	var earlyCount, liveCount, browserCount, submissionCount int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&earlyCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&liveCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_inspections WHERE source='browser'`).Scan(&browserCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissionCount)
	if earlyCount != 0 || liveCount != 0 || browserCount != 0 || submissionCount != 0 {
		t.Fatalf("AI preview or public discovery crossed a live boundary: early=%d live=%d browser=%d submissions=%d", earlyCount, liveCount, browserCount, submissionCount)
	}
	run(`find('adopt').click()`)
	wait(`editor.value.includes('query plan')&&find('status').textContent.includes('Save early answer locally')`)
	assert(`document.querySelector('.early-application-questions [data-question-panel]:not([hidden]) [data-answer-ai]')`)
	run(`editor.value+=' I documented the recommendation.';editor.dispatchEvent(new Event('input',{bubbles:true}));editor.closest('form').querySelector('button[type=submit]').click()`)
	wait(`document.readyState==='complete'&&document.querySelector('.early-application-questions textarea[name=answer]')?.value.includes('documented the recommendation')`)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&earlyCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&liveCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM application_inspections WHERE source='browser'`).Scan(&browserCount)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissionCount)
	if earlyCount != 1 || liveCount != 0 || browserCount != 0 || submissionCount != 0 {
		t.Fatalf("saved early proposal crossed a live boundary: early=%d live=%d browser=%d submissions=%d", earlyCount, liveCount, browserCount, submissionCount)
	}
}
