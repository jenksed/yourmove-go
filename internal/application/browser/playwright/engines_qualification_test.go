//go:build qualification

package playwright

// Shared cross-engine behavioral qualification. The same assertions run
// against Chromium, Firefox, and WebKit wherever the capability is intended
// to be common; engine-specific divergences get focused regression tests.
import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func enginesForQualification(t *testing.T) []string {
	t.Helper()
	if only := os.Getenv("YOURMOVE_QUAL_ENGINE"); only != "" {
		if !ValidEngine(only) {
			t.Fatalf("YOURMOVE_QUAL_ENGINE=%q is not a supported engine", only)
		}
		return []string{only}
	}
	return []string{EngineChromium, EngineFirefox, EngineWebKit}
}

func launchFixtureEngine(t *testing.T, ctx context.Context, engine string, fix *FixtureServer) *Session {
	t.Helper()
	session, err := LaunchFixture(ctx, engine, fix.Server.URL+"/fixture/app")
	if err != nil {
		t.Fatalf("launch %s fixture: %v", engine, err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close %s: %v", engine, err)
		}
		if _, err := os.Stat(session.dir); !os.IsNotExist(err) {
			t.Errorf("%s profile dir was not removed: %s", engine, session.dir)
		}
	})
	return session
}

func fieldByLabel(obs browser.Observation, label, kind string) schema.Field {
	for _, c := range obs.Controls {
		if c.Label == label && (kind == "" || c.Kind == kind) {
			return schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required, Sensitive: c.Sensitive, Options: c.Options}
		}
	}
	return schema.Field{}
}

func TestCrossEngineObserveControlledForm(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	fix := ControlledFormServer()
	defer func() {
		if n := fix.Close(); n != 0 {
			t.Errorf("fixture received %d submission requests", n)
		}
	}()
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			session := launchFixtureEngine(t, ctx, engine, fix)
			obs, err := session.Observe(ctx)
			if err != nil {
				t.Fatalf("observe %s: %v", engine, err)
			}
			if len(obs.Controls) == 0 {
				t.Fatalf("%s observed no controls", engine)
			}
			labels := map[string]bool{}
			for _, c := range obs.Controls {
				labels[c.Label] = true
			}
			for _, want := range []string{"First Name", "Last Name", "Email", "Phone", "Resume", "Cover Letter", "Why are you a good fit?", "Level", "Open to remote?"} {
				if !labels[want] {
					t.Errorf("%s missing control %q (saw %d controls)", engine, want, len(obs.Controls))
				}
			}
			if len(obs.Frames) == 0 {
				t.Errorf("%s recorded no embedded frames", engine)
			}
			t.Logf("%s observed %d controls, %d frames at %s (browser %s)", engine, len(obs.Controls), len(obs.Frames), obs.URL, session.Version())
		})
	}
}

func TestCrossEngineContactTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	fix := ControlledFormServer()
	defer func() {
		if n := fix.Close(); n != 0 {
			t.Errorf("fixture received %d submission requests", n)
		}
	}()
	approved := browser.ContactValues{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com", Phone: "5550100"}
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			session := launchFixtureEngine(t, ctx, engine, fix)
			result, err := session.FillContacts(ctx, approved)
			if err != nil {
				t.Fatalf("fill contacts %s: %v", engine, err)
			}
			if !result.Ready {
				t.Fatalf("%s reports form not ready", engine)
			}
			for _, field := range result.Fields {
				if field.Status != "filled" && field.Status != "correct" {
					t.Errorf("%s field %s: status %q (%s)", engine, field.Key, field.Status, field.Detail)
				}
			}
			// Second pass must not rewrite: values now match the approval.
			again, err := session.FillContacts(ctx, approved)
			if err != nil {
				t.Fatalf("refill contacts %s: %v", engine, err)
			}
			for _, field := range again.Fields {
				if field.Status != "correct" {
					t.Errorf("%s refill field %s: status %q, want correct", engine, field.Key, field.Status)
				}
			}
		})
	}
}

func TestCrossEngineResumeSelection(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	fix := ControlledFormServer()
	defer func() {
		if n := fix.Close(); n != 0 {
			t.Errorf("fixture received %d submission requests", n)
		}
	}()
	resume := filepath.Join(t.TempDir(), "resume.pdf")
	if err := os.WriteFile(resume, []byte("%PDF-1.4 synthetic resume"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			session := launchFixtureEngine(t, ctx, engine, fix)
			result, err := session.AttachResume(ctx, resume)
			if err != nil {
				t.Fatalf("attach resume %s: %v", engine, err)
			}
			if !result.Selected || !result.Verified {
				t.Errorf("%s resume selection selected=%v verified=%v (%s)", engine, result.Selected, result.Verified, result.Detail)
			}
		})
	}
}

func TestCrossEngineAnswerTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	fix := ControlledFormServer()
	defer func() {
		if n := fix.Close(); n != 0 {
			t.Errorf("fixture received %d submission requests", n)
		}
	}()
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			session := launchFixtureEngine(t, ctx, engine, fix)
			// Let the dynamic control finish rerendering before resolving it.
			time.Sleep(800 * time.Millisecond)
			obs, err := session.Observe(ctx)
			if err != nil {
				t.Fatalf("observe %s: %v", engine, err)
			}
			cases := []struct {
				label, kind, value string
				want               string
			}{
				{"Why are you a good fit?", "textarea", "I build reliable systems.", "filled"},
				{"Location", "select", "Remote", "filled"},
				{"Open to remote?", "checkbox", "true", "filled"},
				{"Level", "radio", "Senior", "filled"},
				{"Dynamic field", "text", "rerender-proof", "filled"},
				{"Manual note", "text", "overwrite attempt", "manual"},
			}
			for _, tc := range cases {
				field := fieldByLabel(obs, tc.label, tc.kind)
				if field.ID == "" {
					t.Errorf("%s: no observed field %q (%s)", engine, tc.label, tc.kind)
					continue
				}
				outcome, err := session.FillAnswer(ctx, browser.AnswerValue{Field: field, Value: tc.value})
				if err != nil {
					t.Errorf("%s answer %q: %v", engine, tc.label, err)
					continue
				}
				if outcome.Status != tc.want {
					t.Errorf("%s answer %q: status %q, want %q (%s)", engine, tc.label, outcome.Status, tc.want, outcome.Detail)
				}
			}
			// Already-correct values report without mutation.
			field := fieldByLabel(obs, "Why are you a good fit?", "textarea")
			if field.ID != "" {
				outcome, err := session.FillAnswer(ctx, browser.AnswerValue{Field: field, Value: "I build reliable systems."})
				if err != nil {
					t.Errorf("%s reread: %v", engine, err)
				} else if outcome.Status != "correct" {
					t.Errorf("%s reread: status %q, want correct", engine, outcome.Status)
				}
			}
		})
	}
}

func TestCrossEngineCoverLetter(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	fix := ControlledFormServer()
	defer func() {
		if n := fix.Close(); n != 0 {
			t.Errorf("fixture received %d submission requests", n)
		}
	}()
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			session := launchFixtureEngine(t, ctx, engine, fix)
			letter := "Dear hiring team, I am Ada and I build reliable systems."
			text, err := session.FillCoverLetter(ctx, letter)
			if err != nil {
				t.Fatalf("%s cover text: %v", engine, err)
			}
			if !text.Selected || !text.Verified {
				t.Errorf("%s cover text selected=%v verified=%v (%s)", engine, text.Selected, text.Verified, text.Detail)
			}
			// Re-entering the same text reports a match without mutation.
			same, err := session.FillCoverLetter(ctx, letter)
			if err != nil {
				t.Fatalf("%s cover text reread: %v", engine, err)
			}
			if !same.Selected || !same.Verified {
				t.Errorf("%s cover text reread selected=%v verified=%v", engine, same.Selected, same.Verified)
			}
			// A different body against the now-filled control must not overwrite.
			other, err := session.FillCoverLetter(ctx, "A completely different letter.")
			if err != nil {
				t.Fatalf("%s cover text conflict: %v", engine, err)
			}
			if other.Selected {
				t.Errorf("%s overwrote an existing cover-letter entry", engine)
			}
			// Cover file with a verified digest.
			pdf := filepath.Join(t.TempDir(), "cover.pdf")
			body := []byte("%PDF-1.4 synthetic cover letter")
			if err := os.WriteFile(pdf, body, 0600); err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256(body))
			file, err := session.AttachCoverLetter(ctx, pdf, digest)
			if err != nil {
				t.Fatalf("%s cover file: %v", engine, err)
			}
			if !file.Selected || !file.Verified {
				t.Errorf("%s cover file selected=%v verified=%v (%s)", engine, file.Selected, file.Verified, file.Detail)
			}
			// A mismatched digest never reaches the page.
			if _, err := session.AttachCoverLetter(ctx, pdf, "deadbeef"); err == nil {
				t.Errorf("%s accepted a mismatched cover-letter digest", engine)
			}
		})
	}
}

func TestCrossEngineCrashRecoveryAndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	server := ControlledFormServer()
	defer func() {
		_ = server.Close()
	}()
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			session, err := LaunchFixture(ctx, engine, server.Server.URL+"/fixture/app")
			if err != nil {
				t.Fatalf("launch %s fixture: %v", engine, err)
			}
			dir := session.dir
			if _, err := session.Observe(ctx); err != nil {
				t.Fatalf("observe %s: %v", engine, err)
			}
			// Kill the bridge child process: the browser dies with it and the
			// session must report unavailability instead of hanging.
			if err := session.cmd.Process.Kill(); err != nil {
				t.Fatalf("kill %s bridge: %v", engine, err)
			}
			if _, err := session.Observe(ctx); err == nil {
				t.Errorf("%s observe succeeded after bridge kill", engine)
			} else if !errors.Is(err, browser.ErrBrowserUnavailable) && !errors.Is(err, browser.ErrNoPageTargets) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%s unexpected error after kill: %v", engine, err)
			}
			// Close after a crash still reaps the child and removes the
			// isolated profile; double close stays quiet.
			if err := session.Close(); err != nil {
				t.Errorf("close %s after crash: %v", engine, err)
			}
			if err := session.Close(); err != nil {
				t.Errorf("second close %s: %v", engine, err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Errorf("%s profile dir survived close: %s", engine, dir)
			}
		})
	}
}

func TestCrossEngineMultiStepNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("controlled browser qualification requires real browser processes")
	}
	fix := ControlledFormServer()
	defer func() {
		if n := fix.Close(); n != 0 {
			t.Errorf("fixture received %d submission requests", n)
		}
	}()
	for _, engine := range enginesForQualification(t) {
		t.Run(engine, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			session := launchFixtureEngine(t, ctx, engine, fix)
			step2 := fix.Server.URL + "/fixture/app/application/step2"
			// Simulate the user following the employer's own Continue link.
			var moved struct {
				Href string `json:"href"`
			}
			if err := session.evaluate(ctx, "()=>{location.href="+quoteJSON(step2)+";return {href:location.href}}", nil, &moved); err != nil {
				t.Fatalf("%s navigate to step two: %v", engine, err)
			}
			var obs browser.Observation
			deadline, cancelWait := context.WithTimeout(ctx, 20*time.Second)
			defer cancelWait()
			for {
				var err error
				obs, err = session.Observe(deadline)
				if err == nil && obs.URL == step2 {
					break
				}
				select {
				case <-deadline.Done():
					t.Fatalf("%s did not reach step two: last url %q err %v", engine, obs.URL, err)
				case <-time.After(300 * time.Millisecond):
				}
			}
			field := fieldByLabel(obs, "Step two question", "textarea")
			if field.ID == "" {
				t.Fatalf("%s did not observe the second step (at %s)", engine, obs.URL)
			}
			outcome, err := session.FillAnswer(ctx, browser.AnswerValue{Field: field, Value: "Second step answer."})
			if err != nil {
				t.Fatalf("%s step-two answer: %v", engine, err)
			}
			if outcome.Status != "filled" {
				t.Errorf("%s step-two answer: status %q (%s)", engine, outcome.Status, outcome.Detail)
			}
			resume := filepath.Join(t.TempDir(), "resume.pdf")
			if err := os.WriteFile(resume, []byte("%PDF-1.4 synthetic resume"), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := session.AttachResume(ctx, resume)
			if err != nil {
				t.Fatalf("%s step-two resume: %v", engine, err)
			}
			if !result.Selected || !result.Verified {
				t.Errorf("%s step-two resume selected=%v verified=%v", engine, result.Selected, result.Verified)
			}
		})
	}
}

func quoteJSON(s string) string {
	q, _ := json.Marshal(s)
	return string(q)
}
