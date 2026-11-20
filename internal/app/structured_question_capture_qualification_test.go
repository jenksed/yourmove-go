//go:build qualification

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Provider-neutral structured-question fixture: no ATS namespaces, entry
// roots, or vendor classes. It covers the acceptance inventory through the
// real managed-browser observation path:
//
//  1. native select with a question and options
//  2. standard radio group
//  3. accessible custom dropdown (stable combobox with visible listbox options)
//  4. Render-like custom dropdown: recognizable, option-less, no stable identity
//  5. recognizable custom dropdown with visible options but no Fill identity
//  6. truly unidentified control (no label, no identity)
//  7. candidate-profile and document controls (must stay out of questions)
const structuredCaptureFixture = `<!doctype html><html><head><title>Structured questions</title></head><body>
<h1>Apply for a synthetic role</h1>
<form>
  <label for="pattern">Preferred working pattern</label>
  <select id="pattern" name="pattern" required>
    <option value="">Choose one</option><option>Remote</option><option>Office</option><option>Hybrid</option>
  </select>
  <fieldset><legend>Preferred shift</legend>
    <label><input type="radio" name="shift" value="day">Day</label>
    <label><input type="radio" name="shift" value="night">Night</label>
  </fieldset>
  <span id="source-label">How did you find us?</span>
  <div role="combobox" id="referral-combo" aria-labelledby="source-label" aria-expanded="true" aria-controls="source-popup">
    <input name="referral_source" readonly>
    <ul role="listbox" id="source-popup" aria-labelledby="source-label">
      <li role="option">Search engine</li><li role="option">Friend</li><li role="option">Other</li>
    </ul>
  </div>
  <span id="hear-label">How did you hear about Render?</span>
  <div role="combobox" aria-labelledby="hear-label" aria-expanded="false" tabindex="0" aria-controls="hear-popup"><span>Select&hellip;</span></div>
  <ul role="listbox" id="hear-popup" aria-labelledby="hear-label"></ul>
  <span id="tools-label">Which tools have you used?</span>
  <div role="combobox" aria-labelledby="tools-label" aria-expanded="true" tabindex="0" aria-controls="tools-popup"><span>Select&hellip;</span></div>
  <ul role="listbox" id="tools-popup" aria-labelledby="tools-label">
    <li role="option">Search console</li><li role="option">Support inbox</li>
  </ul>
  <input type="text">
  <label>Email address <input name="email" type="email"></label>
  <label>Resume <input name="resume" type="file"></label>
  <label>Cover letter upload <input name="cover-letter" type="file"></label>
  <button type="submit">Submit application</button>
</form><script>window.submitted=0;document.querySelector('form').addEventListener('submit',e=>{e.preventDefault();window.submitted++})</script>
</body></html>`

func TestControlledStructuredQuestionCapture(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("controlled Chromium only")
	}
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	var mutations atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations.Add(1)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, structuredCaptureFixture)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	session, err := chromium.LaunchFixture(ctx, fixture.URL+"/structured")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	var observation browser.Observation
	for deadline := time.Now().Add(10 * time.Second); ; {
		observation, err = session.Observe(ctx)
		if err == nil && len(observation.Controls) == 9 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("structured observation: controls=%d err=%v observation=%+v", len(observation.Controls), err, observation)
		}
		time.Sleep(50 * time.Millisecond)
	}
	observation.URL = testAshbyPosting + "/application"
	result, err := browser.AsResult(testAshbyPosting, observation)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Note, "14 raw visible DOM controls normalized into 9 distinct questions") {
		t.Fatalf("raw and logical counts were not reported: %s", result.Note)
	}
	find := func(id, label, kind string, options int) schema.Field {
		t.Helper()
		for _, field := range result.Fields {
			if field.ID == "browser:"+id && field.Label == label {
				if field.Kind != kind || len(field.Options) != options {
					t.Fatalf("field %q: kind=%q options=%v, want kind %q option count %d", label, field.Kind, field.Options, kind, options)
				}
				return field
			}
		}
		t.Fatalf("question %q (%s) missing from %+v", label, id, result.Fields)
		return schema.Field{}
	}
	find("pattern", "Preferred working pattern", "select", 4)
	find("shift", "Preferred shift", "radio", 2)
	find("referral-combo", "How did you find us?", "custom_combobox", 3)
	hear := find("aria_group_hear-popup", "How did you hear about Render?", "custom_combobox", 0)
	find("aria_group_tools-popup", "Which tools have you used?", "custom_combobox", 2)
	find("unidentified-controls", "Controls without a stable field identity", "unsupported_ambiguous", 0)
	find("email", "Email address", "email", 0)
	find("resume", "Resume", "file", 0)
	find("cover-letter", "Cover letter upload", "file", 0)
	if len(result.Fields) != 9 {
		t.Fatalf("expected 9 distinct questions, got %d: %+v", len(result.Fields), result.Fields)
	}
	for _, field := range result.Fields {
		if field.Label == "How did you hear about Render?" && field.ID != "browser:aria_group_hear-popup" {
			t.Fatalf("Render-like question reported twice: %+v", result.Fields)
		}
	}

	inspectionID, err := a.saveInspection(1, result)
	if err != nil {
		t.Fatal(err)
	}
	a.browsers.mu.Lock()
	if a.browsers.states == nil {
		a.browsers.states = map[int64]browserRunState{}
	}
	if a.browsers.sessions == nil {
		a.browsers.sessions = map[int64]readOnlySession{}
	}
	a.browsers.states[1] = browserRunState{Active: true, Destination: testAshbyPosting, LatestID: inspectionID}
	a.browsers.sessions[1] = session
	a.browsers.mu.Unlock()
	defer a.browsers.closeAll()

	job, _ := a.jobByID(1)
	profile, _ := a.candidateProfile()
	prep, err := a.prepareQuestions(job, profile)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range prep.ReadyAnswers() {
		t.Fatalf("unexpected Fill-eligible answer without saves: %+v", q.Field)
	}
	editable := map[string]bool{}
	for _, q := range prep.ActionableQuestions() {
		editable[q.Field.Label] = true
	}
	if !editable["Preferred working pattern"] || !editable["Preferred shift"] || len(prep.ActionableQuestions()) != 2 {
		t.Fatalf("actionable questions: %+v", prep.ActionableQuestions())
	}
	review := map[string]preparedQuestion{}
	for _, q := range prep.ManualReviewQuestions() {
		review[q.Field.Label] = q
	}
	for _, label := range []string{"How did you find us?", "How did you hear about Render?", "Which tools have you used?"} {
		q, ok := review[label]
		if !ok || q.Editable {
			t.Fatalf("manual review lost or editable: %q %+v", label, review)
		}
	}

	payload := writingQuestions(t, a, 1)
	unified, _ := payload["questions"].([]any)
	rows := map[string][]map[string]any{}
	for _, item := range unified {
		q := item.(map[string]any)
		rows[q["label"].(string)] = append(rows[q["label"].(string)], q)
	}
	if len(rows["How did you hear about Render?"]) != 1 {
		t.Fatalf("Render-like question must appear once: %v", rows["How did you hear about Render?"])
	}
	hearRow := rows["How did you hear about Render?"][0]
	if hearRow["editable"] != false || hearRow["triageCategory"] != "review" || hearRow["manualReason"] == "" {
		t.Fatalf("Render-like row misrepresented: %v", hearRow)
	}
	if hearRow["kind"] == "select" || hearRow["kind"] == "radio" || hearRow["kind"] == "text" {
		t.Fatalf("Render-like row claims a fillable control type: %v", hearRow["kind"])
	}
	if options, _ := hearRow["options"].([]any); len(options) != 0 {
		t.Fatalf("option-less question gained invented choices: %v", options)
	}
	toolsRow := rows["Which tools have you used?"][0]
	if toolsOptions, _ := toolsRow["options"].([]any); len(toolsOptions) != 2 || toolsOptions[0] != "Search console" {
		t.Fatalf("custom dropdown lost reliably observed choices: %v", toolsRow["options"])
	}
	if count, _ := payload["unidentifiedCount"].(float64); count != 1 {
		t.Fatalf("unidentifiedCount=%v, want 1", payload["unidentifiedCount"])
	}

	// The native select saves normally; the structured custom questions refuse.
	patternKey := rows["Preferred working pattern"][0]["key"].(string)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+patternKey+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Hybrid"}`); rec.Code != 200 {
		t.Fatalf("select save status=%d body=%s", rec.Code, rec.Body.String())
	}
	hearKey := hearRow["key"].(string)
	if rec := postWritingJSON(t, a, http.MethodPost, "/api/jobs/1/answers", `{"question_key":"`+hearKey+`","revision":0,"destination":"`+testAshbyPosting+`","answer":"Friend"}`); rec.Code != 409 {
		t.Fatalf("custom save status=%d body=%s, want 409", rec.Code, rec.Body.String())
	}
	scope := continuationScope(t, a, 1)
	pending, _ := scope["pendingAnswers"].([]any)
	if len(pending) != 1 || pending[0].(map[string]any)["label"] != "Preferred working pattern" {
		t.Fatalf("continuation scope leaked manual-only questions: %v", scope["pendingAnswers"])
	}

	// Even with a stable display identity, the employer control cannot be
	// resolved to a Fill target: the attempt stays manual, the form is
	// unchanged, and nothing is submitted.
	outcome, err := session.FillAnswer(ctx, browser.AnswerValue{Field: hear, Value: "Friend"})
	if err != nil || outcome.Status != "manual" {
		t.Fatalf("custom Fill outcome: %+v %v", outcome, err)
	}
	var rendered struct {
		Result struct {
			Value struct {
				Pattern   string `json:"pattern"`
				Email     string `json:"email"`
				Submitted int    `json:"submitted"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := session.FixtureCall(ctx, fixture.URL+"/structured/application", "Runtime.evaluate", map[string]any{
		"returnByValue": true,
		"expression":    `({pattern:document.querySelector('[name=pattern]').value,email:document.querySelector('[name=email]').value,submitted:window.submitted})`,
	}, &rendered); err != nil {
		t.Fatal(err)
	}
	if rendered.Result.Value.Pattern != "" || rendered.Result.Value.Email != "" || rendered.Result.Value.Submitted != 0 || mutations.Load() != 0 {
		t.Fatalf("observation mutated the employer form: %+v mutations=%d", rendered, mutations.Load())
	}
	t.Log("Managed observation recognized the native select, radio group, named combobox with popup choices, two identity-less custom dropdowns (one option-less Render-like), and one unidentified control; profile/document controls stayed out; saves and continuation exclude manual-only questions; custom Fill stays manual with zero submissions.")
}
