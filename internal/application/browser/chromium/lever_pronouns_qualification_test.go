//go:build qualification

package chromium

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// Lever pronouns arrive as a collection of selectable options under one
// employer question. The read-only observer must keep the real question
// label and the full option evidence; it must never collapse the control
// into a single Yes/No checkbox.
func TestLeverPronounsObservationKeepsOptions(t *testing.T) {
	const machineName = "cards[affbbb40-36c6-4018-95c9-da1116f645ba][pronouns]"
	options := []string{"He/him", "She/her", "They/them", "Xe/xem", "Ze/hir", "Ey/em", "Hir/hir", "Fae/faer", "Hu/hu", "Use name only", "Custom"}
	var items strings.Builder
	for _, opt := range options {
		fmt.Fprintf(&items, `<li><label><input type="radio" name="%s" value="%s" required><span class="application-answer-alternative">%s</span></label></li>`, machineName, opt, opt)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<h1>Lever pronouns fixture</h1><form>`+
			`<li class="application-question custom-question"><div><div class="application-label full-width multiple-choice"><div class="text">Pronouns<span class="required">✱</span></div></div><div class="application-field full-width"><ul>`+
			items.String()+
			`</ul></div></div></li>`+
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
	pronouns, ok := byID[machineName]
	if !ok {
		t.Fatalf("pronouns group missing from observation: %v", observed.Controls)
	}
	if !strings.Contains(pronouns.Label, "Pronouns") {
		t.Fatalf("pronouns group lost its question: %q", pronouns.Label)
	}
	if strings.Contains(pronouns.Label, "cards[") || strings.HasPrefix(pronouns.Label, "Radio group: ") {
		t.Fatalf("pronouns group kept a machine-identifier label: %q", pronouns.Label)
	}
	if pronouns.Kind != "radio" {
		t.Fatalf("pronouns group kind=%q, want radio (single-select observation)", pronouns.Kind)
	}
	if len(pronouns.Options) != len(options) {
		t.Fatalf("pronouns options=%v, want %v", pronouns.Options, options)
	}
	for i := range options {
		if pronouns.Options[i] != options[i] {
			t.Fatalf("pronouns options=%v, want %v", pronouns.Options, options)
		}
	}
	for _, c := range observed.Controls {
		if c.Kind == "checkbox" && strings.Contains(strings.ToLower(c.Label), "pronoun") {
			t.Fatalf("pronouns control collapsed to Yes/No checkbox: %+v", c)
		}
		if c.Label == "Yes" || c.Label == "No" {
			t.Fatalf("unexpected Yes/No control in pronouns fixture: %+v", c)
		}
	}
}
