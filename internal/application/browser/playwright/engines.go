package playwright

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// EngineInfo describes one engine for the browser selector. Only engines
// that can actually run are offered; missing dependencies are explained.
type EngineInfo struct {
	Name      string
	Label     string
	Available bool
	Reason    string
}

// EngineLabels names the engines in UI copy.
var EngineLabels = map[string]string{
	EngineChromium: "Chromium",
	EngineFirefox:  "Firefox",
	EngineWebKit:   "WebKit",
}

// BridgeReady reports whether the Node bridge can start at all.
func BridgeReady() (bool, string) {
	if _, err := nodeExecutable(); err != nil {
		return false, err.Error()
	}
	if bridgeDriver() == "" {
		return false, "playwright bridge driver is missing"
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(bridgeDriver()), "node_modules", "playwright-core", "package.json")); err != nil {
		return false, "playwright bridge dependencies are not installed; run scripts/setup-playwright-bridge.sh"
	}
	return true, ""
}

// browserRoots returns the Playwright browser cache directories to search.
// Precedence: explicit PLAYWRIGHT_BROWSERS_PATH override, then the browsers
// bundled inside a distributed YourMove.app, then the developer caches.
func browserRoots() []string {
	if configured := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); configured != "" {
		return strings.Split(configured, string(os.PathListSeparator))
	}
	roots := []string{}
	if bundled := browser.BundleBrowsersDir(); bundled != "" {
		roots = append(roots, bundled)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return roots
	}
	return append(roots,
		filepath.Join(home, "Library", "Caches", "ms-playwright"),
		filepath.Join(home, ".cache", "ms-playwright"),
	)
}

func engineInstalled(engine string) bool {
	for _, root := range browserRoots() {
		if root == "" {
			continue
		}
		var patterns []string
		switch engine {
		case EngineChromium:
			patterns = []string{
				"chromium-*/chrome-mac*/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
				"chromium-*/chrome-mac*/Chromium.app/Contents/MacOS/Chromium",
			}
		case EngineFirefox:
			patterns = []string{
				"firefox-*/firefox/Nightly.app/Contents/MacOS/firefox",
				"firefox-*/firefox/firefox",
			}
		case EngineWebKit:
			patterns = []string{"webkit-*/Playwright.app"}
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(filepath.Join(root, pattern))
			for _, m := range matches {
				if info, err := os.Stat(m); err == nil && !info.IsDir() {
					return true
				}
				if engine == EngineWebKit {
					if info, err := os.Stat(m); err == nil && info.IsDir() {
						return true
					}
				}
			}
		}
	}
	return false
}

// AvailableEngines lists every supported engine with its run readiness.
func AvailableEngines() []EngineInfo {
	ready, reason := BridgeReady()
	infos := []EngineInfo{}
	for _, name := range []string{EngineChromium, EngineFirefox, EngineWebKit} {
		info := EngineInfo{Name: name, Label: EngineLabels[name]}
		switch {
		case !ready:
			info.Reason = reason
		case !engineInstalled(name):
			info.Reason = EngineLabels[name] + " browser is not installed in the Playwright cache"
		default:
			info.Available = true
		}
		infos = append(infos, info)
	}
	return infos
}
