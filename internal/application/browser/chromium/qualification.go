//go:build qualification

package chromium

// This loopback-only harness is absent from production builds. It lets the
// ordinary Go UI/handler tests exercise the existing CDP session, not a fake
// filler or a second browser implementation.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func fixtureURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost") && u.User == nil
}

func LaunchFixture(ctx context.Context, posting string) (*Session, error) {
	return launchFixturePage(ctx, posting, posting+"/application")
}

func launchFixturePage(ctx context.Context, posting, page string) (*Session, error) {
	if !fixtureURL(posting) {
		return nil, errors.New("qualification accepts loopback fixtures only")
	}
	if !fixtureURL(page) || (page != posting && !strings.HasPrefix(page, posting+"/")) {
		return nil, errors.New("qualification page must stay under its loopback fixture")
	}
	exe, err := executable()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "yourmove-contact-fixture-")
	if err != nil {
		return nil, err
	}
	args := append(launchArguments(dir, page), "--headless=new", "--disable-gpu", "--disable-background-networking")
	if os.Geteuid() == 0 {
		args = append(args, "--no-sandbox")
	}
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	s := &Session{cmd: cmd, dir: dir, postingURL: posting, fixtureOwner: func(candidate string) bool {
		return candidate == posting || strings.HasPrefix(candidate, posting+"/")
	}}
	c, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "DevToolsActivePort")); err == nil {
			if _, err := fmt.Sscanf(string(data), "%d", &s.port); err == nil && s.port > 0 {
				return s, nil
			}
		}
		select {
		case <-c.Done():
			s.Close()
			return nil, c.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// LaunchFixtureForPosting models a loopback form as a same-origin route below
// a synthetic supported posting. It changes only the browser-side route-policy
// inputs used by the qualification fixture; production launch has no override.
func LaunchFixtureForPosting(ctx context.Context, fixturePosting, savedPosting string) (*Session, error) {
	platform := browser.PostingPlatform(savedPosting)
	if platform == "" || !browser.OwnsPostingURL(savedPosting, savedPosting) {
		return nil, errors.New("qualification requires an exact supported posting identity")
	}
	fixture, err := url.Parse(fixturePosting)
	if err != nil || !fixtureURL(fixturePosting) || fixture.Path == "" {
		return nil, errors.New("qualification accepts loopback fixtures only")
	}
	page := fixturePosting
	if platform == "ashby" {
		page += "/application"
	} else if platform == "lever" {
		page += "/apply"
	}
	session, err := launchFixturePage(ctx, fixturePosting, page)
	if err != nil {
		return nil, err
	}
	session.fixtureContactOrigin = fixture.Scheme + "://" + fixture.Host
	session.fixtureContactBase = strings.TrimSuffix(fixture.Path, "/")
	session.fixtureContactPlatform = platform
	return session, nil
}

// PreviewFixture exercises the production read-only DOM projection against a
// local page while preserving a synthetic Ashby posting identity. It is only
// available in qualification builds and removes its temporary profile before
// returning.
func PreviewFixture(ctx context.Context, savedPosting, fixturePosting string) (schema.Result, error) {
	if browser.PostingPlatform(savedPosting) != "ashby" || !browser.OwnsPostingURL(savedPosting, savedPosting) {
		return schema.Result{}, errors.New("qualification requires an exact synthetic Ashby posting")
	}
	session, err := LaunchFixture(ctx, fixturePosting)
	if err != nil {
		return schema.Result{}, err
	}
	dir := session.dir
	observation, observeErr := observePreviewPage(ctx, session)
	closeErr := session.Close()
	if closeErr != nil {
		return schema.Result{}, fmt.Errorf("close controlled preview: %w", closeErr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return schema.Result{}, errors.New("qualification preview profile was not removed")
	}
	if observeErr != nil {
		return schema.Result{}, fmt.Errorf("observe controlled preview fixture: %w", observeErr)
	}
	// The fixture served at loopback is a controlled surrogate for this exact
	// saved Ashby posting; no network request is made to a real employer.
	observation.URL = strings.TrimSuffix(savedPosting, "/") + "/application"
	return browser.AsPreviewResult(savedPosting, observation)
}

// FixtureCall drives only explicitly named local fixture tabs. Production has
// no exported arbitrary-CDP entry point. This also captures visual test evidence.
func (s *Session) FixtureCall(ctx context.Context, page, method string, params, out any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, err := s.fixtureConnection(ctx, page)
	if err != nil {
		return err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	return cdpCall(ctx, ws, 1, method, params, out)
}

// FixturePage holds one local fixture tab's DevTools connection for an
// interactive qualification. Keeping the connection open preserves viewport
// emulation and pointer state across the steps of a real-browser workflow.
func (s *Session) FixturePage(ctx context.Context, page string) (*FixturePage, error) {
	ws, err := s.fixtureConnection(ctx, page)
	if err != nil {
		return nil, err
	}
	return &FixturePage{ws: ws}, nil
}

type FixturePage struct {
	mu sync.Mutex
	ws *websocket.Conn
}

func (p *FixturePage) Call(ctx context.Context, method string, params, out any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ws == nil {
		return errors.New("fixture page is closed")
	}
	return cdpCall(ctx, p.ws, 1, method, params, out)
}

func (p *FixturePage) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ws == nil {
		return nil
	}
	ws := p.ws
	p.ws = nil
	return ws.Close(websocket.StatusNormalClosure, "")
}

// Fixture navigation and backend writes finish independently. Wait only for
// exact read-only target discovery; never replay the requested CDP operation.
func (s *Session) fixtureConnection(ctx context.Context, page string) (*websocket.Conn, error) {
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		ws, err := s.fixtureConnectionOnce(bounded, page)
		if err == nil || (!errors.Is(err, ErrNoMatchingTab) && !errors.Is(err, ErrNoPageTargets)) {
			return ws, err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-bounded.Done():
			timer.Stop()
			return nil, fmt.Errorf("fixture navigation did not reach its exact target: %w", err)
		case <-timer.C:
		}
	}
}

func (s *Session) fixtureConnectionOnce(ctx context.Context, page string) (*websocket.Conn, error) {
	if !fixtureURL(page) {
		return nil, errors.New("qualification target must be a loopback fixture")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/json/list", s.port), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var targets []target
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}
	t, err := selectTargetOwned(s.port, targets, func(candidate string) bool {
		return strings.Split(candidate, "#")[0] == strings.Split(page, "#")[0]
	})
	if err != nil {
		var observed []string
		for _, target := range targets {
			if fixtureURL(target.URL) {
				observed = append(observed, target.URL)
			}
		}
		return nil, fmt.Errorf("%w: expected loopback fixture %q, observed %q", err, page, observed)
	}
	return dialCDP(ctx, t.WebSocketURL)
}

// Keep emulation and capture on the same CDP connection: Chrome drops viewport
// overrides on disconnect. Reused by responsive preparation fixture checks.
func (s *Session) FixtureScreenshot(ctx context.Context, page string, width int) (string, error) {
	return s.FixtureScreenshotAt(ctx, page, width, ".prepare-handoff", "")
}

// FixtureScreenshotAt captures a real local UI at a named editing surface.
func (s *Session) FixtureScreenshotAt(ctx context.Context, page string, width int, selector, theme string) (string, error) {
	return s.FixtureScreenshotViewport(ctx, page, width, 900, selector, theme)
}

// FixtureScreenshotViewport captures a real local UI at an exact viewport size
// and a named editing surface. Keep the height explicit for desktop layout QA.
func (s *Session) FixtureScreenshotViewport(ctx context.Context, page string, width, height int, selector, theme string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if width < 1 || height < 1 {
		return "", fmt.Errorf("invalid fixture viewport %dx%d", width, height)
	}
	ws, err := s.fixtureConnection(ctx, page)
	if err != nil {
		return "", err
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	if err := cdpCall(ctx, ws, 1, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		return "", err
	}
	var overflow bool
	selection, _ := json.Marshal(selector)
	appearance, _ := json.Marshal(theme)
	if err := evaluateRead(ctx, ws, 2, `(()=>{document.documentElement.style.scrollBehavior='auto';if (`+string(appearance)+`) document.documentElement.dataset.theme=`+string(appearance)+`;document.querySelector(`+string(selection)+`).scrollIntoView({behavior:'instant',block:'start'});window.scrollBy(0,-18);document.getAnimations().forEach(animation=>animation.finish());return document.documentElement.scrollWidth>innerWidth})()`, &overflow); err != nil || overflow {
		var elements []string
		if overflow {
			_ = evaluateRead(ctx, ws, 4, `[...document.querySelectorAll('.prepare-workspace *')].filter(e=>e.getBoundingClientRect().right>innerWidth).slice(0,8).map(e=>e.tagName+'.'+e.className+' width='+Math.round(e.getBoundingClientRect().width))`, &elements)
		}
		return "", fmt.Errorf("fixture layout overflow=%v elements=%v: %v", overflow, elements, err)
	}
	var shot struct {
		Data string `json:"data"`
	}
	err = cdpCall(ctx, ws, 3, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &shot)
	return shot.Data, err
}
