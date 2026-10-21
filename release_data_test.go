package main

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationRefusesDocumentWorkspaceWithoutInstallingDatabase(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "legacy.db")
	db, err := openDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "resumes")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := &App{db: db, resumeDir: dir}
	original := []byte("%PDF-1.4\nsynthetic original")
	r, err := a.importResume("original.pdf", bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "destination.db")
	err = migrateLegacyJobs(destination, legacy)
	if err == nil || !strings.Contains(err.Error(), "entire workspace") {
		t.Fatalf("migration should refuse with recovery instructions: %v", err)
	}
	for _, path := range []string{destination, destination + ".migration-tmp"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("migration left %s: %v", path, err)
		}
	}
	after, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("source database changed: %v", err)
	}
	stored, err := os.ReadFile(filepath.Join(dir, r.StoredName))
	if err != nil || !bytes.Equal(original, stored) {
		t.Fatalf("source document changed: %v", err)
	}
}

func TestReimportRequiresIntactStoredDocument(t *testing.T) {
	for _, state := range []string{"missing", "damaged"} {
		t.Run(state, func(t *testing.T) {
			a, _ := testWorkspace(t)
			body := []byte("%PDF-1.4\nsynthetic original")
			r, err := a.importResume("original.pdf", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(a.resumeDir, r.StoredName)
			if state == "missing" {
				err = os.Remove(path)
			} else {
				err = os.WriteFile(path, []byte("%PDF-1.4\ncorruption"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := a.importResume("retry.pdf", bytes.NewReader(body)); err == nil || !strings.Contains(err.Error(), "restore the complete workspace") {
				t.Fatalf("false import success: %v", err)
			}
			var count int
			if err := a.db.QueryRow("SELECT count(*) FROM resumes").Scan(&count); err != nil || count != 1 {
				t.Fatalf("registry changed: %d %v", count, err)
			}
			if state == "missing" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected file write: %v", err)
				}
			} else {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != "%PDF-1.4\ncorruption" {
					t.Fatalf("damaged file overwritten: %v", err)
				}
			}
		})
	}
}

func TestEncodedMaximumLetterSaveReturnsToPrepare(t *testing.T) {
	a, f := scopeFirstWorkspace(t)
	body := strings.Repeat("é", maxLetterBytes/2)
	form := url.Values{"draft_id": {"1"}, "body": {body}}
	if len(form.Encode()) <= 24<<10 {
		t.Fatal("fixture does not exercise encoding expansion")
	}
	r := postApp(newMux(a), "/jobs/1/cover-letter/save", form, "", "")
	if r.Code != 303 || r.Header().Get("Location") != "/jobs/1?cover=saved&draft=1#prepare" {
		t.Fatalf("save: %d %s", r.Code, r.Body.String())
	}
	draft, err := a.coverLetterByID(1, 1)
	if err != nil || draft.Body != body {
		t.Fatalf("saved text differs: %v", err)
	}
	if f.calls != 0 {
		t.Fatal("local save transferred a document")
	}
	tooLong := strings.Repeat("é", maxLetterBytes/2+1)
	r = postApp(newMux(a), "/jobs/1/cover-letter/save", url.Values{"draft_id": {"1"}, "body": {tooLong}}, "", "")
	if r.Code != 422 || !strings.Contains(r.Body.String(), tooLong) {
		t.Fatalf("invalid text not recoverable: %d", r.Code)
	}
	draft, err = a.coverLetterByID(1, 1)
	if err != nil || draft.Body != body {
		t.Fatal("invalid save changed saved revision")
	}
}
