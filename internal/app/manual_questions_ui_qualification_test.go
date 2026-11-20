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
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Real Chromium against the production Vite build and an unsupported posting:
// Capture stays disabled with truthful guidance, the workspace is not dimmed,
// and the applicant can add a question manually, prepare and save an answer,
// reuse Answer Library writing, copy it for the employer site, edit the
// question, and remove it with confirmation. No employer browser is launched
// and nothing is filled or submitted.
func TestReactManualQuestionPreparation(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if len(a.resources.frontendIndex) == 0 {
		t.Fatal("build frontend first")
	}
	if err := a.saveJob(Job{URL: unsupportedPostingURL, Host: "careers.example.test", Title: "Operations Lead"}); err != nil {
		t.Fatal(err)
	}
	// One Answer Library example to reuse while preparing manually.
	seedJob := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer", Company: "Fixture Systems"}
	if err := a.saveJob(seedJob); err != nil {
		t.Fatal(err)
	}
	seedJob, _ = a.jobByID(2)
	seedField := schema.Field{ID: "browser:motivation", Label: "Why do you want to join this team?", Kind: "textarea"}
	if _, err := a.saveInspection(2, schema.Result{Platform: "ashby", Source: "browser", ApplicationURL: seedJob.URL + "/application", Fields: []schema.Field{seedField}}); err != nil {
		t.Fatal(err)
	}
	seedKey := questionKey(seedJob.URL, "ashby", seedField)
	saveLibrarySourceAnswer(t, a, seedJob, seedKey, "I enjoy resolving difficult technical problems for customers.", 0)
	if rec := promoteLibrarySource(t, a, seedJob.ID, seedKey, 1); rec.Code != http.StatusSeeOther {
		t.Fatalf("seed promotion: %d %s", rec.Code, rec.Body.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1", http.StatusSeeOther) })
	router.Handle("/", newMux(a))
	app := httptest.NewServer(router)
	defer app.Close()
	page := app.URL + "/jobs/1"
	ui, err := chromium.LaunchFixture(ctx, app.URL)
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
		if err := ui.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &reply); err != nil {
			return err
		}
		if len(reply.Exception) > 0 {
			return fmt.Errorf("JS %s", reply.Exception)
		}
		if out != nil {
			return json.Unmarshal(reply.Result.Value, out)
		}
		return nil
	}
	run := func(expression string) {
		t.Helper()
		if err := eval(expression, nil); err != nil {
			t.Fatal(err)
		}
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
		_ = eval(`document.body.innerText`, &body)
		t.Fatalf("timeout %s\n%s", expression, body)
	}
	click := func(label string) {
		t.Helper()
		run(fmt.Sprintf(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()==%q && x.offsetParent!==null).click()`, label))
	}
	setInput := func(selector, value string) {
		t.Helper()
		encoded, _ := json.Marshal(value)
		run(fmt.Sprintf(`(()=>{const e=document.querySelector(%q);if(!e)throw Error('missing control');e.focus();Object.getOwnPropertyDescriptor(e.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype,'value').set.call(e,%s);e.dispatchEvent(new Event('input',{bubbles:true}));return true})()`, selector, string(encoded)))
	}

	wait(`document.body.innerText.includes('Operations Lead')`)
	click("Questions")
	wait(`document.querySelector('#writing') && document.body.innerText.includes('Add question manually')`)

	// Capture is unavailable, the workspace is not dimmed, and manual
	// preparation is offered instead of a dead end.
	wait(`Array.from(document.querySelectorAll('button')).some(x=>x.textContent.trim()==='Capture application questions' && x.disabled)`)
	var dimmed bool
	if err := eval(`!!document.querySelector('#writing.questions-unsupported')`, &dimmed); err != nil {
		t.Fatal(err)
	}
	if dimmed {
		t.Fatal("unsupported Questions workspace is dimmed")
	}
	var notices string
	if err := eval(`Array.from(document.querySelectorAll('.writing-notice')).map(x=>x.innerText).join('\n')`, &notices); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notices, "Select Add question manually") || !strings.Contains(notices, "automatic Fill remains unavailable") {
		t.Fatalf("unsupported guidance wrong: %s", notices)
	}

	// Add a manual question; it becomes the selected writing task.
	click("Add question manually")
	wait(`!!document.querySelector('#manual-question-text')`)
	setInput("#manual-question-text", "Describe the support tools you have used")
	click("Add question")
	wait(`document.querySelector('.writing-editor h3')?.textContent==='Describe the support tools you have used'`)
	wait(`!!document.querySelector('#writing-answer') && document.querySelector('#writing-answer').value===''`)
	var tasks string
	if err := eval(`document.querySelector('.question-selector select')?.innerText || ''`, &tasks); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tasks, "Describe the support tools you have used") {
		t.Fatalf("manual question not in navigator: %s", tasks)
	}

	// Prepare, save, and keep editing across a reload.
	setInput("#writing-answer", "I used the search console and the support inbox.")
	click("Save for this application only")
	wait(`document.body.innerText.includes('Saved revision 1')`)
	run(`location.reload()`)
	wait(`document.body.innerText.includes('Operations Lead')`)
	click("Questions")
	wait(`document.querySelector('.writing-editor h3')?.textContent==='Describe the support tools you have used'`)
	wait(`document.querySelector('#writing-answer')?.value==='I used the search console and the support inbox.'`)

	// Edit the employer wording explicitly.
	click("Edit question")
	wait(`!!document.querySelector('#manual-question-edit')`)
	setInput("#manual-question-edit", "Describe the support tools you have used remotely")
	click("Save for this application only")
	wait(`document.querySelector('.writing-editor h3')?.textContent==='Describe the support tools you have used remotely'`)

	// Copy the prepared answer for manual employer-site completion.
	click("Copy answer")
	wait(`document.body.innerText.includes('Paste it into the employer form') || document.body.innerText.includes('Copy the text below manually')`)

	// Reuse Answer Library writing as a starting draft.
	click("Browse all")
	wait(`!!document.querySelector('.library-browse')`)
	var libraryText string
	if err := eval(`document.querySelector('.library-browse')?.innerText || ''`, &libraryText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(libraryText, "Why do you want to join this team?") {
		t.Fatalf("library browse missing seeded example: %s", libraryText)
	}
	run(`Array.from(document.querySelectorAll('.library-browse button')).find(x=>x.textContent.trim()==='Preview full text').click()`)
	wait(`document.querySelector('.writing-preview') && document.querySelector('.writing-preview').innerText.includes('I enjoy resolving difficult technical problems')`)
	click("Use as starting draft")
	wait(`document.querySelector('#writing-answer')?.value==='I enjoy resolving difficult technical problems for customers.'`)
	wait(`document.body.innerText.includes('Started from')`)
	click("Save for this application only")
	wait(`document.body.innerText.includes('Saved for this application only')`)

	// Remove with confirmation; the library example survives.
	click("Remove question")
	wait(`Array.from(document.querySelectorAll('button')).some(x=>x.textContent.trim()==='Confirm remove')`)
	click("Confirm remove")
	wait(`document.body.innerText.includes('No questions discovered yet')`)
	entries, err := a.answerLibraryEntries()
	if err != nil || len(entries) != 1 {
		t.Fatalf("remove damaged the Answer Library: %+v %v", entries, err)
	}
	if entries[0].OriginalQuestion != "Why do you want to join this team?" {
		t.Fatalf("wrong library entry survived: %+v", entries[0])
	}
	t.Log("Unsupported posting: Capture disabled with manual guidance, no workspace dimming; manual question added, answered, saved, reloaded, renamed, copied, library-reused, and removed with confirmation; library intact; no employer Fill.")
}
