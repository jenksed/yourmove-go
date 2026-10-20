package chromium

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// executable finds a Chrome DevTools-compatible visible browser. Prefer an
// explicit override, then the system installation, then Playwright's installed
// Chrome for Testing. Playwright downloads aren't generally on macOS PATH.
func executable() (string, error) {
	if chosen := os.Getenv("YOURMOVE_CHROMIUM"); chosen != "" {
		if !filepath.IsAbs(chosen) {
			return "", errors.New("YOURMOVE_CHROMIUM must be an absolute executable path")
		}
		if !isExecutableFile(chosen) {
			return "", fmt.Errorf("YOURMOVE_CHROMIUM is not an executable file: %s", chosen)
		}
		return chosen, nil
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	home, _ := os.UserHomeDir()
	for _, path := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
		filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		filepath.Join(home, "Applications/Chromium.app/Contents/MacOS/Chromium"),
	} {
		if isExecutableFile(path) {
			return path, nil
		}
	}
	// Playwright is already used by the TypeScript R&D application; reuse the
	// installed browser instead of requiring another several-hundred-MB copy.
	caches := []string{}
	if configured := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); filepath.IsAbs(configured) {
		caches = append(caches, configured)
	}
	if home != "" {
		caches = append(caches,
			filepath.Join(home, "Library", "Caches", "ms-playwright"),
			filepath.Join(home, ".cache", "ms-playwright"),
		)
	}
	for _, cache := range caches {
		if found := cachedPlaywrightChromium(cache); found != "" {
			return found, nil
		}
	}
	return "", errors.New("Chrome/Chromium not found. Install Google Chrome or Playwright Chromium, or set YOURMOVE_CHROMIUM to the browser executable")
}

func isExecutableFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

// Only match full, interactive browser executables. In particular, do not pick
// chromium_headless_shell, which cannot show an application for human review.
func cachedPlaywrightChromium(cache string) string {
	if !filepath.IsAbs(cache) {
		return ""
	}
	for _, suffix := range []string{
		"chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
		"chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
		"chrome-mac/Chromium.app/Contents/MacOS/Chromium",
		"chrome-mac-arm64/Chromium.app/Contents/MacOS/Chromium",
		"chrome-mac-x64/Chromium.app/Contents/MacOS/Chromium",
	} {
		matches, _ := filepath.Glob(filepath.Join(cache, "chromium-*", suffix))
		for i := len(matches) - 1; i >= 0; i-- {
			if isExecutableFile(matches[i]) {
				return matches[i]
			}
		}
	}
	return ""
}
