//go:build qualification

package chromium

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LaunchVisibleFixture is available only in qualification builds. It owns
// precisely one loopback fixture subtree and uses the normal Chromium effects.
func LaunchVisibleFixture(ctx context.Context, posting string) (*Session, error) {
	u, err := url.Parse(posting)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/synthetic" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("qualification requires an exact loopback fixture")
	}
	exe, err := executable()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "yourmove-native-fixture-browser-")
	if err != nil {
		return nil, err
	}
	args := launchArguments(dir, posting+"/application")
	args = append(args, "--disable-background-networking")
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
