// Package chromium owns one isolated visible Chromium process for inspection
// and explicitly approved field/document transfers. Navigation, authentication,
// sensitive disclosures, consent, and final submission stay manual.
package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/providers/ashby"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// Session-lifecycle sentinels are owned by the browser package so every engine
// reports the same conditions. Aliases preserve the existing chromium paths.
var ErrNoMatchingTab = browser.ErrNoMatchingTab
var ErrNoPageTargets = browser.ErrNoPageTargets
var ErrBrowserUnavailable = browser.ErrBrowserUnavailable

type Session struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	dir        string
	port       int
	postingURL string
	// Test-only ownership seam; production always uses exact supported posting identity.
	fixtureOwner func(string) bool
	closed       bool
}

func Launch(ctx context.Context, posting string) (*Session, error) {
	return launch(ctx, posting, false)
}

func launch(ctx context.Context, posting string, headless bool) (*Session, error) {
	if !browser.OwnsPostingURL(posting, posting) {
		return nil, errors.New("managed inspection supports saved Ashby, Greenhouse, and Lever postings")
	}
	exe, err := executable()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "yourmove-browser-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	// Use Ashby's public posting API to open its exact applyUrl on the FIRST
	// browser launch. The adapter falls back to this posting's /application
	// route if the public lookup is unavailable or returns an unsafe URL.
	// The original posting URL remains the session identity for popup/step checks.
	landing := posting
	if browser.PostingPlatform(posting) == "ashby" {
		landing, err = ashby.New(nil).ApplicationDestination(ctx, posting)
		if err != nil {
			os.RemoveAll(dir)
			return nil, fmt.Errorf("resolve Ashby application: %w", err)
		}
	} else if browser.PostingPlatform(posting) == "lever" {
		u, _ := url.Parse(posting)
		u.Path = strings.TrimSuffix(u.Path, "/") + "/apply"
		landing = u.String()
	}
	args := launchArguments(dir, landing)
	if headless {
		args = previewLaunchArguments(dir, landing)
	}
	// Root-only container testing opt-in. Never disable the sandbox by default.
	if os.Getenv("YOURMOVE_CHROME_NO_SANDBOX") == "1" {
		args = append(args, "--no-sandbox")
	}
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("start Chromium: %w", err)
	}
	s := &Session{cmd: cmd, dir: dir, postingURL: posting}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		b, e := os.ReadFile(filepath.Join(dir, "DevToolsActivePort"))
		if e == nil {
			first, _, _ := strings.Cut(string(b), "\n")
			port, err := strconv.Atoi(strings.TrimSpace(first))
			if err == nil && port > 0 && port < 65536 {
				s.port = port
				return s, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, errors.Join(ctx.Err(), s.Close())
		case <-deadline.C:
			return nil, errors.Join(errors.New("Chromium did not expose a debugging port; verify it can launch"), s.Close())
		case <-tick.C:
		}
	}
}

// Preview opens one public Ashby application route in an isolated temporary
// Chromium profile, observes its current DOM once, and closes only that process.
// It does not join the managed application watcher or expose mutation methods.
func Preview(ctx context.Context, posting string) (result schema.Result, err error) {
	if browser.PostingPlatform(posting) != "ashby" || !browser.OwnsPostingURL(posting, posting) {
		return schema.Result{}, errors.New("read-only preview supports only the saved Ashby posting")
	}
	bounded, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	session, err := launch(bounded, posting, true)
	if err != nil {
		return schema.Result{}, err
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close read-only preview: %w", closeErr))
			result = schema.Result{}
		}
	}()
	observation, err := observePreviewPage(bounded, session)
	if err != nil {
		return schema.Result{}, err
	}
	return browser.AsPreviewResult(posting, observation)
}

func observePreviewPage(ctx context.Context, session *Session) (browser.Observation, error) {
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	started := time.Now()
	var last browser.Observation
	haveSnapshot := false
	for {
		observation, err := session.Observe(bounded)
		if err != nil {
			if haveSnapshot && errors.Is(err, context.DeadlineExceeded) {
				if contextErr := ctx.Err(); contextErr != nil {
					return browser.Observation{}, contextErr
				}
				return last, nil
			}
			return browser.Observation{}, err
		}
		last, haveSnapshot = observation, true
		if observation.ReadyState == "complete" && (len(observation.Controls) > 0 || observation.Forms > 0 || len(observation.Frames) > 0 || time.Since(started) >= 5*time.Second) {
			return observation, nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-bounded.Done():
			timer.Stop()
			if err := ctx.Err(); err != nil {
				return browser.Observation{}, err
			}
			return last, nil
		case <-timer.C:
		}
	}
}

// launchArguments is deliberately small and independently tested: Chromium
// must start on the application page, never the job description page.
func launchArguments(profileDir, applicationURL string) []string {
	return []string{"--remote-debugging-port=0", "--remote-debugging-address=127.0.0.1", "--user-data-dir=" + profileDir, "--no-first-run", "--no-default-browser-check", "--disable-sync", "--window-size=1150,850", "--app=" + applicationURL}
}

func previewLaunchArguments(profileDir, applicationURL string) []string {
	return append(launchArguments(profileDir, applicationURL), "--headless=new")
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	return os.RemoveAll(s.dir)
}

type target struct {
	Type         string `json:"type"`
	URL          string `json:"url"`
	WebSocketURL string `json:"webSocketDebuggerUrl"`
}

func (s *Session) target(ctx context.Context) (target, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/json/list", s.port), nil)
	if err != nil {
		return target{}, err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return target{}, fmt.Errorf("%w: %v", ErrBrowserUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return target{}, fmt.Errorf("browser target list HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 128<<10))
	if err != nil {
		return target{}, err
	}
	var targets []target
	if err := json.Unmarshal(body, &targets); err != nil {
		return target{}, err
	}
	if s.fixtureOwner != nil {
		return selectTargetOwned(s.port, targets, s.fixtureOwner)
	}
	return selectTarget(s.postingURL, s.port, targets)

}

// selectTarget accepts CDP-provided localhost and loopback websocket hosts,
// then always normalizes the actual connection to loopback. Prefer a matching
// application popup to its still-open job posting, without trusting any other job.
func selectTarget(saved string, port int, targets []target) (target, error) {
	return selectTargetOwned(port, targets, func(candidate string) bool {
		return browser.OwnsPostingURL(saved, candidate)
	})
}

// Strict host/posting verification is the default. Synthetic tests alone may
// supply an ownership predicate on a private Session to exercise loopback fixtures.
func selectTargetOwned(port int, targets []target, owns func(string) bool) (target, error) {
	var fallback target
	hasPage := false
	expected := fmt.Sprintf("127.0.0.1:%d", port)
	alternate := fmt.Sprintf("localhost:%d", port)
	for _, t := range targets {
		if t.Type == "page" {
			hasPage = true
		}
		if t.Type != "page" || !owns(t.URL) {
			continue
		}
		u, err := url.Parse(t.WebSocketURL)
		if err != nil || u.Scheme != "ws" || (u.Host != expected && u.Host != alternate) || !strings.HasPrefix(u.Path, "/devtools/page/") || u.User != nil {
			continue
		}
		u.Host = expected
		t.WebSocketURL = u.String()
		if strings.Contains(t.URL, "/application") || strings.Contains(t.URL, "/apply") {
			return t, nil
		}
		if fallback.URL == "" {
			fallback = t
		}
	}
	if fallback.URL != "" {
		return fallback, nil
	}
	if !hasPage {
		return target{}, ErrNoPageTargets
	}
	return target{}, ErrNoMatchingTab
}

func (s *Session) Observe(ctx context.Context) (browser.Observation, error) {
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if err := s.lockContext(c); err != nil {
		return browser.Observation{}, err
	}
	defer s.mu.Unlock()
	if s.closed {
		return browser.Observation{}, errors.New("inspection browser is closed")
	}

	t, err := s.target(c)
	if err != nil {
		return browser.Observation{}, err
	}
	ws, err := dialCDP(c, t.WebSocketURL)
	if err != nil {
		return browser.Observation{}, fmt.Errorf("connect to local inspection tab: %w", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	request := map[string]any{"id": 1, "method": "Runtime.evaluate", "params": map[string]any{"expression": browser.ReadOnlyScript, "returnByValue": true, "awaitPromise": false, "silent": true}}
	payload, _ := json.Marshal(request)
	if err := ws.Write(c, websocket.MessageText, payload); err != nil {
		return browser.Observation{}, err
	}
	for {
		_, data, err := ws.Read(c)
		if err != nil {
			return browser.Observation{}, fmt.Errorf("read inspection response: %w", err)
		}
		if int64(len(data)) > maxCDPMessageBytes {
			return browser.Observation{}, errors.New("inspection response too large")
		}
		var response struct {
			ID    int `json:"id"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Result struct {
				Exception json.RawMessage `json:"exceptionDetails"`
				Result    struct {
					Type  string          `json:"type"`
					Value json.RawMessage `json:"value"`
				} `json:"result"`
			} `json:"result"`
		}
		if json.Unmarshal(data, &response) != nil || response.ID != 1 {
			continue
		}
		if response.Error != nil {
			return browser.Observation{}, fmt.Errorf("browser inspection error: %s", response.Error.Message)
		}
		if len(response.Result.Exception) > 0 {
			return browser.Observation{}, errors.New("browser page rejected read-only inspection")
		}
		if len(response.Result.Result.Value) == 0 {
			return browser.Observation{}, errors.New("browser page is not ready for DOM inspection")
		}
		var snapshot browser.Observation
		if err := json.Unmarshal(response.Result.Result.Value, &snapshot); err != nil {
			return browser.Observation{}, err
		}
		owns := func(candidate string) bool { return browser.OwnsPostingURL(s.postingURL, candidate) }
		if s.fixtureOwner != nil {
			owns = s.fixtureOwner
		}
		// Chromium can expose the owned destination target a moment before its
		// first document replaces about:blank. The target itself was ownership
		// checked by s.target; the bounded preview readiness loop will wait for
		// the document URL before producing a result.
		if snapshot.URL == "about:blank" && owns(t.URL) {
			return snapshot, nil
		}
		if !owns(snapshot.URL) {
			return browser.Observation{}, fmt.Errorf("current browser page changed to another job or origin (%s); nothing saved", safeObservedAddress(snapshot.URL))
		}
		return snapshot, nil
	}
}

func safeObservedAddress(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "unknown address"
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// lockContext serializes live observation and mutation without making callers
// wait past cancellation. A canceled operation never acquires write authority.
func (s *Session) lockContext(ctx context.Context) error {
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for !s.mu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	return nil
}
