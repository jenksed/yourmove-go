//go:build qualification

package chromium

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"example.com/yourmove-lite/internal/application/browser"
	"example.com/yourmove-lite/internal/application/schema"
)

func TestTrialBlurAndLocation(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in Chromium qualification")
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<div class="ashby-application-form-container"><form>
 <label>Name<input name="name" required></label><input type="file">
 <label>LinkedIn Profile<input name="linkedin" type="url"></label>
 <label>Location<input name="location" role="combobox" aria-controls="places" required></label>
 <div id="places" role="listbox" hidden><div role="option">Cadillac, Michigan, United States</div></div>
 </form></div><script>
 window.committed={};window.submissions=0;window.selection=false;
 document.querySelector('form').onsubmit=e=>{e.preventDefault();window.submissions++};
 document.querySelectorAll('input').forEach(e=>e.addEventListener('blur',()=>{window.committed[e.name]=e.value}));
 const locationInput=document.querySelector('[name=location]');
 locationInput.oninput=()=>{document.getElementById('places').hidden=false;locationInput.setCustomValidity('Select a location')};
 document.querySelector('[role=option]').onclick=()=>{window.selection=true;locationInput.value='Cadillac, Michigan, United States';locationInput.setCustomValidity('');document.getElementById('places').hidden=true};
 </script>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	posting := fixture.URL + "/job"
	s, err := LaunchFixture(ctx, posting)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var observed browser.Observation
	for ctx.Err() == nil {
		observed, err = s.Observe(ctx)
		if err == nil && len(observed.Controls) >= 4 {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	contacts, err := s.FillContacts(ctx, browser.ContactValues{FirstName: "Casey", LastName: "Fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts.Fields) == 0 || contacts.Fields[0].Status != "filled" {
		t.Fatalf("contacts: %+v", contacts)
	}
	for _, key := range []string{"linkedin", "location"} {
		var field schema.Field
		for _, c := range observed.Controls {
			if c.ID == key {
				field = schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required, Options: c.Options}
			}
		}
		value := "https://www.linkedin.com/in/fixture"
		if key == "location" {
			value = "Cadillac, Michigan, USA"
		}
		out, err := s.FillAnswer(ctx, browser.AnswerValue{Field: field, Value: value})
		if err != nil || out.Status != "filled" {
			t.Fatalf("%s: %+v %v", key, out, err)
		}
	}
	var state struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	err = s.FixtureCall(ctx, posting+"/application", "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `window.committed.name==='Casey Fixture'&&window.committed.linkedin==='https://www.linkedin.com/in/fixture'&&window.committed.location==='Cadillac, Michigan, United States'&&window.selection&&document.querySelector('form').checkValidity()&&window.submissions===0`}, &state)
	if err != nil || !state.Result.Value {
		t.Fatalf("focus/selection/validation failed: %+v %v", state, err)
	}
	// A saved manual location must survive subsequent approved transfers.
	var ignored any
	err = s.FixtureCall(ctx, posting+"/application", "Runtime.evaluate", map[string]any{"expression": `document.querySelector('[name=location]').value='Manual location'`}, &ignored)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range observed.Controls {
		if c.ID == "location" {
			out, err := s.FillAnswer(ctx, browser.AnswerValue{Field: schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required}, Value: "Cadillac, Michigan, USA"})
			if err != nil || out.Status != "manual" {
				t.Fatalf("manual location: %+v %v", out, err)
			}
		}
	}
}

func TestTrialCopyDescription(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in Chromium qualification")
	}
	script, err := os.ReadFile("../../../../static/ui.js")
	if err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ui.js" {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(script)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<button data-copy-description hidden>Copy job description</button><span data-copy-description-status role="status"></span><div id="job-description" style="white-space:pre-wrap">Full role description

Responsibilities
• Help customers &amp; teammates
• Investigate issues

End of complete description.</div><script>Object.defineProperty(navigator,'clipboard',{value:{writeText:async text=>{window.copied=text}}})</script><script src="/ui.js"></script>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	posting := fixture.URL + "/job"
	s, err := LaunchFixture(ctx, posting)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var state struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	for ctx.Err() == nil {
		err = s.FixtureCall(ctx, posting+"/application", "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `(()=>{const b=document.querySelector('[data-copy-description]');if(!b||b.hidden)return false;b.click();return true})()`}, &state)
		if err == nil && state.Result.Value {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	err = s.FixtureCall(ctx, posting+"/application", "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `window.copied==='Full role description\n\nResponsibilities\n• Help customers & teammates\n• Investigate issues\n\nEnd of complete description.'&&document.querySelector('[data-copy-description-status]').textContent==='Job description copied.'`}, &state)
	if err != nil || !state.Result.Value {
		t.Fatalf("plain text clipboard payload: %+v %v", state, err)
	}
}
