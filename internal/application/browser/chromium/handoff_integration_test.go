package chromium

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
)

// Run explicitly: YOURMOVE_QUALIFY_BROWSER=1 go test ./internal/application/browser/chromium -run TestControlledApplicationHandoff -v -count=1
// This is a loopback-only synthetic form, NEVER a real employer or candidate packet.
func TestControlledApplicationHandoff(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in real Chromium qualification")
	}
	exe, err := executable()
	if err != nil {
		t.Fatalf("interactive Chromium unavailable (required for opt-in qualification): %v", err)
	}
	const resumeBody = "%PDF-1.4\nsynthetic qualification document\n%%EOF"
	name := "controlled-resume.pdf"
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(resumeBody), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(resumeBody))
	expectedHash := hex.EncodeToString(sum[:])

	var mu sync.Mutex
	var launchPaths []string
	var uploads []struct{ name, hash, field string }
	const base = "/render/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	fixture := http.NewServeMux()
	fixture.HandleFunc("/render/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		launchPaths = append(launchPaths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == base+"/application" {
			fmt.Fprint(w, `<html><body><h1>Ashby-shaped synthetic application</h1><form id="step1"><label>Email <input name="email"></label><button type="button" id="continue" onclick="location.href=location.pathname+'/next'">Continue</button></form></body></html>`)
			return
		}
		if r.URL.Path == base+"/application/next" {
			fmt.Fprint(w, `<html><body><h1>Second application page</h1><main id="mount"></main><script>
            setTimeout(()=>{
              document.getElementById('mount').innerHTML = '<form id="step2"><section data-field-entry-id="resume"><div class="ashby-application-form-question-title">Resume</div><input name="resume" type="file" accept=".pdf" /></section><section data-field-entry-id="cover"><div class="ashby-application-form-question-title">Cover Letter</div><input name="cover" type="file" /></section><div class="ashby-application-form-autofill-input-root"><input type="file" name="parser" /></div><button id="final-submit" type="submit">Submit application</button></form>';
              document.getElementById('step2').addEventListener('submit',e=>{e.preventDefault();document.body.dataset.submitted='yes'});
              document.querySelector('input[name=resume]').addEventListener('change', async e=>{const f=new FormData();f.append('resume',e.target.files[0]); await fetch('/upload',{method:'POST',body:f})});
            }, 180);
            </script></body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body><h1>Job description</h1></body></html>`)
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
				s := sha256.Sum256(b)
				mu.Lock()
				uploads = append(uploads, struct{ name, hash, field string }{fh.Filename, hex.EncodeToString(s[:]), field})
				mu.Unlock()
			}
		}
		w.WriteHeader(204)
	})
	srv := httptest.NewServer(fixture)
	defer srv.Close()
	posting := srv.URL + base
	apply := posting + "/application"
	// Test-owned host boundary: only this loopback fixture, never another port.
	owns := func(candidate string) bool {
		return candidate == posting || candidate == apply || candidate == apply+"/next"
	}
	dir := t.TempDir()
	args := launchArguments(dir, apply)
	args = append(args, "--headless=new", "--disable-gpu", "--disable-background-networking")
	if os.Geteuid() == 0 {
		args = append(args, "--no-sandbox")
	}
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("launch controlled Chromium: %v", err)
	}
	s := &Session{
		cmd: cmd, dir: dir, postingURL: posting, fixtureOwner: owns,
		fixtureContactOrigin: srv.URL, fixtureContactBase: base, fixtureContactPlatform: "ashby",
	}
	defer s.Close()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(filepath.Join(dir, "DevToolsActivePort"))
		if err == nil {
			var port int
			fmt.Sscanf(string(b), "%d", &port)
			if port > 0 {
				s.port = port
				break
			}
		}
		time.Sleep(80 * time.Millisecond)
	}
	if s.port == 0 {
		t.Fatal("Chromium DevTools port unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for time.Now().Before(deadline) {
		obs, err := s.Observe(ctx)
		if err == nil && obs.URL == apply {
			break
		}
		time.Sleep(90 * time.Millisecond)
	}
	mu.Lock()
	first := append([]string(nil), launchPaths...)
	mu.Unlock()
	if len(first) == 0 || first[0] != base+"/application" {
		t.Fatalf("did not land on application directly: requests=%v", first)
	}
	if result, err := s.AttachResume(ctx, path); !errors.Is(err, ErrNoResumeInput) {
		t.Fatalf("first step should not upload: result=%+v err=%v", result, err)
	}
	// A test driver advances the synthetic employer step. Production never clicks Continue.
	target, err := s.target(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dialCDP(ctx, target.WebSocketURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := cdpCall(ctx, conn, 1, "Runtime.evaluate", map[string]any{"expression": "document.getElementById('continue').click()"}, nil); err != nil {
		t.Fatal(err)
	}
	conn.Close(websocket.StatusNormalClosure, "")
	// Wait for the next step AND its delayed controls. The same session owns it.
	deadline = time.Now().Add(10 * time.Second)
	var selected AttachmentResult
	for time.Now().Before(deadline) {
		result, e := s.AttachResume(ctx, path)
		if e == nil {
			selected = result
			break
		}
		if errors.Is(e, ErrNoResumeInput) || strings.Contains(e.Error(), "resume input disappeared") {
			time.Sleep(160 * time.Millisecond)
			continue
		}
		t.Fatalf("step 2 attachment failed: %v", e)
	}
	if !selected.Selected {
		t.Fatal("file was never selected into the second-step resume input")
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(uploads)
		mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(75 * time.Millisecond)
	}
	mu.Lock()
	got := append([]struct{ name, hash, field string }(nil), uploads...)
	mu.Unlock()
	if len(got) != 1 || got[0].field != "resume" || got[0].name != name || got[0].hash != expectedHash {
		t.Fatalf("wrong delivery or duplicate: %+v", got)
	}
	// Repeating the operation must preserve the existing selected file, not upload again.
	if _, err := s.AttachResume(ctx, path); err == nil || !strings.Contains(err.Error(), "already attached") {
		t.Fatalf("second attachment was not blocked: %v", err)
	}
	mu.Lock()
	count := len(uploads)
	mu.Unlock()
	if count != 1 {
		t.Fatalf("unexpected repeated upload: %d", count)
	}
	after, err := s.target(ctx)
	if err != nil {
		t.Fatal(err)
	}
	verify, err := dialCDP(ctx, after.WebSocketURL)
	if err != nil {
		t.Fatal(err)
	}
	defer verify.Close(websocket.StatusNormalClosure, "")
	var state struct {
		Cover     bool `json:"cover"`
		Submitted bool `json:"submitted"`
	}
	if err := evaluateRead(ctx, verify, 44, `({cover:document.querySelector('input[name=cover]').files.length>0,submitted:document.body.dataset.submitted==='yes'})`, &state); err != nil {
		t.Fatal(err)
	}
	if state.Cover || state.Submitted {
		t.Fatalf("cover letter touched or final submission triggered: %+v", state)
	}
	// Increment B: continue in the same session, touching only the cover control.
	letterPath := filepath.Join(t.TempDir(), "controlled-letter.pdf")
	letterBytes := []byte("%PDF-1.4\ncontrolled saved cover letter\n%%EOF")
	if err := os.WriteFile(letterPath, letterBytes, 0600); err != nil {
		t.Fatal(err)
	}
	letterHash := fmt.Sprintf("%x", sha256.Sum256(letterBytes))
	began := time.Now()
	letter, err := s.AttachCoverLetter(ctx, letterPath, letterHash)
	if err != nil || !letter.Selected || !letter.Verified {
		t.Fatalf("cover transfer: %+v %v", letter, err)
	}
	if _, err := s.AttachCoverLetter(ctx, letterPath, letterHash); err == nil {
		t.Fatal("duplicate cover selection allowed")
	}
	var continuation struct {
		Resume, Cover string
		Submitted     bool
	}
	if err := evaluateRead(ctx, verify, 45, `({Resume:document.querySelector('input[name=resume]').files[0]?.name,Cover:document.querySelector('input[name=cover]').files[0]?.name,Submitted:document.body.dataset.submitted==='yes'})`, &continuation); err != nil {
		t.Fatal(err)
	}
	if continuation.Resume != name || continuation.Cover != "controlled-letter.pdf" || continuation.Submitted {
		t.Fatalf("continuation damaged: %+v", continuation)
	}
	// Exact text transfer is an explicit alternative, never an automatic retry of
	// an uncertain upload. The fixture replaces just the cover control.
	if err := cdpCall(ctx, verify, 46, "Runtime.evaluate", map[string]any{"expression": `document.querySelector('[data-field-entry-id=cover]').innerHTML='<label>Cover Letter<textarea name="cover-text"></textarea></label>';document.querySelector('textarea').addEventListener('input',()=>{const old=document.querySelector('textarea');const next=old.cloneNode();next.value=old.value;old.replaceWith(next)})`}, nil); err != nil {
		t.Fatal(err)
	}
	const savedText = "Dear Hiring Team,\n\nThis exact saved letter includes Unicode: 世界."
	textResult, err := s.FillCoverLetter(ctx, savedText)
	if err != nil || !textResult.Verified {
		t.Fatalf("rerender text transfer: %+v %v", textResult, err)
	}
	if err := cdpCall(ctx, verify, 47, "Runtime.evaluate", map[string]any{"expression": `document.querySelector('textarea').value='My manual correction'`}, nil); err != nil {
		t.Fatal(err)
	}
	preserved, err := s.FillCoverLetter(ctx, savedText)
	if err != nil || preserved.Selected {
		t.Fatalf("manual letter overwritten: %+v %v", preserved, err)
	}
	var final struct {
		Text, Resume string
		Submitted    bool
	}
	if err := evaluateRead(ctx, verify, 48, `({Text:document.querySelector('textarea').value,Resume:document.querySelector('input[name=resume]').files[0]?.name,Submitted:document.body.dataset.submitted==='yes'})`, &final); err != nil {
		t.Fatal(err)
	}
	if final.Text != "My manual correction" || final.Resume != name || final.Submitted {
		t.Fatalf("final state: %+v", final)
	}
	t.Logf("Cover PDF + rerendered Unicode text + manual preservation: %s; one resume selection, one cover selection, zero submissions", time.Since(began))

}

// Explicit read-only live qualification. It never receives candidate values or
// calls a mutation. A URL must be supplied for each invocation.
func TestLiveContactStructure(t *testing.T) {
	raw := os.Getenv("YOURMOVE_LIVE_OBSERVE_URL")
	if raw == "" {
		t.Skip("explicit live read-only URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	s, err := Launch(ctx, strings.TrimRight(raw, "/"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	deadline := time.Now().Add(22 * time.Second)
	for time.Now().Before(deadline) {
		observation, err := s.Observe(ctx)
		if err == nil && len(observation.Controls) > 0 {
			t.Logf("Read-only live form: URL=%s forms=%d controls=%+v", observation.URL, observation.Forms, observation.Controls)
			target, err := s.target(ctx)
			if err != nil {
				t.Fatal(err)
			}
			ws, err := dialCDP(ctx, target.WebSocketURL)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close(websocket.StatusNormalClosure, "")
			var structure any
			err = evaluateRead(ctx, ws, 1, `[...document.querySelectorAll('input')].map(e=>({type:e.type,name:e.name,id:e.id,autocomplete:e.autocomplete,parents:[e.parentElement,e.parentElement?.parentElement,e.parentElement?.parentElement?.parentElement,e.closest('[class*=application-form]')].filter(Boolean).map(p=>({tag:p.tagName,class:p.className,field:p.getAttribute('data-field-entry-id')}))}))`, &structure)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Read-only contact structure: %+v", structure)
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	t.Fatal("live application controls were not observed")
}
