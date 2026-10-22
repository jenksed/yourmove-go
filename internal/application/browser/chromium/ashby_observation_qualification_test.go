//go:build qualification

package chromium

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

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

const syntheticAshbyPosting = "https://jobs.ashbyhq.com/synthetic/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

const ashbyQuestionFixture = `<!doctype html><html><head><title>Synthetic Ashby questions</title></head><body>
<h1>Apply for a synthetic role</h1>
<div class="ashby-application-form-container"><form class="ashby-application-form">
  <section data-field-entry-id="native-radio"><div class="ashby-application-form-question-title">Preferred work arrangement</div>
    <label><input type="radio" name="arrangement" value="remote">Remote</label>
    <label><input type="radio" name="arrangement" value="hybrid">Hybrid</label>
    <label><input type="radio" name="arrangement" value="office">Office</label>
    <label><input type="radio" name="arrangement" value="flexible">Flexible</label>
    <label><input type="radio" name="arrangement" value="other">Other</label>
  </section>
  <fieldset><legend>How did you find us?</legend>
    <label><input type="radio" name="referral" value="search">Search</label>
    <label><input type="radio" name="referral" value="friend">Friend</label>
    <label><input type="radio" name="referral" value="other">Other</label>
  </fieldset>
  <section data-field-entry-id="custom-radio"><div class="ashby-application-form-question-title">Preferred schedule</div>
    <div role="group" aria-label="Preferred schedule">
      <div role="radio" aria-label="Full time"></div><div role="radio" aria-label="Part time"></div>
      <div role="radio" aria-label="Contract"></div>
    </div>
  </section>
  <div role="radiogroup" id="synthetic-radio-group" aria-label="Preferred shift">
    <div role="radio" id="shift-day" aria-label="Day"></div><div role="radio" id="shift-night" aria-label="Night"></div>
  </div>
  <section data-field-entry-id="checkbox-group"><div class="ashby-application-form-question-title">Tools used</div>
    <label><input type="checkbox" name="tools[]" value="one">Tool One</label>
    <label><input type="checkbox" name="tools[]" value="two">Tool Two</label>
    <label><input type="checkbox" name="tools[]" value="three">Tool Three</label>
  </section>
  <section data-field-entry-id="custom-checkbox-group"><div class="ashby-application-form-question-title">Areas of interest</div>
    <div role="group" aria-label="Areas of interest">
      <div role="checkbox" aria-label="Customer success"></div><div role="checkbox" aria-label="Technical support"></div>
    </div>
  </section>
  <section data-field-entry-id="standalone-checkbox"><div class="ashby-application-form-question-title">Keep me informed</div>
    <label><input type="checkbox" name="updates">Yes</label>
  </section>
  <fieldset><legend>Which other contact methods work for you?</legend>
    <label><input type="checkbox" name="contact_methods[]" value="email">Email</label>
    <label><input type="checkbox" name="contact_methods[]" value="phone">Phone</label>
    <label><input type="checkbox" name="contact_methods[]" value="text">Text message</label>
    <label><input type="checkbox" name="share_updates" value="yes">Send me occasional updates</label>
  </fieldset>
  <section data-field-entry-id="native-select"><div class="ashby-application-form-question-title">Region</div>
    <select name="region"><option value="">Choose one</option><option value="east">East</option><option value="west">West</option></select>
  </section>
  <section data-field-entry-id="custom-combobox"><div class="ashby-application-form-question-title">Current location</div>
    <div role="combobox" aria-label="Current location"><input name="location" value="DO_NOT_CAPTURE"></div>
  </section>
  <section data-field-entry-id="duplicate-radio"><div class="ashby-application-form-question-title">Experience level</div>
    <label><input type="radio" name="duplicate" value="junior">Junior</label>
    <label><input type="radio" name="duplicate" value="senior">Senior</label>
    <div role="group"><div role="radio" aria-label="Junior"></div><div role="radio" aria-label="Senior"></div></div>
  </section>
  <section data-field-entry-id="same-label-first"><div class="ashby-application-form-question-title">Other information</div><input name="first_detail"></section>
  <section data-field-entry-id="same-label-second"><div class="ashby-application-form-question-title">Other information</div><input name="second_detail"></section>
  <section data-field-entry-id="race"><div class="ashby-application-form-question-title">Race and ethnicity</div>
    <label><input type="checkbox" name="race[]" value="a">Option A</label>
    <label><input type="checkbox" name="race[]" value="b">Option B</label>
    <label><input type="checkbox" name="race[]" value="c">Option C</label>
    <label><input type="checkbox" name="race[]" value="d">Option D</label>
    <label><input type="checkbox" name="race[]" value="e">Option E</label>
    <label><input type="checkbox" name="race[]" value="f">Option F</label>
    <label><input type="checkbox" name="race[]" value="g">Option G</label>
    <label><input type="checkbox" name="race[]" value="h">Option H</label>
    <label><input type="checkbox" name="race[]" value="i">Option I</label>
    <label><input type="checkbox" name="race[]" value="j">Option J</label>
  </section>
  <section data-field-entry-id="support-years"><div class="ashby-application-form-question-title">How many years of technical support experience do you have?</div>
    <div role="group" aria-label="How many years of technical support experience do you have?">
      <div role="radio" aria-label="Less than one year"></div>
      <div role="radio" aria-label="1–2 years"></div>
      <div role="radio" aria-label="3–5 years"></div>
      <div role="radio" aria-label="6–10 years"></div>
      <div role="radio" aria-label="More than 10 years"></div>
    </div>
  </section>
  <section data-field-entry-id="story"><div class="ashby-application-form-question-title">Describe a customer issue you resolved</div><textarea name="story"></textarea></section>
  <label>Email address <input name="email" type="email" value="DO_NOT_CAPTURE"></label>
  <button type="submit">Submit application</button>
</form></div><script>window.submitted=0;document.querySelector('form').addEventListener('submit',e=>{e.preventDefault();window.submitted++})</script>
</body></html>`

func TestControlledAshbyQuestionNormalization(t *testing.T) {
	if testing.Short() || strings.TrimSpace(strings.ToLower(os.Getenv("YOURMOVE_QUALIFY_BROWSER"))) != "1" {
		t.Skip("opt-in controlled Chromium question normalization qualification")
	}
	var mutations atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations.Add(1)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, ashbyQuestionFixture)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	preview, err := PreviewFixture(ctx, syntheticAshbyPosting, fixture.URL+"/synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Source != "preview" || len(preview.Fields) != 18 {
		t.Fatalf("expected 18 logical questions in preview, got %d: %+v", len(preview.Fields), preview.Fields)
	}
	if !strings.Contains(preview.Note, "49 raw visible DOM controls normalized into 18 distinct questions") {
		t.Fatalf("raw and logical counts were not reported: %s", preview.Note)
	}
	assertField := func(id, label, kind string, optionCount int) schema.Field {
		t.Helper()
		for _, field := range preview.Fields {
			if field.ID == "preview:"+id && field.Label == label {
				if field.Kind != kind || len(field.Options) != optionCount {
					t.Fatalf("field %q: kind=%q options=%v, want kind=%q option count=%d", label, field.Kind, field.Options, kind, optionCount)
				}
				return field
			}
		}
		t.Fatalf("question %q (%s) missing from %+v", label, id, preview.Fields)
		return schema.Field{}
	}
	assertField("native-radio", "Preferred work arrangement", "radio", 5)
	assertField("referral", "How did you find us?", "radio", 3)
	assertField("custom-radio", "Preferred schedule", "custom_radio_group", 3)
	assertField("aria_group_synthetic-radio-group", "Preferred shift", "custom_radio_group", 2)
	assertField("checkbox-group", "Tools used", "checkbox_group", 3)
	assertField("custom-checkbox-group", "Areas of interest", "custom_checkbox_group", 2)
	assertField("standalone-checkbox", "Keep me informed", "checkbox", 0)
	assertField("contact_methods[]", "Which other contact methods work for you?", "checkbox_group", 3)
	assertField("share_updates", "Which other contact methods work for you?", "checkbox", 0)
	assertField("native-select", "Region", "select", 3)
	assertField("custom-combobox", "Current location", "custom_combobox", 0)
	duplicate := assertField("duplicate-radio", "Experience level", "unsupported_ambiguous", 2)
	if duplicate.Sensitive {
		t.Fatal("ordinary duplicated control was unexpectedly classified sensitive")
	}
	assertField("same-label-first", "Other information", "text", 0)
	assertField("same-label-second", "Other information", "text", 0)
	race := assertField("race", "Race and ethnicity", "checkbox_group", 10)
	if !race.Sensitive {
		t.Fatal("race field lost sensitive classification")
	}
	assertField("support-years", "How many years of technical support experience do you have?", "custom_radio_group", 5)
	assertField("story", "Describe a customer issue you resolved", "textarea", 0)
	assertField("email", "Email address", "email", 0)
	serialized, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "DO_NOT_CAPTURE") {
		t.Fatal("the observer persisted an entered answer value")
	}

	// Reobserve the same controlled DOM through the real live filler and qualify
	// the existing native-radio path under the new stable field identity.
	session, err := LaunchFixture(ctx, fixture.URL+"/synthetic")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var observation browser.Observation
	for ctx.Err() == nil {
		observation, err = session.Observe(ctx)
		if err == nil && len(observation.Controls) == 18 {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if err != nil || len(observation.Controls) != 18 {
		t.Fatalf("controlled Ashby observation: controls=%d err=%v", len(observation.Controls), err)
	}
	observation.URL = syntheticAshbyPosting + "/application"
	live, err := browser.AsResult(syntheticAshbyPosting, observation)
	if err != nil {
		t.Fatal(err)
	}
	var radio schema.Field
	for _, field := range live.Fields {
		if field.ID == "browser:native-radio" {
			radio = field
			break
		}
	}
	if radio.ID == "" {
		t.Fatal("stable native-radio live identity was not produced")
	}
	outcome, err := session.FillAnswer(ctx, browser.AnswerValue{Field: radio, Value: "Hybrid"})
	if err != nil || outcome.Status != "filled" {
		t.Fatalf("qualified native radio fill: %+v %v", outcome, err)
	}
	var state struct {
		Result struct {
			Value struct {
				Selected  bool `json:"selected"`
				Submitted int  `json:"submitted"`
			} `json:"value"`
		} `json:"result"`
	}
	err = session.FixtureCall(ctx, fixture.URL+"/synthetic/application", "Runtime.evaluate", map[string]any{
		"returnByValue": true,
		"expression":    `({selected:document.querySelector('[name="arrangement"][value="hybrid"]').checked,submitted:window.submitted})`,
	}, &state)
	if err != nil || !state.Result.Value.Selected || state.Result.Value.Submitted != 0 || mutations.Load() != 0 {
		t.Fatalf("live radio verification or read-only boundary failed: %+v mutations=%d err=%v", state, mutations.Load(), err)
	}
}
