package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func normalWorkspace(t *testing.T, root string) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		canonical, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(canonical, "Library", "Application Support", "YourMove")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(canonical, "config", "YourMove")
}

func normalStorageEnvironment(t *testing.T, root string) {
	t.Helper()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("YOURMOVE_DATA_DIR", "") // register restoration, then model an absent override
	if err := os.Unsetenv("YOURMOVE_DATA_DIR"); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitWorkspaceIsolationAndRestart(t *testing.T) {
	working := t.TempDir()
	t.Chdir(working)
	legacy := filepath.Join(working, "jobs.db")
	db, err := OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO jobs(url,host,title) VALUES('https://legacy.example.test','legacy.example.test','legacy-only')`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	roots := []string{t.TempDir(), t.TempDir()}
	for i, root := range roots {
		var err error
		roots[i], err = filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
	}
	for round := 0; round < 3; round++ {
		for i, root := range roots {
			t.Setenv("YOURMOVE_DATA_DIR", root)
			path, resumes, err := ApplicationStorage()
			if err != nil {
				t.Fatal(err)
			}
			canonical, _ := filepath.EvalSymlinks(root)
			if path != filepath.Join(canonical, "jobs.db") || resumes != filepath.Join(canonical, "resumes") {
				t.Fatal("wrong workspace paths")
			}
			db, err := OpenDB(path)
			if err != nil {
				t.Fatal(err)
			}
			var count int
			if err = db.QueryRow(`SELECT count(*) FROM jobs`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if round == 0 {
				if count != 0 {
					t.Fatal("explicit empty workspace imported legacy jobs")
				}
				if _, err = db.Exec(`INSERT INTO jobs(url,host,title) VALUES('https://synthetic.example.test','synthetic.example.test',?)`, []string{"workspace-A", "workspace-B"}[i]); err != nil {
					t.Fatal(err)
				}
			} else {
				var title string
				if count != 1 {
					t.Fatalf("unexpected job count: %d", count)
				}
				if err = db.QueryRow(`SELECT title FROM jobs`).Scan(&title); err != nil || title != []string{"workspace-A", "workspace-B"}[i] {
					t.Fatalf("cross-workspace data: %s %v", title, err)
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	after, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("synthetic legacy changed", err)
	}
}

func TestWorkspaceInvalidConfigurationFailsClosed(t *testing.T) {
	for _, value := range []string{"", "relative", "/"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("YOURMOVE_DATA_DIR", value)
			if db, resumes, err := ApplicationStorage(); err == nil || db != "" || resumes != "" {
				t.Fatalf("unsafe fallback: %q %q %v", db, resumes, err)
			}
		})
	}
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_DATA_DIR", filepath.Join(file, "child"))
	if _, _, err := ApplicationStorage(); err == nil {
		t.Fatal("inaccessible ancestor accepted")
	}
	for _, member := range []string{"jobs.db", "resumes"} {
		t.Run(member, func(t *testing.T) {
			workspace := t.TempDir()
			outside := t.TempDir()
			if err := os.Symlink(outside, filepath.Join(workspace, member)); err != nil {
				t.Fatal(err)
			}
			t.Setenv("YOURMOVE_DATA_DIR", workspace)
			if _, _, err := ApplicationStorage(); err == nil {
				t.Fatal("redirected workspace member accepted")
			}
		})
	}
}

func TestWorkspaceRootAliasIsCanonical(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_DATA_DIR", alias)
	db, resumes, err := ApplicationStorage()
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if db != filepath.Join(canonical, "jobs.db") || resumes != filepath.Join(canonical, "resumes") {
		t.Fatal("alias not canonicalized")
	}
}
