package chromium

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecutableOverride(t *testing.T) {
	browser := filepath.Join(t.TempDir(), "Google Chrome for Testing")
	if err := os.WriteFile(browser, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_CHROMIUM", browser)
	got, err := executable()
	if err != nil || got != browser {
		t.Fatalf("executable()=%q, %v", got, err)
	}
	t.Setenv("YOURMOVE_CHROMIUM", "relative/chrome")
	if _, err := executable(); err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("relative path not rejected: %v", err)
	}
	t.Setenv("YOURMOVE_CHROMIUM", filepath.Dir(browser))
	if _, err := executable(); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("directory not rejected: %v", err)
	}
	t.Setenv("YOURMOVE_CHROMIUM", filepath.Join(t.TempDir(), "no-such-browser"))
	if _, err := executable(); err == nil || !strings.Contains(err.Error(), "executable") {
		t.Fatalf("missing path not rejected: %v", err)
	}
}

func TestResolveFallbackPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, chrome, edge, cached, want string
	}{
		{"chrome wins over edge and cache", "/c/chrome", "/e/msedge", "/p/chrome", "/c/chrome"},
		{"edge wins over cache", "", "/e/msedge", "/p/chrome", "/e/msedge"},
		{"cache serves alone", "", "", "/p/chrome", "/p/chrome"},
		{"edge serves alone", "", "/e/msedge", "", "/e/msedge"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveFallback(tc.chrome, tc.edge, tc.cached)
			if err != nil || got != tc.want {
				t.Fatalf("resolveFallback()=%q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := resolveFallback("", "", ""); err == nil || !strings.Contains(err.Error(), "Microsoft Edge") {
		t.Fatalf("missing-browser error must stay actionable and mention Edge: %v", err)
	}
}

func TestDiscoverEdgeStaysWindowsOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"msedge", "msedge.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("edge"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	found := discoverEdge()
	if runtime.GOOS != "windows" {
		if found != "" {
			t.Fatalf("Edge discovery must not affect macOS/Linux, found %q", found)
		}
		return
	}
	if found == "" || !strings.HasPrefix(found, dir) {
		t.Fatalf("Edge on PATH was not discovered on Windows: %q", found)
	}
}

func TestEdgePathsScope(t *testing.T) {
	paths := edgePaths()
	if runtime.GOOS != "windows" {
		if paths != nil {
			t.Fatalf("Edge paths must be empty off Windows: %q", paths)
		}
		return
	}
	if len(paths) == 0 {
		t.Fatal("Edge paths must not be empty on Windows")
	}
	for _, path := range paths {
		if !strings.HasSuffix(path, filepath.Join("Microsoft", "Edge", "Application", "msedge.exe")) {
			t.Fatalf("unexpected Edge layout: %q", path)
		}
	}
}

func TestEdgePathsFromBases(t *testing.T) {
	base := t.TempDir() // Absolute on any host.
	got := edgePathsFromBases([]string{base, "", "relative/base"})
	if len(got) != 1 {
		t.Fatalf("expected one absolute Edge candidate, got %q", got)
	}
	want := filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe")
	if got[0] != want {
		t.Fatalf("unexpected Edge layout: got %q want %q", got[0], want)
	}
}

func TestPlaywrightMacCacheDiscovery(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "ms-playwright")
	expected := filepath.Join(cache, "chromium-1234", "chrome-mac-arm64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing")
	if err := os.MkdirAll(filepath.Dir(expected), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, []byte("browser"), 0700); err != nil {
		t.Fatal(err)
	}
	headless := filepath.Join(cache, "chromium_headless_shell-9999", "chrome-headless-shell-mac-arm64", "chrome-headless-shell")
	if err := os.MkdirAll(filepath.Dir(headless), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(headless, []byte("shell"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := cachedPlaywrightChromium(cache); got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
	if err := os.Chmod(expected, 0600); err != nil {
		t.Fatal(err)
	}
	if got := cachedPlaywrightChromium(cache); got != "" {
		t.Fatalf("accepted non-executable candidate %q", got)
	}
}

func TestPlaywrightWindowsCacheDiscovery(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "ms-playwright")
	expected := filepath.Join(cache, "chromium-1181", "chrome-win64", "chrome.exe")
	if err := os.MkdirAll(filepath.Dir(expected), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, []byte("browser"), 0700); err != nil {
		t.Fatal(err)
	}
	// The headless shell must never satisfy interactive-browser discovery,
	// on the Windows layout as on the macOS layout.
	headless := filepath.Join(cache, "chromium_headless_shell-1181", "chrome-win", "headless_shell.exe")
	if err := os.MkdirAll(filepath.Dir(headless), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(headless, []byte("shell"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := cachedPlaywrightChromium(cache); got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
	if err := os.Chmod(expected, 0600); err != nil {
		t.Fatal(err)
	}
	if got := cachedPlaywrightChromium(cache); got != "" {
		t.Fatalf("accepted non-executable candidate %q", got)
	}
}

func TestPlaywrightCacheIgnoresRelativeRoot(t *testing.T) {
	if got := cachedPlaywrightChromium("chromium-cache"); got != "" {
		t.Fatalf("accepted relative browser cache: %q", got)
	}
}
