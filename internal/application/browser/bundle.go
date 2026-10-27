package browser

import (
	"os"
	"path/filepath"
)

// bundle.go: macOS .app resource resolution for a self-contained release.
//
// A distributable YourMove.app carries its Playwright runtime inside
// Contents/Resources:
//
//	Resources/playwright-bridge/  driver.mjs plus installed node_modules
//	Resources/nodejs/bin/node     official Node.js macOS arm64 binary
//	Resources/ms-playwright/      pinned browser builds (chromium, chromium
//	                              headless shell, firefox, webkit, ffmpeg
//	                              revisions matching the vendored
//	                              playwright-core)
//
// The bridge driver, the Node runtime, and every engine resolve
// executable-relative paths first, so a user without Node, npm, Go, or a
// Playwright cache gets working engines with no downloads at runtime. The
// bundled cache sorts before any developer cache so the shipped revision is
// the one that runs, even on a machine with a newer local Playwright cache.

// BundleResourcesDir returns the application bundle Resources directory when
// the current executable runs from inside a macOS .app, else "".
func BundleResourcesDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	contents := filepath.Clean(filepath.Join(filepath.Dir(exe), ".."))
	resources := filepath.Join(contents, "Resources")
	info, err := os.Stat(filepath.Join(contents, "Info.plist"))
	if err != nil || info.IsDir() {
		return ""
	}
	if info, err := os.Stat(resources); err != nil || !info.IsDir() {
		return ""
	}
	return resources
}

// BundleBrowsersDir returns the bundled Playwright browser cache when the
// executable runs from a bundle that ships one, else "".
func BundleBrowsersDir() string {
	resources := BundleResourcesDir()
	if resources == "" {
		return ""
	}
	dir := filepath.Join(resources, "ms-playwright")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
