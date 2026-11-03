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
