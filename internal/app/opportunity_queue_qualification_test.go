//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

func TestControlledOpportunityQueue(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_QUEUE") != "1" {
		t.Skip("opt-in isolated queue workflow")
	}
	a, _ := testWorkspace(t)
	for _, j := range []Job{
		{URL: "https://example.test/review", Title: "Support Engineer", Company: "Example Support"},
		{URL: "https://example.test/ready", Title: "Implementation Engineer", Company: "Example Systems", Stage: StageReady},
		{URL: "https://example.test/closed", Title: "Closed role", Stage: StageClosed},
		{URL: "https://example.test/applied", Title: "Recorded role"},
	} {
		if err := a.saveJob(j); err != nil {
			t.Fatal(err)
		}
		if j.Stage != "" {
			var id int64
			if err := a.db.QueryRow("SELECT id FROM jobs WHERE url=?", j.URL).Scan(&id); err != nil {
				t.Fatal(err)
			}
			saved, err := a.jobByID(id)
			if err != nil {
				t.Fatal(err)
			}
			saved.Stage = j.Stage
			if err := a.updateJob(saved); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := a.recordSubmission(4, "Previously recorded", reviewedSubmissionToken(t, a, 4)); err != nil {
		t.Fatal(err)
	}
	// The desktop adapter is exercised over this isolated loopback fixture.
	// No production storage, employer browser, or submission is involved.
	mux := newDesktopMux(a)
	router := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.Clone(r.Context())
		r.Host = "localhost"
		r.Header.Del("Origin")
		mux.ServeHTTP(w, r)
	})
	server := httptest.NewServer(router)
	defer server.Close()
	page := server.URL + "/jobs"
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, page)
	if err != nil {
		t.Fatal(err)
	}
	defer chrome.Close()

	for deadline := time.Now().Add(5 * time.Second); ; {
		err := chrome.FixtureCall(ctx, page+"/application", "Page.navigate", map[string]any{"url": page}, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	evaluate := func(script string) bool {
		t.Helper()
		var response struct {
			Result           struct{ Value bool }
			ExceptionDetails json.RawMessage
		}
		if err := chrome.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": script, "returnByValue": true}, &response); err != nil {
			return false // Navigation briefly has no target at the destination URL.
		}
		return response.Result.Value
	}
	wait := func(script string) {
		t.Helper()
		for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
			if evaluate(script) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("UI assertion failed: %s", script)
	}
	wait("document.readyState==='complete' && !!document.querySelector('[data-job-row]') && document.getElementById('results-status').textContent.includes('shown')")
	evidence := "/tmp/yourmove-queue-evidence"
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	evaluate("(()=>{const s=document.querySelector('[data-theme-choice]');s.value='light';s.dispatchEvent(new Event('change'));return true})()")
	for _, width := range []int{1280, 390} {
		shot, err := chrome.FixtureScreenshotAt(ctx, page, width, ".opportunity-dashboard", "light")
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("queue-%d.png", width)
		if err := os.WriteFile(filepath.Join(evidence, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if shot, err := chrome.FixtureScreenshotAt(ctx, page, 1280, ".opportunity-dashboard", "dark"); err != nil {
		t.Fatal(err)
	} else {
		data, err := base64.StdEncoding.DecodeString(shot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidence, "queue-1280-dark.png"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	wait("document.querySelector('[data-remaining-count]').textContent==='2 left to work on' && document.querySelector('[data-job-stage]').value==='active' && document.querySelectorAll('[data-job-row]:not([hidden])').length===2 && !!document.querySelector('.opportunity-dashboard') && document.querySelector('[data-recorded-today]').textContent==='1' && document.querySelector('.queue-progress-bar').value===1 && document.querySelector('.queue-progress-bar').max===3")
	for _, stage := range []string{"all", "applied", "closed", "ready", "active"} {
		expected := 1
		if stage == "all" {
			expected = 4
		}
		if stage == "active" {
			expected = 2
		}
		if !evaluate(fmt.Sprintf("(()=>{const s=document.querySelector('[data-job-stage]');s.value=%q;s.dispatchEvent(new Event('change'));return document.querySelectorAll('[data-job-row]:not([hidden])').length===%d})()", stage, expected)) {
			t.Fatalf("filter %s wrong", stage)
		}
	}
	if !evaluate("(()=>{const s=document.querySelector('[data-job-stage]');s.value='applied';s.dispatchEvent(new Event('change'));document.querySelector('[data-remaining-count]').click();return s.value==='active' && document.querySelectorAll('[data-job-row]:not([hidden])').length===2})()") {
		t.Fatal("remaining metric did not restore active work")
	}
	// Invisible selections cannot be accidentally deleted after changing filters.
	if !evaluate("(()=>{const s=document.querySelector('[data-job-stage]');s.value='all';s.dispatchEvent(new Event('change'));document.querySelector('[data-stage=closed] [name=job_ids]').checked=true;s.value='active';s.dispatchEvent(new Event('change'));return !document.querySelector('[data-stage=closed] [name=job_ids]').checked})()") {
		t.Fatal("filtered selection retained")
	}
	// Follow a real form POST through the desktop redirect document to history.
	evaluate("(()=>{location.href='/jobs/1#prepare';return true})()")
	page = server.URL + "/jobs/1"
	wait("!!document.querySelector('[data-record-completion]')")
	evaluate("(()=>{const d=document.querySelector('#record-submission');d.open=true;document.querySelector('[data-record-completion] [name=confirm]').checked=true;document.querySelector('[data-record-completion] button[type=submit]').click();return true})()")
	wait("location.hash==='#history' && !!document.querySelector('#history') && !document.querySelector('#history').hidden && document.querySelector('#history').textContent.includes('Submission recorded locally.')")
	// Reusing the opportunity list refreshes the actual server-computed queue.
	evaluate("(()=>{location.href='/jobs';return true})()")
	page = server.URL + "/jobs"
	wait("document.querySelector('[data-remaining-count]')?.textContent==='1 left to work on' && document.querySelectorAll('[data-job-row]:not([hidden])').length===1 && document.querySelector('[data-recorded-today]').textContent==='2' && document.querySelector('[data-recorded-overall]').textContent==='2' && document.querySelector('.queue-progress-bar').value===2 && document.querySelector('.queue-progress-bar').max===3")
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 {
		t.Fatal("completion lost or duplicated", err)
	}
	if !evaluate("!document.body.textContent.includes('Ready to apply') && !document.body.textContent.includes('Ready to Apply')") {
		t.Fatal("unsupported readiness claim remains")
	}
	// With no JS, the server still renders complete records and accurate counts.
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "data-remaining-count><strong>1</strong>") || strings.Count(rec.Body.String(), "data-job-row ") != 4 {
		t.Fatal("no-JS view lost records or count")
	}
	t.Log("Active queue decreases after desktop completion; history renders; all stage filters work; hidden selections cleared; 1280/390px layout; no-JS records preserved.")
}
