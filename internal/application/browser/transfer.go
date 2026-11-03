package browser

import (
	"errors"
	"strings"
)

// LocalizeScript rewrites engine-named UI copy inside shared page scripts
// for non-Chromium engines. The Chromium label short-circuits so the
// release-path CDP driver keeps byte-identical sources.
func LocalizeScript(script, label string) string {
	if label == "" || label == "Chromium" {
		return script
	}
	return strings.ReplaceAll(script, "Chromium", label)
}

// Session-lifecycle sentinels. Every managed engine (CDP Chromium today,
// Playwright engines next) reports these so session ownership, recovery, and
// UI copy stay engine-neutral.
var (
	ErrNoPageTargets      = errors.New("the managed browser has no open pages")
	ErrBrowserUnavailable = errors.New("the managed browser is no longer reachable")
	ErrNoMatchingTab      = errors.New("saved application is not the current browser page")
)
