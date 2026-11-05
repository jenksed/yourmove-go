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

// This drives the production Vite React build in controlled Chromium against
// a Render-like Ashby discovery set and asserts the actually rendered
// question list: one row per distinct question, saved drafts prepared with
// their early identity, profile fields out of the editor, documents out of
// the list, the unsupported source question shown once as manual, and the
// unidentified bucket condensed into a notice. No employer browser is
// launched and nothing is filled or submitted.
func TestReactQuestionsUnifiedList(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if len(a.resources.frontendIndex) == 0 {
		t.Fatal("build frontend first")
	}
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic Render role", Description: "Synthetic Render description"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.test", Phone: "555-0100", City: "Portland", Region: "Oregon", Country: "USA", LinkedIn: "https://example.test/ada", Pronouns: "They/them", Approved: true}); err != nil {
		t.Fatal(err)
	}
	nsA := "aaaaaaaa-1111-4111-8111-aaaaaaaaaaaa"
	nsB := "bbbbbbbb-2222-4222-8222-bbbbbbbbbbbb"
	preview := func(suffix string) string { return "preview:" + nsA + suffix }
	storyLabel := "Tell us about a developer product you genuinely enjoy using and why."
	previewFields := []schema.Field{
		{ID: preview("__systemfield_name"), Label: "Name", Kind: "text", Required: true},
		{ID: preview("__systemfield_email"), Label: "Email", Kind: "email", Required: true},
		{ID: preview("_c91a27dd-b714-41a6-9484-50ea8a524f94"), Label: "Phone", Kind: "tel", Required: true},
		{ID: preview("__systemfield_location"), Label: "Location", Kind: "combobox"},
		{ID: preview("_fd231a7d-aac9-4546-88f4-7d796da5e281"), Label: "LinkedIn", Kind: "text"},
		{ID: preview("_d0f9c56d-5a87-4137-9a0a-f0241d0af505"), Label: "Website", Kind: "text"},
		{ID: preview("_e523cd20-b243-4230-9a13-50ac028dd5a0"), Label: storyLabel, Kind: "text"},
		{ID: preview("_f73c44bf-c40a-45b2-973a-858bc7c851ed"), Label: "How did you hear about Render?", Kind: "combobox"},
		{ID: preview("_a93c8020-5d35-4732-98bc-3e6fd7535612"), Label: "Pronouns", Kind: "text"},
		{ID: preview("_b1111111-1111-4111-8111-111111111111"), Label: "Race", Kind: "radio", Sensitive: true, Options: []string{"Asian", "Decline to self-identify"}},
		{ID: preview("_b6666666-4666-4666-8666-666666666666"), Label: "Work authorization status", Kind: "text"},
		{ID: preview("_c2222222-2222-4222-8222-222222222222"), Label: "Veteran Status", Kind: "radio", Sensitive: true, Options: []string{"I am not a protected veteran", "I decline to self-identify"}},
		{ID: preview("__systemfield_resume"), Label: "Resume", Kind: "file", Required: true},
		{ID: preview("_aab326cd-f9c6-4608-9262-4a98b773a08e"), Label: "Cover Letter", Kind: "file"},
		{ID: "preview:unidentified-controls", Label: "Controls without a stable field identity", Kind: "unsupported_ambiguous"},
		{ID: preview("_d3333333-3333-4333-8333-333333333333"), Label: "Other information", Kind: "text"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: testAshbyPosting + "/application", Fields: previewFields}); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	drafts := map[string]string{
		preview("_d0f9c56d-5a87-4137-9a0a-f0241d0af505"): "https://example.test/portfolio",
		preview("_e523cd20-b243-4230-9a13-50ac028dd5a0"): "I enjoy using Render because deploys are boring in the best way.",
	}
	for id, value := range drafts {
		q := earlyQuestionByID(t, a, job, id, "")
		if rec := postApp(mux, "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", value), "", ""); rec.Code != 303 {
			t.Fatalf("save %s: %d %s", id, rec.Code, rec.Body.String())
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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
		deadline := time.Now().Add(20 * time.Second)
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
	taskLabels := func() []string {
		var labels []string
		if err := eval(`Array.from(document.querySelectorAll('.writing-question strong')).map(x=>x.textContent)`, &labels); err != nil {
			t.Fatal(err)
		}
		return labels
	}
	count := func(labels []string, want string) int {
		n := 0
		for _, label := range labels {
			if label == want {
				n++
			}
		}
		return n
	}

	wait(`document.body.innerText.includes('Synthetic Render role')`)
	click("3. Questions")
	wait(`Array.from(document.querySelectorAll('.writing-question strong')).some(x=>x.textContent==='Website')`)
	got := taskLabels()
	for label, want := range map[string]int{"Website": 1, storyLabel: 1, "Other information": 1} {
		if count(got, label) != want {
			t.Fatalf("phase 1 tasks %q: %v", label, got)
		}
	}
	for _, absent := range []string{"Name", "Email", "Phone", "Location", "LinkedIn", "Pronouns", "Race", "Veteran Status", "How did you hear about Render?", "Resume", "Cover Letter", "Controls without a stable field identity"} {
		if count(got, absent) != 0 {
			t.Fatalf("phase 1 %q must not be a writing task: %v", absent, got)
		}
	}
	var guidance string
	if err := eval(`document.querySelector('.writing-previous')?.innerText || ''`, &guidance); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Candidate Profile fields are handled automatically when authorized.", "Complete on the employer site", "Race", "Needs manual completion", "How did you hear about Render?", "Work authorization status"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("phase 1 guidance missing %q: %s", want, guidance)
		}
	}
	for _, absent := range []string{"LinkedIn", "Location"} {
		if strings.Contains(guidance, absent) {
			t.Fatalf("phase 1 profile field listed instead of summarized: %q in %s", absent, guidance)
		}
	}
	for text, want := range map[string]int{"Work authorization status": 1, "How did you hear about Render?": 1, "Pronouns": 0} {
		if got := strings.Count(guidance, text); got != want {
			t.Fatalf("phase 1 guidance %q count=%d, want %d: %s", text, got, want, guidance)
		}
	}
	var notices string
	if err := eval(`Array.from(document.querySelectorAll('.writing-notice')).map(x=>x.innerText).join('\n')`, &notices); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notices, "One employer control could not be stably identified; inspect it directly in the employer browser.") {
		t.Fatalf("phase 1 unidentified notice wrong: %s", notices)
	}

	// Opening the managed browser must not duplicate the list, and saved
	// drafts keep their early editor identity.
	live := func(suffix string) string { return "browser:" + nsB + suffix }
	liveFields := []schema.Field{
		{ID: live("__systemfield_name"), Label: "Name", Kind: "text", Required: true},
		{ID: live("__systemfield_email"), Label: "Email", Kind: "email", Required: true},
		{ID: live("_c91a27dd-b714-41a6-9484-50ea8a524f94"), Label: "Phone", Kind: "tel", Required: true},
		{ID: live("__systemfield_location"), Label: "Location", Kind: "combobox"},
		{ID: live("_fd231a7d-aac9-4546-88f4-7d796da5e281"), Label: "LinkedIn", Kind: "text"},
		{ID: live("_d0f9c56d-5a87-4137-9a0a-f0241d0af505"), Label: "Website", Kind: "text"},
		{ID: live("_e523cd20-b243-4230-9a13-50ac028dd5a0"), Label: storyLabel, Kind: "text"},
		{ID: live("_f73c44bf-c40a-45b2-973a-858bc7c851ed"), Label: "How did you hear about Render?", Kind: "combobox"},
		{ID: live("_a93c8020-5d35-4732-98bc-3e6fd7535612"), Label: "Pronouns", Kind: "radio", Options: []string{"She/her", "They/them", "He/him", "Decline to state"}},
		{ID: live("_b1111111-1111-4111-8111-111111111111"), Label: "Race", Kind: "radio", Sensitive: true, Options: []string{"Asian", "Decline to self-identify"}},
		{ID: live("_b6666666-4666-4666-8666-666666666666"), Label: "Work authorization status", Kind: "radio", Options: []string{"Yes", "No"}},
		{ID: live("_c2222222-2222-4222-8222-222222222222"), Label: "Veteran Status", Kind: "radio", Sensitive: true, Options: []string{"I am not a protected veteran", "I decline to self-identify"}},
		{ID: "browser:unidentified-controls", Label: "Controls without a stable field identity", Kind: "unsupported_ambiguous"},
		{ID: live("_d3333333-3333-4333-8333-333333333333"), Label: "Other information", Kind: "text"},
		{ID: live("_e4444444-4444-4444-8444-444444444444"), Label: "Other information", Kind: "text"},
	}
	liveRowID, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: liveFields})
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	a.browsers.states[1] = browserRunState{Destination: testAshbyPosting, LatestID: liveRowID}
	if a.browsers.sessions == nil {
		a.browsers.sessions = map[int64]readOnlySession{}
	}
	a.browsers.sessions[1] = &fakeBrowser{url: testAshbyPosting + "/application"}
	a.browsers.mu.Unlock()
	click("Refresh questions")
	wait(`Array.from(document.querySelectorAll('.writing-question strong')).filter(x=>x.textContent==='Other information').length===2`)
	got = taskLabels()
	for label, want := range map[string]int{"Website": 1, storyLabel: 1, "Other information": 2, "How did you hear about Render?": 0} {
		if count(got, label) != want {
			t.Fatalf("phase 2 tasks %q: %v", label, got)
		}
	}
	if err := eval(`document.querySelector('.writing-previous')?.innerText || ''`, &guidance); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Candidate Profile fields are handled automatically when authorized.", "Work authorization status", "How did you hear about Render?"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("phase 2 guidance missing %q: %s", want, guidance)
		}
	}
	for _, absent := range []string{"LinkedIn", "Location"} {
		if strings.Contains(guidance, absent) {
			t.Fatalf("phase 2 profile field listed instead of summarized: %q in %s", absent, guidance)
		}
	}
	for text, want := range map[string]int{"Work authorization status": 1, "How did you hear about Render?": 1, "Pronouns": 0} {
		if got := strings.Count(guidance, text); got != want {
			t.Fatalf("phase 2 guidance %q count=%d, want %d: %s", text, got, want, guidance)
		}
	}
	if err := eval(`Array.from(document.querySelectorAll('.writing-notice')).map(x=>x.innerText).join('\n')`, &notices); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notices, "One employer control could not be stably identified; inspect it directly in the employer browser.") {
		t.Fatalf("phase 2 unidentified notice wrong: %s", notices)
	}
	run(`Array.from(document.querySelectorAll('.writing-question')).find(x=>x.textContent.includes('Website')).click()`)
	wait(`document.querySelector('#writing-answer')?.value === 'https://example.test/portfolio'`)
	var editor string
	if err := eval(`document.querySelector('.writing-editor')?.innerText || ''`, &editor); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(editor, "Published definition (preview)") {
		t.Fatalf("saved draft lost its early editor identity: %s", editor)
	}
	t.Log("Unified React list: one task row per distinct question; drafts prepared with early identity; profile fields summarized; employer and manual guidance shown once from the current observation; unidentified controls condensed into one notice.")
}
