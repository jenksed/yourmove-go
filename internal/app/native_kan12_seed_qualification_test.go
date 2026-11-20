//go:build qualification

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/answerlibrary"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// TestNativeReactFixtureKan12OptIn covers the opt-in KAN-12 seed: fresh
// workspaces gain the saved answers and library examples, restarts preserve
// applicant edits, and deleting answers is never silently repopulated.
func TestNativeReactFixtureKan12OptIn(t *testing.T) {
	root, err := os.MkdirTemp(t.TempDir(), "yourmove-react-native-")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_DATA_DIR", root)
	t.Setenv("YOURMOVE_QUALIFY_KAN12", "1")
	if err := os.WriteFile(filepath.Join(root, ".yourmove-react-fixture"), []byte("yourmove-react-fixture-v1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "synthetic-letter-source.docx"), sampleDOCXForPreview(t, "Synthetic fixture source"), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(filepath.Join(root, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	docs := filepath.Join(root, "resumes")
	if err := os.Mkdir(docs, 0700); err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, db, docs)
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:1234/synthetic"); err != nil {
		t.Fatal(err)
	}
	var answers int
	if err := db.QueryRow("SELECT count(*) FROM application_answers").Scan(&answers); err != nil || answers != 2 {
		t.Fatalf("kan12 answers: %d %v", answers, err)
	}
	var library int
	if err := db.QueryRow("SELECT count(*) FROM answer_library").Scan(&library); err != nil || library != 2 {
		t.Fatalf("kan12 library: %d %v", library, err)
	}
	// The seed must be compatible with production ranked search. Single
	// rare terms legitimately fall below the ranker gates, so the supported
	// queries carry at least two informative terms or a phrase overlap.
	for query, want := range map[string]string{
		"poison message":            "Describe a hard production incident you resolved",
		"reliability end":           "Why do you want to join a new engineering team?",
		"join new engineering team": "Why do you want to join a new engineering team?",
	} {
		found, err := answerlibrary.SearchPaged(db, query, answerlibrary.LibrarySearchLimit)
		if err != nil || len(found) == 0 {
			t.Fatalf("search %q: %d %v", query, len(found), err)
		}
		if found[0].OriginalQuestion != want {
			t.Fatalf("search %q top hit: %q", query, found[0].OriginalQuestion)
		}
	}
	if _, err := a.previewApplication(context.Background(), NativeReactFixturePosting); err == nil || !strings.Contains(err.Error(), "synthetic preview disabled") {
		t.Fatalf("kan12 preview gate: %v", err)
	}
	const edited = "Edited in the KAN-12 qualification journey."
	if _, err := db.Exec("UPDATE application_answers SET answer=?, revision=2 WHERE job_id=1 AND label LIKE '%Fixtureworks%'", edited); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:4321/synthetic"); err != nil {
		t.Fatalf("rebind same workspace: %v", err)
	}
	var kept string
	if err := db.QueryRow("SELECT answer FROM application_answers WHERE job_id=1 AND label LIKE '%Fixtureworks%'").Scan(&kept); err != nil || kept != edited {
		t.Fatalf("restart replaced edit: %q %v", kept, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM application_answers").Scan(&answers); err != nil || answers != 2 {
		t.Fatalf("restart duplicated answers: %d %v", answers, err)
	}
	if _, err := db.Exec("DELETE FROM application_answers"); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:4321/synthetic"); err != nil {
		t.Fatalf("rebind after delete: %v", err)
	}
	if err := db.QueryRow("SELECT count(*) FROM application_answers").Scan(&answers); err != nil || answers != 0 {
		t.Fatalf("restart repopulated deleted answers: %d %v", answers, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM answer_library").Scan(&library); err != nil || library != 2 {
		t.Fatalf("library changed on restart: %d %v", library, err)
	}
}
