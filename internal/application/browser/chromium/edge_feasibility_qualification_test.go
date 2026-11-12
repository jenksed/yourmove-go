//go:build qualification

package chromium

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// TestEdgeFeasibilityWorkflow exercises the existing guarded Chromium CDP
// session against Microsoft Edge on native Windows x64. It uses only the
// production launch arguments, CDP transport, and transfer guards; Edge is
// supplied through the existing YOURMOVE_CHROMIUM override mechanism, which
// the production resolver also honors. No separate engine is introduced.
//
// Run: YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification ./internal/application/browser/chromium -run TestEdgeFeasibilityWorkflow -v -count=1
func TestEdgeFeasibilityWorkflow(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in Edge feasibility qualification")
	}
	if os.Getenv("YOURMOVE_CHROME_NO_SANDBOX") == "1" {
		t.Fatal("sandbox must stay enabled for Edge feasibility; unset YOURMOVE_CHROME_NO_SANDBOX")
	}
	edge := discoverEdge()
	if edge == "" {
		t.Fatal("Microsoft Edge was not discovered on this host; cannot qualify Edge feasibility")
	}
	t.Logf("discovered Edge executable: %s", edge)
	if out, err := exec.Command(edge, "--version").CombinedOutput(); err != nil {
		t.Fatalf("Edge version check failed: %v (%s)", err, strings.TrimSpace(string(out)))
	} else {
		t.Logf("Edge version: %s", strings.TrimSpace(string(out)))
	}
	for _, arg := range launchArguments(t.TempDir(), "http://127.0.0.1:9/application") {
		if arg == "--no-sandbox" {
			t.Fatal("production launch arguments must not disable the sandbox")
		}
		if arg == "--remote-debugging-address=127.0.0.1" {
			t.Logf("CDP debugging is scoped to loopback: %s", arg)
		}
	}
	// Force the existing session onto the discovered Edge binary through the
	// supported explicit-override path. Production keeps Chrome precedence;
	// this experiment pins Edge to prove the same transport works with it.
	t.Setenv("YOURMOVE_CHROMIUM", edge)
	if resolved, err := executable(); err != nil || resolved != edge {
		t.Fatalf("executable()=%q, %v; want the discovered Edge binary", resolved, err)
	}

	const resumeBody = "%PDF-1.4\nsynthetic Edge feasibility resume\n%%EOF"
	resumeName := "edge-feasibility-resume.pdf"
	resumePath := filepath.Join(t.TempDir(), resumeName)
	if err := os.WriteFile(resumePath, []byte(resumeBody), 0600); err != nil {
		t.Fatal(err)
	}
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resumeBody)))

	var mu sync.Mutex
	var launchPaths []string
	var uploads []struct{ name, hash, field string }
	fixture := http.NewServeMux()
	fixture.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		launchPaths = append(launchPaths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/edgejob/application":
			fmt.Fprint(w, `<html><body><h1>Synthetic application start</h1><button type="button" id="continue" onclick="location.href=location.pathname+'/next'">Continue</button></body></html>`)
		case "/edgejob/application/next":
			fmt.Fprint(w, `<html><body><h1>Second application page</h1><form id="ats">`+
				`<label>First Name* <input id="first_name" name="first_name"></label>`+
				`<label>Last Name* <input id="last_name" name="last_name"></label>`+
				`<label>Email <input name="email" type="email" value="manual@example.test"></label>`+
				`<label>Phone <input name="phone" type="tel"></label>`+
				`<section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input name="resume" type="file" accept=".pdf"></section>`+
				`<label>Why this team?<textarea name="motivation"></textarea></label>`+
				`<button id="final-submit" type="submit">Submit application</button></form>`+
				`<script>window.submitted=0;window.resumeChanges=0;`+
				`document.querySelector('form').addEventListener('submit',e=>{e.preventDefault();window.submitted++});`+
				`document.querySelector('input[name=resume]').addEventListener('change',async e=>{const f=new FormData();f.append('resume',e.target.files[0]);await fetch('/upload',{method:'POST',body:f})});`+
				`</script></body></html>`)
		default:
			fmt.Fprint(w, `<html><body><h1>Synthetic posting</h1></body></html>`)
		}
	})
	fixture.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method", 405)
			return
		}
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			http.Error(w, "multipart", 400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		for field, files := range r.MultipartForm.File {
			for _, fh := range files {
				f, err := fh.Open()
				if err != nil {
					http.Error(w, "file", 400)
					return
				}
				b, err := io.ReadAll(io.LimitReader(f, 2<<20))
				f.Close()
				if err != nil {
					http.Error(w, "read", 400)
					return
				}
				sum := sha256.Sum256(b)
				mu.Lock()
				uploads = append(uploads, struct{ name, hash, field string }{fh.Filename, fmt.Sprintf("%x", sum), field})
				mu.Unlock()
			}
		}
		w.WriteHeader(204)
	})
	srv := httptest.NewServer(fixture)
	defer srv.Close()
	posting := srv.URL + "/edgejob"
	apply := posting + "/application"
	next := apply + "/next"

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	s, err := LaunchFixture(ctx, posting)
	if err != nil {
		t.Fatalf("launch Edge session: %v", err)
	}
	profileDir := s.dir
	defer s.Close()
	if s.port <= 0 {
		t.Fatal("Edge did not expose a debugging port")
	}
	t.Logf("Edge CDP debugging port: %d (sandbox enabled, no --no-sandbox)", s.port)

	// CDP stays on localhost: the endpoint list must only offer loopback hosts.
	listResp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", s.port))
	if err != nil {
		t.Fatalf("CDP endpoint is not reachable on localhost: %v", err)
	}
	var targets []struct {
		Type         string `json:"type"`
		WebSocketURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&targets); err != nil {
		listResp.Body.Close()
		t.Fatal(err)
	}
	listResp.Body.Close()
	if len(targets) == 0 {
		t.Fatal("no CDP targets exposed")
	}
	for _, target := range targets {
		if strings.Contains(target.WebSocketURL, "://") && !strings.Contains(target.WebSocketURL, "127.0.0.1") && !strings.Contains(target.WebSocketURL, "localhost") {
			t.Fatalf("non-loopback CDP endpoint: %s", target.WebSocketURL)
		}
	}
	t.Logf("CDP target list is loopback-scoped (%d targets)", len(targets))

	deadline := time.Now().Add(30 * time.Second)
	for {
		obs, err := s.Observe(ctx)
		if err == nil && obs.URL == apply {
			t.Logf("Edge landed on the application directly: forms=%d controls=%d actions=%d", obs.Forms, len(obs.Controls), len(obs.Actions))
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Edge did not reach the application step: %v", err)
		}
		time.Sleep(90 * time.Millisecond)
	}
	mu.Lock()
	first := append([]string(nil), launchPaths...)
	mu.Unlock()
	if len(first) == 0 || first[0] != "/edgejob/application" {
		t.Fatalf("did not land on the application directly: requests=%v", first)
	}
	if result, err := s.AttachResume(ctx, resumePath); !errors.Is(err, ErrNoResumeInput) {
		t.Fatalf("first step must not accept a resume: result=%+v err=%v", result, err)
	}
	t.Log("first step correctly reports no resume input; nothing uploaded")

	// The test driver advances the synthetic employer step, as an applicant
	// would. Production never clicks Continue on the applicant's behalf.
	target, err := s.target(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialCDP(ctx, target.WebSocketURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := cdpCall(ctx, conn, 1, "Runtime.evaluate", map[string]any{"expression": "document.getElementById('continue').click()"}, nil); err != nil {
		conn.Close(websocket.StatusNormalClosure, "")
		t.Fatal(err)
	}
	conn.Close(websocket.StatusNormalClosure, "")

	deadline = time.Now().Add(30 * time.Second)
	var observed browser.Observation
	for {
		obs, err := s.Observe(ctx)
		if err == nil && obs.URL == next && obs.Forms == 1 && len(obs.Controls) >= 4 {
			observed = obs
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("second application step did not settle: %v", err)
		}
		time.Sleep(90 * time.Millisecond)
	}
	t.Logf("second step inspected through Edge: forms=%d controls=%d", observed.Forms, len(observed.Controls))

	contacts, err := s.FillContacts(ctx, browser.ContactValues{FirstName: "Casey", LastName: "Fixture", Email: "approved@example.test", Phone: "202-555-0104"})
	if err != nil || !contacts.Ready {
		t.Fatalf("approved contact transfer: %+v %v", contacts, err)
	}
	byKey := map[string]browser.ContactOutcome{}
	for _, outcome := range contacts.Fields {
		byKey[outcome.Key] = outcome
	}
	for _, key := range []string{"first_name", "last_name", "phone"} {
		if byKey[key].Status != "filled" {
			t.Fatalf("contact %s was not filled: %+v", key, contacts.Fields)
		}
	}
	if byKey["email"].Status != "manual" {
		t.Fatalf("prefilled email must be preserved, not overwritten: %+v", contacts.Fields)
	}
	t.Logf("contact outcomes: %+v", contacts.Fields)

	selected, err := s.AttachResume(ctx, resumePath)
	if err != nil || !selected.Selected || !selected.Verified {
		t.Fatalf("resume attachment: %+v %v", selected, err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		n := len(uploads)
		mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("resume bytes never reached the synthetic employer")
		}
		time.Sleep(75 * time.Millisecond)
	}
	mu.Lock()
	got := append([]struct{ name, hash, field string }(nil), uploads...)
	mu.Unlock()
	if len(got) != 1 || got[0].field != "resume" || got[0].name != resumeName || got[0].hash != expectedHash {
		t.Fatalf("wrong delivery or duplicate: %+v", got)
	}
	t.Logf("resume verified in the employer form: name=%s sha256=%s", got[0].name, got[0].hash)
	if _, err := s.AttachResume(ctx, resumePath); !errors.Is(err, ErrExistingDocument) {
		t.Fatalf("existing attachment was not preserved: %v", err)
	}

	var field schema.Field
	for _, c := range observed.Controls {
		if c.ID == "motivation" {
			field = schema.Field{ID: "browser:" + c.ID, Label: c.Label, Kind: c.Kind, Required: c.Required, Options: c.Options}
		}
	}
	if field.ID == "" {
		t.Fatalf("motivation question missing from observation: %+v", observed.Controls)
	}
	answer, err := s.FillAnswer(ctx, browser.AnswerValue{Field: field, Value: "I enjoy resolving technical support incidents."})
	if err != nil || answer.Status != "filled" {
		t.Fatalf("approved answer transfer: %+v %v", answer, err)
	}
	t.Logf("answer outcome: status=%s detail=%s", answer.Status, answer.Detail)

	var state struct {
		Result struct {
			Value struct {
				First, Last, Email, Phone, Resume, Motivation string
				Submitted, ResumeChanges                      int
			} `json:"value"`
		} `json:"result"`
	}
	err = s.FixtureCall(ctx, next, "Runtime.evaluate", map[string]any{"returnByValue": true, "expression": `({First:document.querySelector('[name=first_name]').value,Last:document.querySelector('[name=last_name]').value,Email:document.querySelector('[name=email]').value,Phone:document.querySelector('[name=phone]').value,Resume:document.querySelector('[name=resume]').files[0]?.name,Motivation:document.querySelector('[name=motivation]').value,Submitted:window.submitted,ResumeChanges:window.resumeChanges})`}, &state)
	final := state.Result.Value
	if err != nil || final.First != "Casey" || final.Last != "Fixture" || final.Email != "manual@example.test" || final.Phone != "202-555-0104" || final.Resume != resumeName || final.Motivation != "I enjoy resolving technical support incidents." || final.Submitted != 0 {
		t.Fatalf("employer form state: %+v contacts=%+v answer=%+v %v", final, contacts, answer, err)
	}
	t.Log("multi-step navigation, transfers, and no-submission verified in one Edge session")

	if err := s.Close(); err != nil {
		t.Fatalf("close Edge session: %v", err)
	}
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("temporary Edge profile was not removed: %s (err=%v)", profileDir, err)
	}
	t.Logf("Edge closed; temporary profile removed: %s", profileDir)
}
