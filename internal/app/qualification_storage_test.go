//go:build qualification

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requireQualificationStorage runs before a controlled browser starts. All
// mutable test state must live under testWorkspace's temporary directory.
func requireQualificationStorage(t *testing.T, a *App) string {
	t.Helper()
	var sequence int
	var name, databasePath string
	if err := a.db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &databasePath); err != nil {
		t.Fatal(err)
	}
	root, rootErr := filepath.EvalSymlinks(filepath.Dir(a.resumeDir))
	database, databaseErr := filepath.EvalSymlinks(databasePath)
	resumes, resumesErr := filepath.EvalSymlinks(a.resumeDir)
	within := func(path string) bool {
		relative, err := filepath.Rel(root, path)
		return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
	}
	if name != "main" || rootErr != nil || databaseErr != nil || resumesErr != nil || !within(database) || !within(resumes) {
		t.Fatalf("controlled browser test storage escaped its temporary workspace: database=%q resumes=%q root=%q errors=(%v,%v,%v)", databasePath, a.resumeDir, root, rootErr, databaseErr, resumesErr)
	}
	t.Logf("isolated Chromium qualification storage: SQLite=%s resumes=%s root=%s", database, resumes, root)
	return root
}
