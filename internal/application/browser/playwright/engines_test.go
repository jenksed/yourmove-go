package playwright

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFirefoxLayout(t *testing.T, root, dir, name string) string {
	t.Helper()
	bin := filepath.Join(root, dir, name)
	if err := os.MkdirAll(filepath.Dir(bin), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestAvailableEnginesSeesCurrentFirefoxLayout(t *testing.T) {
	if ok, _ := BridgeReady(); !ok {
		t.Skip("playwright bridge dependencies are not installed")
	}
	root := t.TempDir()
	writeFirefoxLayout(t, root, "firefox-1509", "firefox")
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", root)
	found := false
	for _, info := range AvailableEngines() {
		if info.Name == EngineFirefox {
			found = true
			if !info.Available {
				t.Fatalf("firefox reported unavailable: %s", info.Reason)
			}
		}
	}
	if !found {
		t.Fatal("firefox missing from AvailableEngines")
	}
}

func TestEngineInstalledSeesWindowsLayouts(t *testing.T) {
	root := t.TempDir()
	winChromium := filepath.Join(root, "chromium-1181", "chrome-win64", "chrome.exe")
	if err := os.MkdirAll(filepath.Dir(winChromium), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winChromium, []byte("browser"), 0644); err != nil {
		t.Fatal(err)
	}
	winFirefox := filepath.Join(root, "firefox-1509", "firefox", "firefox.exe")
	if err := os.MkdirAll(filepath.Dir(winFirefox), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winFirefox, []byte("browser"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", root)
	// Existence-only check: Windows .exe files carry no execute bits, so
	// plain 0644 fixtures must satisfy discovery on any host.
	if !engineInstalled(EngineChromium) {
		t.Fatal("windows chromium layout not recognized")
	}
	if !engineInstalled(EngineFirefox) {
		t.Fatal("windows firefox layout not recognized")
	}
}

func TestAvailableEnginesExplainsMissingEngines(t *testing.T) {
	if ok, _ := BridgeReady(); !ok {
		t.Skip("playwright bridge dependencies are not installed")
	}
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", t.TempDir())
	for _, info := range AvailableEngines() {
		if info.Available {
			t.Fatalf("%s reported available from an empty cache", info.Name)
		}
		if info.Reason == "" {
			t.Fatalf("%s has no explanation for being unavailable", info.Name)
		}
	}
}
