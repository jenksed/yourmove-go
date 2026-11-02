//go:build qualification

package chromium

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Local fixtures exercise the real CDP operations without sending applicant
// material to an employer. Live tenant variants still need read-only qualification.
func TestHostedPlatformForms(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in local Chromium qualification")
	}
	for _, platform := range []string{"greenhouse", "lever"} {
		t.Run(platform, func(t *testing.T) {
			nameControls := `<label for="first_name">First Name*</label><input id="first_name" name="first_name"><label for="last_name">Last Name*</label><input id="last_name" name="last_name">`
			if platform == "lever" {
				nameControls = `<li class="application-question awli-application-row"><div class="awli-button-container"><input type="file" aria-label="Resume" name="linkedin_resume"><button type="button" class="awli-button" onclick="window.linkedinClicks++">Apply with LinkedIn</button></div></li><li class="application-question"><label><div class="application-label">Full name<span>✱</span></div><div class="application-field"><input type="text" name="name" required></div></label></li>`
			}
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprint(w, `<h1>Local application fixture</h1><form>`+nameControls+`
<label>Email<input name="email" type="email" value="manual@example.test"></label>
<label>Phone<input name="phone" type="tel"></label>
<label>Manager email<input name="manager_email" type="email"></label>
<label>Work authorization<input name="sponsorship"></label>
<li class="application-question resume"><label><div class="application-label">Resume/CV <span>✱</span></div><div class="application-field"><a href="#" class="visible-resume-upload"><span class="filename"></span><span>ATTACH RESUME/CV</span><input class="application-file-input invisible-resume-upload" id="resume-upload-input" name="resume" type="file" style="opacity:0;position:absolute"></a></div></label></li>
<section class="application-question"><div class="application-label">Cover Letter</div><input type="file" name="cover_letter" accept=".pdf" style="display:none"></section>
<label>Why this team?<textarea name="motivation"></textarea></label>
<button type="submit">Submit application</button></form>
<script>window.submitted=0;window.linkedinClicks=0;window.resumeChanges=0;document.querySelector("[name=resume]").addEventListener("change",()=>window.resumeChanges++);document.querySelector('form').addEventListener('submit',e=>{e.preventDefault();window.submitted++})</script>`)
			}))
			defer fixture.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			posting := fixture.URL + "/fixture/job"
			savedPosting := "https://boards.greenhouse.io/acme/jobs/123"
			if platform == "lever" {
				savedPosting = "https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
			}
			s, err := LaunchFixtureForPosting(ctx, posting, savedPosting)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			applicationPage := posting
			if platform == "lever" {
				applicationPage += "/apply"
			}
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
			for _, action := range observed.Actions {
				if action.Label == "Apply with LinkedIn" {
					t.Fatal("social authentication offered as an application action")
				}
			}
			contacts, err := s.FillContacts(ctx, browser.ContactValues{FirstName: "Casey", LastName: "Fixture", Email: "approved@example.test", Phone: "202-555-0104"})
			if err != nil || !contacts.Ready {
				t.Fatalf("contacts: %+v %v", contacts, err)
			}
			data := []byte("%PDF-1.4\nlocal fixture only\n%%EOF")
			resume := filepath.Join(t.TempDir(), "Casey_Fixture_Resume_Support.pdf")
			letter := filepath.Join(t.TempDir(), "Casey_Fixture_Cover_Letter_Support.pdf")
			for _, path := range []string{resume, letter} {
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := s.AttachResume(ctx, resume); err != nil || !out.Verified {
				t.Fatalf("resume: %+v %v", out, err)
			}
			if out, err := s.AttachCoverLetter(ctx, letter, fmt.Sprintf("%x", sha256.Sum256(data))); err != nil || !out.Verified {
				t.Fatalf("letter: %+v %v", out, err)
			}
			if _, err := s.AttachResume(ctx, resume); !errors.Is(err, ErrExistingDocument) {
				t.Fatalf("existing resume was not preserved: %v", err)
			}
			var field schema.Field
			for _, c := range observed.Controls {
				if c.ID == "motivation" {
					field = schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required, Options: c.Options}
				}
			}
			out, err := s.FillAnswer(ctx, browser.AnswerValue{Field: field, Value: "I enjoy resolving technical support incidents."})
			if err != nil || out.Status != "filled" {
				t.Fatalf("answer: %+v %v", out, err)
			}
			var state struct {
				Result struct {
					Value struct {
						Name, Email, Manager, Phone, Sensitive, Resume, Cover string
						Submitted, LinkedinClicks, ResumeChanges              int
					} `json:"value"`
				} `json:"result"`
			}
			err = s.FixtureCall(ctx, applicationPage, "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `({Name:document.querySelector('[name=name]')?.value||[document.querySelector('[name=first_name]')?.value||'',document.querySelector('[name=last_name]')?.value||''].join(' '),Email:document.querySelector('[name=email]').value,Phone:document.querySelector('[name=phone]').value,Manager:document.querySelector('[name=manager_email]').value,Sensitive:document.querySelector('[name=sponsorship]').value,Resume:document.querySelector('[name=resume]').files[0]?.name,Cover:document.querySelector('[name=cover_letter]').files[0]?.name,Submitted:window.submitted,LinkedinClicks:window.linkedinClicks,ResumeChanges:window.resumeChanges})`}, &state)
			got := state.Result.Value
			if err != nil || got.Name != "Casey Fixture" || got.Email != "manual@example.test" || got.Phone != "202-555-0104" || got.Manager != "" || got.Sensitive != "" || got.Submitted != 0 || got.LinkedinClicks != 0 || got.ResumeChanges != 1 || got.Resume != filepath.Base(resume) || got.Cover != filepath.Base(letter) {
				t.Fatalf("form state: %+v contacts=%+v %v", got, contacts, err)
			}

			foreignStep := applicationPage + "/unowned-step"
			if browser.OwnsPostingURL(savedPosting, savedPosting+"/unowned-step") {
				t.Fatal("fixture descendant must be outside the saved posting's ownership")
			}
			if err := s.FixtureCall(ctx, applicationPage, "Page.navigate", map[string]any{"url": foreignStep}, nil); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				var current struct {
					Result struct {
						Value string `json:"value"`
					} `json:"result"`
				}
				err := s.FixtureCall(ctx, foreignStep, "Runtime.evaluate", map[string]any{"expression": "location.href", "returnByValue": true}, &current)
				if err == nil && current.Result.Value == foreignStep {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("fixture did not reach unowned route %s: %v", foreignStep, err)
				}
				time.Sleep(25 * time.Millisecond)
			}
			if _, err := s.FillContacts(ctx, browser.ContactValues{FirstName: "Other", LastName: "Person", Email: "other@example.test", Phone: "202-555-0199"}); err == nil {
				t.Fatal("contact transfer accepted an unowned descendant route")
			}
			var preserved struct {
				Result struct {
					Value string `json:"value"`
				} `json:"result"`
			}
			if err := s.FixtureCall(ctx, foreignStep, "Runtime.evaluate", map[string]any{"expression": `document.querySelector('[name=email]').value`, "returnByValue": true}, &preserved); err != nil || preserved.Result.Value != "manual@example.test" {
				t.Fatalf("unowned route changed the form: %+v %v", preserved, err)
			}
		})
	}
}
