package playwright

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// bridgeDriver locates the Node driver script. Resolution order:
//  1. YOURMOVE_BRIDGE_DIR override (packaging tests and custom layouts).
//  2. Alongside the running executable, including the macOS bundle
//     Resources layout (YourMove.app/Contents/Resources/playwright-bridge).
//  3. The source tree (go run / go test development).
//
// A desktop installer must ship the bridge directory (driver plus its
// installed node_modules) at one of these locations; the bridge never
// downloads anything at runtime.
func bridgeDriver() string {
	if env := os.Getenv("YOURMOVE_BRIDGE_DIR"); env != "" {
		if driver := driverIn(env); driver != "" {
			return driver
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, candidate := range []string{
			filepath.Join(dir, "playwright-bridge"),
			filepath.Join(dir, "..", "Resources", "playwright-bridge"),
		} {
			if driver := driverIn(candidate); driver != "" {
				return driver
			}
		}
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return driverIn(filepath.Join(filepath.Dir(file), "bridge"))
}

func driverIn(dir string) string {
	driver := filepath.Join(dir, "driver.mjs")
	info, err := os.Stat(driver)
	if err != nil || info.IsDir() {
		return ""
	}
	return driver
}

// nodeExecutable finds a Node.js runtime for the bridge without requiring a
// developer toolchain on PATH to be any particular version.
func nodeExecutable() (string, error) {
	if chosen := os.Getenv("YOURMOVE_NODE"); chosen != "" {
		if !filepath.IsAbs(chosen) {
			return "", errors.New("YOURMOVE_NODE must be an absolute executable path")
		}
		if !isExecutableFile(chosen) {
			return "", fmt.Errorf("YOURMOVE_NODE is not an executable file: %s", chosen)
		}
		return chosen, nil
	}
	if path, err := exec.LookPath("node"); err == nil {
		return path, nil
	}
	candidates := []string{
		"/opt/homebrew/bin/node",
		"/usr/local/bin/node",
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append([]string{
			filepath.Join(dir, "node"),
			filepath.Join(dir, "..", "Resources", "nodejs", "bin", "node"),
		}, candidates...)
	}
	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", errors.New("Node.js is required for managed Firefox/WebKit/Chromium sessions; install Node.js 20+ or set YOURMOVE_NODE")
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

// bridgeEnv passes the Playwright browser cache through and blocks surprise
// network downloads: the driver resolves installed browsers and never
// installs. Operators may point PLAYWRIGHT_BROWSERS_PATH at a managed cache.
func bridgeEnv(env []string) []string {
	out := make([]string, 0, len(env)+2)
	sawBrowsers, sawDownload := false, false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PLAYWRIGHT_BROWSERS_PATH=") {
			sawBrowsers = true
		}
		if strings.HasPrefix(kv, "PLAYWRIGHT_DOWNLOAD_HOST=") {
			sawDownload = true
		}
		out = append(out, kv)
	}
	if !sawBrowsers {
		// A single directory, not a list: prefer the browsers bundled in a
		// distributed YourMove.app, then the macOS cache, then the XDG
		// cache. An operator override is passed through untouched.
		if bundled := browser.BundleBrowsersDir(); bundled != "" {
			out = append(out, "PLAYWRIGHT_BROWSERS_PATH="+bundled)
		} else if home, err := os.UserHomeDir(); err == nil {
			for _, candidate := range []string{
				filepath.Join(home, "Library", "Caches", "ms-playwright"),
				filepath.Join(home, ".cache", "ms-playwright"),
			} {
				if info, err := os.Stat(candidate); err == nil && info.IsDir() {
					out = append(out, "PLAYWRIGHT_BROWSERS_PATH="+candidate)
					break
				}
			}
		}
	}
	if !sawDownload {
		// Never consult a download host; installed browsers only.
		out = append(out, "PLAYWRIGHT_DOWNLOAD_HOST=0.0.0.0")
	}
	return out
}

// call performs one bridge RPC with context cancellation.
func (s *Session) call(ctx context.Context, op string, params map[string]any, out any) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("browser session closed")
	}
	s.next++
	id := s.next
	ch := make(chan bridgeResponse, 1)
	s.pending[id] = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()
	payload := map[string]any{"id": id, "op": op}
	for k, v := range params {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	s.scanMu.Lock()
	_, err = s.stdin.Write(data)
	s.scanMu.Unlock()
	if err != nil {
		return fmt.Errorf("playwright bridge write: %w", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		if resp.OK {
			if out != nil && len(resp.Value) > 0 {
				return json.Unmarshal(resp.Value, out)
			}
			return nil
		}
		if resp.Error == nil {
			return errors.New("playwright bridge error")
		}
		return s.classifyBridgeError(resp.Error)
	case <-time.After(45 * time.Second):
		return errors.New("playwright bridge timed out")
	}
}

func (s *Session) classifyBridgeError(be *bridgeError) error {
	switch be.Kind {
	case "closed":
		if strings.Contains(be.Message, "no open pages") {
			return fmt.Errorf("%w: %s", browser.ErrNoPageTargets, be.Message)
		}
		return fmt.Errorf("%w: %s", browser.ErrBrowserUnavailable, be.Message)
	case "launch":
		return fmt.Errorf("managed %s could not launch: %s", s.engine, be.Message)
	case "timeout":
		return fmt.Errorf("managed %s timed out: %s", s.engine, be.Message)
	default:
		return errors.New(be.Message)
	}
}

func (s *Session) readLoop(f *os.File) {
	defer f.Close()
	for s.scanner.Scan() {
		var resp bridgeResponse
		if err := json.Unmarshal(s.scanner.Bytes(), &resp); err != nil {
			continue
		}
		s.mu.Lock()
		ch := s.pending[resp.ID]
		s.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
	s.mu.Lock()
	s.readerErr = errors.New("playwright bridge exited")
	for _, ch := range s.pending {
		ch <- bridgeResponse{Error: &bridgeError{Kind: "closed", Message: "playwright bridge exited"}}
	}
	s.pending = map[int64]chan bridgeResponse{}
	s.mu.Unlock()
}

// Close kills the bridge and its browser, then removes the isolated profile.
func (s *Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	closed := s.call(ctx, "close", nil, nil) == nil
	_ = s.stdin.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		if closed {
			// Graceful bridge shutdown also closes its browser; reap
			// quickly and escalate to the whole process group if needed.
			done := make(chan struct{})
			go func() { _ = s.cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				_ = killProcessGroup(s.cmd)
				_ = s.cmd.Wait()
			}
		} else {
			// The bridge is unreachable (crash or kill): terminate the
			// group so an orphaned browser cannot outlive the session.
			_ = killProcessGroup(s.cmd)
			_ = s.cmd.Wait()
		}
	}
	return os.RemoveAll(s.dir)
}
