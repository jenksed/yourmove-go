//go:build qualification

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestQuestionsCapturePollingPreservesDraft(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Capture polling fixture"}); err != nil {
		t.Fatal(err)
	}
	field := schema.Field{ID: "browser:story", Label: "Describe a production incident", Kind: "textarea"}
	id, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field}})
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.sessions = map[int64]readOnlySession{1: &fakeBrowser{url: testAshbyPosting + "/application"}}
	defer a.browsers.close(1)
	a.browsers.states = map[int64]browserRunState{1: {Destination: testAshbyPosting, LatestID: id, Active: true, Phase: "watching"}}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1", 303) })
	router.Handle("/", newMux(a))
	server := httptest.NewServer(router)
	defer server.Close()
	ui, err := chromium.LaunchFixture(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer ui.Close()
	eval := func(expression string) bool {
		var reply struct {
			Result struct {
				Value json.RawMessage `json:"value"`
			} `json:"result"`
		}
		if err := ui.FixtureCall(ctx, server.URL+"/jobs/1", "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &reply); err != nil {
			return false
		}
		var ok bool
		_ = json.Unmarshal(reply.Result.Value, &ok)
		return ok
	}
	wait := func(expression string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if eval(expression) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("condition not reached: %s", expression)
	}
	wait("document.body.innerText.includes('Capture polling fixture')")
	eval("Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()==='Questions').click()")
	wait("!!document.querySelector('#writing-answer')")
	eval("(()=>{const e=document.querySelector('#writing-answer');Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(e,'Unsaved synthetic draft');e.dispatchEvent(new Event('input',{bubbles:true}));return true})()")
	wait("document.querySelector('#writing-answer')?.value==='Unsaved synthetic draft'")
	id, err = a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: testAshbyPosting + "/application", Fields: []schema.Field{field, {ID: "browser:additional", Label: "Additional observed question", Kind: "textarea"}}})
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.setState(1, "attention", "Synthetic incoming observation status", id, false)
	wait("document.body.innerText.includes('Synthetic incoming observation status')")
	if !eval("document.querySelector('#writing-answer')?.value==='Unsaved synthetic draft'") {
		t.Fatal("polling overwrote unsaved draft")
	}
}
