//go:build qualification

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestNativeReactFixtureGuardAndRestart(t *testing.T) {
	root, err := os.MkdirTemp(t.TempDir(), "yourmove-react-native-")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_DATA_DIR", root)
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
	if err := ConfigureNativeReactFixture(a, root, "http://example.com/synthetic"); err == nil {
		t.Fatal("accepted non-loopback fixture")
	}
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:1234/other"); err == nil {
		t.Fatal("accepted unrelated route")
	}
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:1234/synthetic"); err != nil {
		t.Fatal(err)
	}
	letter, err := a.latestCoverLetter(1)
	if err != nil || letter.Provider != "fixture" {
		t.Fatalf("missing synthetic letter: %+v %v", letter, err)
	}
	const edited = "Synthetic applicant edited this letter and expects the exact text to survive restart."
	if err := a.saveCoverLetterBody(1, letter.ID, edited); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureNativeReactFixture(a, root, "http://127.0.0.1:4321/synthetic"); err != nil {
		t.Fatalf("rebind same workspace: %v", err)
	}
	letter, err = a.latestCoverLetter(1)
	if err != nil || letter.Body != edited {
		t.Fatalf("restart replaced letter: %+v %v", letter, err)
	}
	var letters int
	if err := db.QueryRow("SELECT count(*) FROM cover_letters").Scan(&letters); err != nil || letters != 1 {
		t.Fatalf("unexpected letter count %d: %v", letters, err)
	}
	if _, err := a.browsers.factory(context.Background(), "https://jobs.ashbyhq.com/other/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"); err == nil {
		t.Fatal("allowed another posting")
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM jobs").Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected seed count %d: %v", count, err)
	}
}
