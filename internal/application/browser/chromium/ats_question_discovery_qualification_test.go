//go:build qualification

package chromium

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// Question identity and labeling through the real read-only inspector.
// The fixture replicates the reported failure shapes with Lever's actual
// container structure (observed read-only on a live Lever posting) and
// Greenhouse-style controls: a machine-named radio group, a placeholder-only
// text input beside its real container label, and a label-less textarea
// carrying only a generic placeholder. Read-only Observe only: no Fill,
// no attachments, no submissions.
func TestATSQuestionDiscoveryLabels(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in local Chromium qualification")
	}
	const machineRadio = "cards[affbbb40-36c6-4018-95c9-da1116f645ba][field0]"
	const machineText = "cards[affbbb40-36c6-4018-95c9-da1116f645ba][field1]"
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<h1>Local ATS fixture</h1><form>`+
			`<li class="application-question"><label><div class="application-label">Full name<span>✱</span></div><div class="application-field"><input type="text" name="name" required></div></label></li>`+
			`<li class="application-question custom-question"><div><div class="application-label full-width multiple-choice"><div class="text">How did you hear about this role?<span class="required">✱</span></div></div><div class="application-field full-width"><ul>`+
			`<li><label><input type="radio" name="`+machineRadio+`" value="Spreedly Careers Page" required><span class="application-answer-alternative">Spreedly Careers Page</span></label></li>`+
			`<li><label><input type="radio" name="`+machineRadio+`" value="LinkedIn" required><span class="application-answer-alternative">LinkedIn</span></label></li>`+
			`<li><label><input type="radio" name="`+machineRadio+`" value="Other (Please explain below)" required><span class="application-answer-alternative">Other (Please explain below)</span></label></li>`+
			`</ul></div></div></li>`+
			`<li class="application-question custom-question"><div><div class="application-label full-width text"><div class="text">Other</div></div><div class="application-field full-width"><input class="card-field-input" type="text" placeholder="Type your response" value="" name="`+machineText+`"></div></div></li>`+
			`<label for="gh_first">First Name*</label><input id="gh_first" name="first_name">`+
			`<label>Resume<input type="file" name="resume"></label>`+
			`<label for="gh_why">Why this team?</label><textarea id="gh_why" name="motivation"></textarea>`+
			`<textarea name="notes" placeholder="Type your reply..."></textarea>`+
			`<button type="submit">Submit application</button></form>`)
	}))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	posting := fixture.URL + "/fixture/job"
	s, err := LaunchFixture(ctx, posting)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var observed browser.Observation
	for {
		observed, err = s.Observe(ctx)
		if err == nil && observed.Forms == 1 {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("fixture did not load: %v", err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	byID := map[string]browser.Control{}
	for _, c := range observed.Controls {
		byID[c.ID] = c
	}
	// The machine-named radio group reports the employer's container
	// question, never a fabricated identifier label.
	radio, ok := byID[machineRadio]
	if !ok {
		t.Fatalf("radio group missing from observation: %v", observed.Controls)
	}
	if !strings.Contains(radio.Label, "How did you hear about this role?") {
		t.Fatalf("radio group lost its question: %q", radio.Label)
	}
	if strings.Contains(radio.Label, "Radio group:") || strings.Contains(radio.Label, "cards[") {
		t.Fatalf("radio group kept a machine-identifier label: %q", radio.Label)
	}
	if radio.Kind != "radio" {
		t.Fatalf("radio group kind=%q, want radio", radio.Kind)
	}
	wantOptions := []string{"Spreedly Careers Page", "LinkedIn", "Other (Please explain below)"}
	if len(radio.Options) != len(wantOptions) {
		t.Fatalf("radio group options=%v, want %v", radio.Options, wantOptions)
	}
	for i := range wantOptions {
		if radio.Options[i] != wantOptions[i] {
			t.Fatalf("radio group options=%v, want %v", radio.Options, wantOptions)
		}
	}
	if !radio.Required {
		t.Fatal("radio group lost its required state")
	}
	// The placeholder-only text input reports its container question, not
	// the DOM placeholder.
	other, ok := byID[machineText]
	if !ok {
		t.Fatalf("follow-up text input missing from observation: %v", observed.Controls)
	}
	if other.Label != "Other" {
		t.Fatalf("placeholder promoted to question label: %q", other.Label)
	}
	// Ordinary labeled controls keep their employer wording.
	if got := byID["name"].Label; !strings.Contains(got, "Full name") {
		t.Fatalf("name label=%q", got)
	}
	if got := byID["first_name"].Label; !strings.Contains(got, "First Name") {
		t.Fatalf("first name label=%q", got)
	}
	if got := byID["motivation"].Label; got != "Why this team?" {
		t.Fatalf("narrative label=%q", got)
	}
	if got := byID["resume"]; got.Kind != "file" {
		t.Fatalf("resume kind=%q, want file", got.Kind)
	}
	// A control with no recoverable question keeps an honest
	// unidentified-control notice instead of placeholder wording.
	notes, ok := byID["notes"]
	if !ok {
		t.Fatalf("placeholder-only control missing from observation: %v", observed.Controls)
	}
	if notes.Label != "Unlabeled control" {
		t.Fatalf("placeholder-only control labeled %q, want the honest unidentified notice", notes.Label)
	}
	for _, c := range observed.Controls {
		if c.Label == "Type your reply..." || c.Label == "Type your response" {
			t.Fatalf("placeholder text became a question label: %+v", c)
		}
		if strings.HasPrefix(c.Label, "Radio group: ") {
			t.Fatalf("fabricated radio label observed: %+v", c)
		}
	}
}
