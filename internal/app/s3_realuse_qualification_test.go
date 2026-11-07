//go:build qualification

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jenksed/yourmove-go/internal/application/schema"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// Explicit opt-in: real public form, synthetic materials, never submit.
func TestS3LiveQuestionsAndContacts(t *testing.T) {
	posting := os.Getenv("YOURMOVE_LIVE_FILL_URL")
	if posting == "" {
		t.Skip("explicit synthetic live-fill URL required")
	}
	a, _ := testWorkspace(t)
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if len(a.resources.frontendIndex) == 0 {
		t.Fatal("build frontend first")
	}
	if err := a.saveJob(Job{URL: posting, Host: "jobs.ashbyhq.com", Title: "Synthetic live qualification"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Phone: "2025550100", Approved: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.importResume("synthetic.pdf", bytes.NewReader(mustQualificationPDF(t))); err != nil {
		t.Fatal(err)
	}
	var previews atomic.Int32
	a.previewApplication = func(ctx context.Context, raw string) (schema.Result, error) {
		previews.Add(1)
		return chromium.Preview(ctx, raw)
	}
	defer a.browsers.closeAll()
	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/1#questions", http.StatusSeeOther)
	})
	router.Handle("/", newMux(a))
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ui, err := chromium.LaunchFixture(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Close()
	eval := func(expression string, out any) error {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
			Exception json.RawMessage `json:"exceptionDetails"`
		}
		err := ui.FixtureCall(ctx, server.URL+"/jobs/1", "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &reply)
		if err != nil {
			return err
		}
		if len(reply.Exception) > 0 {
			return fmt.Errorf("JS: %s", reply.Exception)
		}
		if out != nil {
			return json.Unmarshal(reply.Result.Value, out)
		}
		return nil
	}
	wait := func(expression string) {
		t.Helper()
		deadline := time.Now().Add(25 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			if eval(expression, &ok) == nil && ok {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		var body string
		_ = eval("document.body.innerText", &body)
		t.Fatalf("timeout %s: %s", expression, body)
	}
	click := func(label string) {
		t.Helper()
		if err := eval(fmt.Sprintf(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()===%q&&x.offsetParent!==null).click()`, label), nil); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	wait(`!!document.querySelector('#writing-answer')`)
	t.Logf("First eligible editor visible after %s", time.Since(started))
	if a.browsers.state(1).Active {
		t.Fatal("discovery opened managed browser")
	}
	var inventory string
	_ = eval(`document.querySelector('[aria-label="Question navigation"]')?.innerText`, &inventory)
	t.Logf("Question inventory: %s", inventory)
	click("Documents")
	cachedStarted := time.Now()
	click("Questions")
	wait(`!!document.querySelector('#writing-answer')`)
	t.Logf("Cached Questions re-entry: %s; previews=%d", time.Since(cachedStarted), previews.Load())
	if previews.Load() != 1 {
		t.Fatal("ordinary tab navigation repeated discovery")
	}
	click("Documents")
	click("Change résumé")
	wait(`document.querySelector('.resume-chooser')?.open===true`)
	if err := eval(`Array.from(document.querySelectorAll('.resume-chooser input[name=chooser-resume]')).find(x=>!x.checked).click()`, nil); err != nil {
		t.Fatal(err)
	}
	wait(`document.querySelector('.resume-chooser')?.open===false`)
	click("Apply")
	wait(`Array.from(document.querySelectorAll('button')).some(x=>x.textContent.trim()==='Fill Application'&&!x.disabled)`)
	click("Fill Application")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		state := a.browsers.state(1)
		filled := map[string]bool{}
		for _, outcome := range state.ContactOutcomes {
			if outcome.Status == "filled" || outcome.Status == "correct" {
				filled[outcome.Key] = true
			}
		}
		if filled["name"] && filled["email"] {
			t.Logf("resume=%s contacts=%s outcomes=%+v", state.ResumeStatus, state.ContactStatus, state.ContactOutcomes)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("contact filling did not finish: %+v", a.browsers.state(1))
}
