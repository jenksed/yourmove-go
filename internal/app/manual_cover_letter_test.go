package app

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func manualCoverLetterRequest(a *App, jobID string, body string) *httptest.ResponseRecorder {
	form := url.Values{"body": {body}}
	r := httptest.NewRequest(http.MethodPost, "/jobs/"+jobID+"/cover-letter/manual", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", jobID)
	w := httptest.NewRecorder()
	a.createManualCoverLetterHandler(w, r)
	return w
}

func TestCreateManualCoverLetterHasNoGeneratedSourceEvidence(t *testing.T) {
	a, dir := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/role", Host: "example.test", Stage: StageClosed}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	var foreignKeys int
	if err := a.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign-key enforcement is not enabled: %d, %v", foreignKeys, err)
	}

	body := "A manually written cover letter for a saved job, with no AI generation or resume evidence."
	w := manualCoverLetterRequest(a, "1", body)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("create manual letter: %d %s", w.Code, w.Body.String())
	}
	var id int64
	var storedBody, provider, model, masterHash, roleHash, descriptionHash string
	var masterID, roleID sql.NullInt64
	err := a.db.QueryRow(`SELECT id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256
		FROM cover_letters WHERE job_id=1`).Scan(&id, &storedBody, &provider, &model, &masterID, &masterHash, &roleID, &roleHash, &descriptionHash)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 || storedBody != body || provider != "manual" || model != "user-authored" || masterID.Valid || roleID.Valid || masterHash != "" || roleHash != "" || descriptionHash != "" {
		t.Fatalf("manual provenance is inaccurate: id=%d provider=%q model=%q master=%+v role=%+v hashes=%q/%q/%q", id, provider, model, masterID, roleID, masterHash, roleHash, descriptionHash)
	}
	if w.Header().Get("Location") != coverDraftNoticeURL(1, id, "saved") {
		t.Fatalf("unexpected redirect: %q", w.Header().Get("Location"))
	}

	// Reopen through the normal migration path and ensure the saved draft remains usable.
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var reopenedBody, reopenedProvider, reopenedModel string
	if err := reopened.QueryRow("SELECT body,provider,model FROM cover_letters WHERE id=? AND job_id=1", id).Scan(&reopenedBody, &reopenedProvider, &reopenedModel); err != nil {
		t.Fatal(err)
	}
	if reopenedBody != body || reopenedProvider != "manual" || reopenedModel != "user-authored" {
		t.Fatalf("manual draft did not survive reopen: body=%q provider=%q model=%q", reopenedBody, reopenedProvider, reopenedModel)
	}
}

func TestCreateManualCoverLetterRejectsInvalidTextAndUnknownJobs(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/role", Host: "example.test"}); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		"Too short text.",
		strings.Repeat("x", maxLetterBytes+1),
		"This body contains a NUL byte\x00 and must be rejected.",
		"This body contains invalid UTF-8 \xff and must be rejected.",
	} {
		w := manualCoverLetterRequest(a, "1", body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("invalid body status=%d body=%q", w.Code, w.Body.String())
		}
	}
	if w := manualCoverLetterRequest(a, "2", "A sufficiently long manually written letter for a job that does not exist."); w.Code != http.StatusNotFound {
		t.Fatalf("unknown job status=%d body=%q", w.Code, w.Body.String())
	}
	var count int
	if err := a.db.QueryRow("SELECT count(*) FROM cover_letters").Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid requests created drafts: count=%d err=%v", count, err)
	}
}
