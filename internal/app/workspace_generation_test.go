package app

import (
	"bytes"
	"context"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

// The request-bound App owns its DB and document directory for its entire
// lifetime. A changed launch environment must not redirect AI results or GETs.
func TestGenerationRetainsWorkspaceAcrossEnvironmentChanges(t *testing.T) {
	a, root := testWorkspace(t)
	b, _ := testWorkspace(t)
	for _, instance := range []*App{a, b} {
		if err := instance.saveJob(Job{URL: "https://synthetic.example.test/role", Host: "synthetic.example.test", Title: "Synthetic", Description: "Synthetic work on local systems"}); err != nil {
			t.Fatal(err)
		}
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Synthetic master career evidence")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Synthetic selected experience")))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	started, finish := make(chan struct{}), make(chan struct{})
	a.coverLetterGenerate = func(context.Context, string) (string, error) {
		close(started)
		<-finish
		return "Synthetic generated letter using only the selected fixture evidence.", nil
	}
	t.Setenv("YOURMOVE_DATA_DIR", filepath.Dir(b.resumeDir))
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- postApp(newDesktopMux(a), "/jobs/1/cover-letter/generate", url.Values{"confirm_local": {"yes"}}, "wails://localhost", "")
	}()
	<-started
	page := httptest.NewRecorder()
	newDesktopMux(a).ServeHTTP(page, httptest.NewRequest("GET", "wails://localhost/resumes", nil))
	close(finish)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "role.docx") {
		t.Fatalf("unrelated navigation blocked or wrong workspace: %d %s", page.Code, page.Body.String())
	}
	result := <-done
	if result.Code != 200 || !strings.Contains(result.Body.String(), "cover=generated") {
		t.Fatal("generation failed", result.Code)
	}
	if _, err = a.latestCoverLetter(1); err != nil {
		t.Fatal(err)
	}
	if _, err = b.latestCoverLetter(1); err == nil {
		t.Fatal("AI result leaked into second workspace")
	}
	if a.resumeDir != filepath.Join(root, "resumes") {
		t.Fatal("document root changed")
	}
	a.Close()
	if err = a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.OpenDB(filepath.Join(root, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err = reopened.QueryRow(`SELECT count(*) FROM cover_letters WHERE job_id=1 AND role_sha256=?`, role.SHA256).Scan(&count); err != nil || count != 1 {
		t.Fatal("AI result association not persistent", count, err)
	}
}
