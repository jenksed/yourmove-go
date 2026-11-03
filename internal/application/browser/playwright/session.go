// Package playwright drives Chromium, Firefox, and WebKit through the
// official Playwright runtime behind a narrowly scoped Go-managed Node.js
// bridge. Go owns authorization, posting ownership, session lifecycle, and
// outcome reporting; the bridge only launches one visible managed browser,
// evaluates authorized page scripts, sets approved files on identified
// inputs, and reports observed results.
//
// The employer-page JavaScript is single-sourced in the browser package and
// byte-identical to the proven CDP scripts; only the transport differs.
package playwright

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// Engines supported through the bridge.
const (
	EngineChromium = "chromium"
	EngineFirefox  = "firefox"
	EngineWebKit   = "webkit"
)

// ValidEngine reports whether name selects a supported managed engine.
func ValidEngine(name string) bool {
	return name == EngineChromium || name == EngineFirefox || name == EngineWebKit
}

type bridgeError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type bridgeResponse struct {
	ID    int64           `json:"id"`
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value"`
	Error *bridgeError    `json:"error"`
}

// Session is one managed visible browser owned by a single job workflow.
type Session struct {
	mu         sync.Mutex
	opMu       sync.Mutex
	cmd        *exec.Cmd
	stdin      *os.File
	scanner    *bufio.Scanner
	scanMu     sync.Mutex
	next       int64
	pending    map[int64]chan bridgeResponse
	readerErr  error
	dir        string
	engine     string
	version    string
	postingURL string
	// Qualification-only owner override, mirroring the CDP session seam.
	fixtureOwner         func(string) bool
	fixtureContactOrigin string
	fixtureContactBase   string
	closed               bool
}

// Engine names the running browser engine.
func (s *Session) Engine() string { return s.engine }

// label names the engine for user-facing copy.
func (s *Session) label() string { return EngineLabels[s.engine] }

// Version reports the launched browser version for diagnostics.
func (s *Session) Version() string { return s.version }

// Launch opens one visible managed browser on the application page. The
// destination must satisfy the current provider/posting ownership policy; the
// posting identity stays the session owner exactly as with CDP sessions.
func Launch(ctx context.Context, engine, posting string) (*Session, error) {
	return launch(ctx, engine, posting, false)
}

func launch(ctx context.Context, engine, posting string, headless bool) (*Session, error) {
	if !ValidEngine(engine) {
		return nil, fmt.Errorf("unsupported browser engine: %s", engine)
	}
	if !browser.OwnsPostingURL(posting, posting) {
		return nil, errors.New("managed inspection supports saved Ashby, Greenhouse, and Lever postings")
	}
	dir, err := os.MkdirTemp("", "yourmove-playwright-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	// Use Ashby's public posting API to open its exact applyUrl on the first
	// browser launch, mirroring CDP launch behavior.
	landing, err := applicationLanding(ctx, posting)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	return launchRaw(ctx, engine, landing, headless, dir, posting)
}

// launchRaw starts the bridge against an exact page URL without posting
// validation or landing resolution. Production code uses launch; tests use
// launchFixturePage, which sets the fixture ownership seam afterwards.
func launchRaw(ctx context.Context, engine, page string, headless bool, dir, posting string) (*Session, error) {
	if dir == "" {
		fresh, err := os.MkdirTemp("", "yourmove-playwright-")
		if err != nil {
			return nil, err
		}
		dir = fresh
	}
	if err := os.Chmod(dir, 0700); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	node, err := nodeExecutable()
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	driver := bridgeDriver()
	if driver == "" {
		os.RemoveAll(dir)
		return nil, errors.New("playwright bridge driver is unavailable")
	}
	profile := filepath.Join(dir, "profile")
	cmd := exec.Command(node, driver)
	isolateProcessGroup(cmd)
	cmd.Dir = filepath.Dir(driver)
	cmd.Env = bridgeEnv(os.Environ())
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	cmd.Stdin = stdinR
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		stdinR.Close()
		stdinW.Close()
		os.RemoveAll(dir)
		return nil, err
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		stdinR.Close()
		stdinW.Close()
		stdoutR.Close()
		stdoutW.Close()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("start playwright bridge: %w", err)
	}
	stdinR.Close()
	stdoutW.Close()
	s := &Session{cmd: cmd, stdin: stdinW, scanner: bufio.NewScanner(stdoutR), pending: map[int64]chan bridgeResponse{}, dir: dir, engine: engine, postingURL: posting}
	s.scanner.Buffer(make([]byte, 64*1024), 8<<20)
	go s.readLoop(stdoutR)
	var value struct {
		Engine  string `json:"engine"`
		Version string `json:"version"`
	}
	if err := s.call(ctx, "launch", map[string]any{"engine": engine, "url": page, "profileDir": profile, "headless": headless}, &value); err != nil {
		_ = s.Close()
		return nil, err
	}
	s.version = value.Version
	return s, nil
}

// owns reports whether the observed page belongs to the saved posting.
func (s *Session) owns(candidate string) bool {
	if s.fixtureOwner != nil {
		return s.fixtureOwner(candidate)
	}
	return browser.OwnsPostingURL(s.postingURL, candidate)
}

// contactScope resolves the origin/base inputs used by the shared page
// scripts, honoring the qualification fixture override.
func (s *Session) contactScope() (origin, base string, err error) {
	if s.fixtureContactOrigin != "" {
		return s.fixtureContactOrigin, s.fixtureContactBase, nil
	}
	saved, err := url.Parse(s.postingURL)
	if err != nil {
		return "", "", err
	}
	return saved.Scheme + "://" + saved.Host, strings.TrimRight(saved.Path, "/"), nil
}
