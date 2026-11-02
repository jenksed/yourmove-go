//go:build qualification

package app

// Cross-engine application workflow qualification. The actual HTTP workflow
// (open browser, observe, authorize resume/contacts/answers, transfer, stop)
// runs against a real managed browser per engine. The employer form is a
// controlled loopback fixture; nothing here touches a live employer site.
import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/playwright"
)

// playwrightFixtureSession maps the loopback fixture identity onto the saved
// posting identity so session ownership and observation conversion see the
// employer's own origin, mirroring the Chromium fixture-session pattern.
type playwrightFixtureSession struct {
	*playwright.Session
	fixture, saved string
}

func (s *playwrightFixtureSession) Observe(ctx context.Context) (browser.Observation, error) {
	o, err := s.Session.Observe(ctx)
	if err == nil {
		o.URL = strings.Replace(o.URL, s.fixture, s.saved, 1)
	}
	return o, err
}

func playwrightWorkflowEngines(t *testing.T) []string {
	t.Helper()
	if only := os.Getenv("YOURMOVE_QUAL_ENGINE"); only != "" {
		if !playwright.ValidEngine(only) {
			t.Fatalf("YOURMOVE_QUAL_ENGINE=%q is not a supported engine", only)
		}
		return []string{only}
	}
	return []string{playwright.EngineChromium, playwright.EngineFirefox, playwright.EngineWebKit}
}

func TestPlaywrightEngineApplicationWorkflow(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled cross-engine qualification")
	}
	for _, engine := range playwrightWorkflowEngines(t) {
		t.Run(engine, func(t *testing.T) {
			fix := playwright.ControlledFormServer()
			defer func() {
				if n := fix.Close(); n != 0 {
					t.Errorf("%s fixture received %d submission requests", engine, n)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			base := fix.Server.URL + "/fixture/app"
			session, err := playwright.LaunchFixture(ctx, engine, base)
			if err != nil {
				t.Fatalf("launch %s: %v", engine, err)
			}
			wrapped := &playwrightFixtureSession{Session: session, fixture: base, saved: testAshbyPosting}
			a, _ := testWorkspace(t)
			requireQualificationStorage(t, a)
			if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic " + engine + " workflow", Description: "Support customers through production incidents"}); err != nil {
				t.Fatal(err)
			}
			resume, err := a.importResume("approved-resume.pdf", strings.NewReader("%PDF-1.4 synthetic resume"))
			if err != nil {
				t.Fatal(err)
			}
			if err := a.assignResume(1, resume.ID); err != nil {
				t.Fatal(err)
			}
			profile := CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Phone: "202-555-0104", Approved: true}
			if err := a.saveCandidateProfile(profile); err != nil {
				t.Fatal(err)
			}
			savedProfile, err := a.candidateProfile()
			if err != nil {
				t.Fatal(err)
			}
			a.browsers.pollEvery = 200 * time.Millisecond
			a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return wrapped, nil }
			defer a.browsers.closeAll()
			server := httptest.NewServer(newMux(a))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			post := func(path string, form url.Values) *http.Response {
				t.Helper()
				resp, err := client.PostForm(server.URL+path, form)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				_, _ = io.Copy(io.Discard, resp.Body)
				return resp
			}
			status := func() browserRunState {
				t.Helper()
				resp, err := client.Get(server.URL + "/jobs/1/browser-status")
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				var state browserRunState
				if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
					t.Fatal(err)
				}
				return state
			}
			waitState := func(description string, fn func(browserRunState) bool) browserRunState {
				t.Helper()
				deadline := time.Now().Add(60 * time.Second)
				for time.Now().Before(deadline) {
					state := status()
					if fn(state) {
						return state
					}
					time.Sleep(300 * time.Millisecond)
				}
				t.Fatalf("timed out: %s; last=%+v", description, status())
				return browserRunState{}
			}

			// 1. Open the managed employer browser with the requested engine.
			resp := post("/jobs/1/browser/start", url.Values{"engine": {engine}})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("browser start status %d", resp.StatusCode)
			}
			state := waitState("browser active", func(s browserRunState) bool { return s.Active })
			if state.Engine != engine {
				t.Fatalf("session engine %q, want %q", state.Engine, engine)
			}

			// 2. Authorize resume + contacts through the real handoff command.
			job, err := a.jobByID(1)
			if err != nil {
				t.Fatal(err)
			}
			handoff := url.Values{
				"destination":       {job.URL},
				"resume_id":         {itoa(resume.ID)},
				"resume_hash":       {resume.SHA256},
				"transfer_contacts": {"yes"},
				"profile_revision":  {itoa(savedProfile.Revision)},
			}
			resp = post("/jobs/1/handoff", handoff)
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("handoff status %d", resp.StatusCode)
			}
			state = waitState("contacts complete and resume selected", func(s browserRunState) bool {
				return s.ContactStatus == "complete" && s.ResumeStatus == "selected"
			})
			if got := status().Engine; got != engine {
				t.Fatalf("status engine %q, want %q", got, engine)
			}

			// 3. Observe, save one live answer, and transfer it.
			resp = post("/jobs/1/browser/observe", url.Values{"return_to": {"prepare"}})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("observe status %d", resp.StatusCode)
			}
			prepared, err := a.prepareQuestions(job, savedProfile)
			if err != nil {
				t.Fatal(err)
			}
			var essay *preparedQuestion
			for i := range prepared.Questions {
				if prepared.Questions[i].Field.Label == "Why are you a good fit?" {
					essay = &prepared.Questions[i]
				}
			}
			if essay == nil {
				t.Fatalf("no observed essay question (saw %d)", len(prepared.Questions))
			}
			resp = post("/jobs/1/answers/save", url.Values{
				"question_key": {essay.Key},
				"revision":     {itoa(essay.Revision)},
				"destination":  {job.URL},
				"answer":       {"I build reliable systems."},
			})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("answer save status %d", resp.StatusCode)
			}
			profile, err = a.candidateProfile()
			if err != nil {
				t.Fatal(err)
			}
			prepared, err = a.prepareQuestions(job, profile)
			if err != nil {
				t.Fatal(err)
			}
			// The Prepare screen authorizes only pending answers: already
			// transferred contacts are excluded, exactly as the UI computes.
			transferScope := questionPreparation{Questions: pendingAnswers(prepared.ReadyAnswers(), status().AnswerAttempted, status().ContactStatus != "")}
			resp = post("/jobs/1/answers/transfer", url.Values{
				"review_hash": {transferScope.ReviewHash()},
				"destination": {job.URL},
			})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("answer transfer status %d", resp.StatusCode)
			}
			state = waitState("answers complete", func(s browserRunState) bool { return s.AnswerStatus == "complete" })
			filled := 0
			for _, outcome := range state.AnswerOutcomes {
				if outcome.Label == "Why are you a good fit?" && outcome.Status == "filled" {
					filled++
				}
				if outcome.Status == "uncertain" {
					t.Errorf("%s uncertain outcome: %s (%s)", engine, outcome.Label, outcome.Detail)
				}
			}
			if filled != 1 {
				t.Errorf("%s essay outcome not filled (%+v)", engine, state.AnswerOutcomes)
			}

			// 4. No silent engine switch: a different explicit engine is rejected.
			other := playwright.EngineChromium
			if engine == playwright.EngineChromium {
				other = playwright.EngineFirefox
			}
			resp = post("/jobs/1/browser/start", url.Values{"engine": {other}})
			if resp.StatusCode != http.StatusConflict {
				t.Errorf("%s engine-switch start status %d, want 409", engine, resp.StatusCode)
			}

			// 5. Restart keeps the engine; stop closes the session and profile.
			resp = post("/jobs/1/browser/restart", url.Values{"confirm_restart": {"yes"}, "engine": {engine}})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("restart status %d", resp.StatusCode)
			}
			state = waitState("browser active after restart", func(s browserRunState) bool { return s.Active })
			if state.Engine != engine {
				t.Fatalf("restart engine %q, want %q", state.Engine, engine)
			}
			resp = post("/jobs/1/browser/stop", url.Values{})
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("stop status %d", resp.StatusCode)
			}
			if current := status(); current.Active {
				t.Fatalf("%s session still active after stop", engine)
			}
		})
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
