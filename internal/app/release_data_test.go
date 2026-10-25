package app

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

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

func TestWorkspaceRestartKeepsMultipleJobResumeAssociations(t *testing.T) {
	a, dir := testWorkspace(t)
	for _, job := range []Job{
		{URL: "https://example.test/role-one", Host: "example.test", Title: "Synthetic role one"},
		{URL: "https://example.test/role-two", Host: "example.test", Title: "Synthetic role two"},
		{URL: "https://example.test/role-three", Host: "example.test", Title: "Synthetic role three"},
	} {
		if err := a.saveJob(job); err != nil {
			t.Fatal(err)
		}
	}

	contents := [][]byte{
		[]byte("%PDF-1.4\nsynthetic resume alpha"),
		[]byte("%PDF-1.4\nsynthetic resume beta"),
	}
	resumes := make([]Resume, len(contents))
	for i, content := range contents {
		resume, err := a.importResume("synthetic-resume-"+string(rune('a'+i))+".pdf", bytes.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		resumes[i] = resume
	}
	for jobID, resumeID := range map[int64]int64{1: resumes[0].ID, 2: resumes[1].ID, 3: resumes[0].ID} {
		if err := a.assignResume(jobID, resumeID); err != nil {
			t.Fatalf("assign job %d to resume %d: %v", jobID, resumeID, err)
		}
	}
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}

	reopenedDB, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	reopened := newTestApp(t, reopenedDB, a.resumeDir)
	t.Cleanup(func() { _ = reopenedDB.Close() })
	for jobID, resumeIndex := range map[int64]int{1: 0, 2: 1, 3: 0} {
		job, err := reopened.jobByID(jobID)
		if err != nil || job.SelectedResumeID != resumes[resumeIndex].ID {
			t.Fatalf("job %d selected resume = %d, want %d: %v", jobID, job.SelectedResumeID, resumes[resumeIndex].ID, err)
		}
		stored, err := reopened.readResumeBytes(resumes[resumeIndex])
		if err != nil || !bytes.Equal(stored, contents[resumeIndex]) {
			t.Fatalf("job %d resume bytes mismatch: %v", jobID, err)
		}
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
