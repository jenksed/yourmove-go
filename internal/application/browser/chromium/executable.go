package chromium

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// executable finds a Chrome DevTools-compatible visible browser. Prefer an
// explicit override, then the system installation, then Playwright's installed
// Chrome for Testing. Playwright downloads aren't generally on macOS PATH.
func executable() (string, error) {
	if chosen := os.Getenv("YOURMOVE_CHROMIUM"); chosen != "" {
		if !filepath.IsAbs(chosen) {
			return "", errors.New("YOURMOVE_CHROMIUM must be an absolute executable path")
		}
		if !browser.IsExecutableFile(chosen) {
			return "", fmt.Errorf("YOURMOVE_CHROMIUM is not an executable file: %s", chosen)
		}
		return chosen, nil
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "chrome.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	home, _ := os.UserHomeDir()
	paths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/Applications/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
		filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		filepath.Join(home, "Applications/Chromium.app/Contents/MacOS/Chromium"),
	}
	// Windows installs outside PATH and outside the macOS .app layouts.
	// LOCALAPPDATA covers per-user installs; ProgramFiles covers machine
	// installs. Env-dependent entries are added only when set so a missing
	// variable can never produce a relative path.
	if runtime.GOOS == "windows" {
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
			if base == "" || !filepath.IsAbs(base) {
				continue
			}
			paths = append(paths, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
		}
	}
	for _, path := range paths {
		if browser.IsExecutableFile(path) {
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
	// Playwright's default Windows cache lives under LOCALAPPDATA, not under
	// either Unix-style cache directory above.
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" && filepath.IsAbs(localAppData) {
			caches = append(caches, filepath.Join(localAppData, "ms-playwright"))
		}
	}
	for _, cache := range caches {
		if found := cachedPlaywrightChromium(cache); found != "" {
			return found, nil
		}
	}
	return "", errors.New("Chrome/Chromium not found. Install Google Chrome or Playwright Chromium, or set YOURMOVE_CHROMIUM to the browser executable")
}

// ResolveExecutable reports the browser binary the production session
// launches, so UI availability claims match the real launch path.
func ResolveExecutable() (string, error) { return executable() }

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
		"chrome-win64/chrome.exe",
		"chrome-win/chrome.exe",
		"chrome-linux/chrome",
	} {
		matches, _ := filepath.Glob(filepath.Join(cache, "chromium-*", suffix))
		for i := len(matches) - 1; i >= 0; i-- {
			if browser.IsExecutableFile(matches[i]) {
				return matches[i]
			}
		}
	}
	return ""
}
