package nativeidentity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureRun = "0123456789abcdef0123456789abcdef"

func TestQualificationRequiresExplicitMarkedWorkspace(t *testing.T) {
	t.Setenv("YOURMOVE_NATIVE_RUN_ID", fixtureRun)
	for _, root := range []string{"", "relative", t.TempDir()} {
		t.Setenv("YOURMOVE_DATA_DIR", root)
		if _, err := Validate(); err == nil {
			t.Fatalf("unsafe qualification root accepted: %q", root)
		}
	}
	root := t.TempDir()
	t.Setenv("YOURMOVE_DATA_DIR", root)
	if err := os.WriteFile(filepath.Join(root, ".yourmove-synthetic-marker"), []byte(fixtureRun+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if run, err := Validate(); err != nil || run != fixtureRun {
		t.Fatal(run, err)
	}
	for _, run := range []string{"", "stale-window", "../unsafe"} {
		t.Setenv("YOURMOVE_NATIVE_RUN_ID", run)
		if _, err := Validate(); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}

func TestIdentityIsExclusiveAndBoundToProcessAndWorkspace(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "jobs.db")
	resumes := filepath.Join(root, "resumes")
	if err := os.WriteFile(filepath.Join(root, ".yourmove-synthetic-marker"), []byte(fixtureRun+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	title, err := Write(fixtureRun, db, resumes)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".yourmove-native-"+fixtureRun+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r Record
	if err = json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if r.PID != os.Getpid() || r.DataRoot != root || r.Database != db || r.Resumes != resumes || len(r.SHA256) != 64 || r.WindowTitle != title || !strings.Contains(title, fixtureRun) {
		t.Fatalf("incorrect identity: %+v", r)
	}
	if _, err = Write(fixtureRun, db, resumes); err == nil {
		t.Fatal("stale identity overwritten")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("identity not private", err)
	}
}
