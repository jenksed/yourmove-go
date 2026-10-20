package main

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testWorkspace(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := openDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	resumeDir := filepath.Join(dir, "resumes")
	if err := os.Mkdir(resumeDir, 0700); err != nil {
		t.Fatal(err)
	}
	return &App{db: db, resumeDir: resumeDir}, dir
}

func sampleDOCX(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	types, err := z.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(types, `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`); err != nil {
		t.Fatal(err)
	}
	f, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, `<w:document xmlns:w="a"/>`); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestResumeImportAndAssignSubmissionLifecycle(t *testing.T) {
	a, dir := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/job", Host: "example.com", Title: "Test role"}); err != nil {
		t.Fatal(err)
	}
	source := []byte("%PDF-1.4\nresume content")
	imported, err := a.importResume(`../my-resume.pdf`, bytes.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if imported.Name != "my-resume.pdf" || imported.Size != int64(len(source)) {
		t.Fatalf("bad resume: %+v", imported)
	}
	same, err := a.importResume(`duplicate.pdf`, bytes.NewReader(source))
	if err != nil || same.ID != imported.ID {
		t.Fatalf("dedupe: %+v %v", same, err)
	}
	files, err := os.ReadDir(a.resumeDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v error=%v", files, err)
	}
	if err := a.assignResume(1, imported.ID); err != nil {
		t.Fatal(err)
	}
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "Confirmation ABC <script>", token); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "replayed", token); err != nil {
		t.Fatal(err)
	}
	second, err := a.importResume("new.docx", bytes.NewReader(sampleDOCX(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, second.ID); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil || job.SelectedResumeID != second.ID || job.SubmissionCount != 1 || job.StageLabel() != "Applied" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 || history[0].ResumeID != imported.ID || history[0].ResumeName != "my-resume.pdf" || history[0].ResumeSHA256 != imported.SHA256 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	// New DB handle retains both the current resume and the submission snapshot.
	reopened, err := openDB(filepath.Join(dir, "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after, err := (&App{db: reopened}).jobByID(1)
	if err != nil || after.SubmissionCount != 1 || after.SelectedResumeID != second.ID {
		t.Fatalf("reopened=%+v %v", after, err)
	}
}

func TestInvalidResumeRejectsWithoutWriting(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"fake.pdf", []byte("not a PDF")},
		{"foo.exe", []byte("%PDF-1.4\n")},
		{"fake.docx", []byte("PK-invalid")},
		{"empty.pdf", nil},
		{"oversized.pdf", append([]byte("%PDF-1.4"), bytes.Repeat([]byte("X"), maxResumeSize)...)},
	} {
		if _, err := a.importResume(tc.name, bytes.NewReader(tc.body)); err == nil {
			t.Errorf("accepted %q", tc.name)
		}
	}
	files, err := os.ReadDir(a.resumeDir)
	if err != nil || len(files) != 0 {
		t.Fatalf("unexpected files: %d err=%v", len(files), err)
	}
}

func TestApplicationHTTPBrowserAndDesktop(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Sample"}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("resume", "test.pdf")
	if err != nil {
		t.Fatal(err)
	}
	file.Write([]byte("%PDF-1.4\nhello"))
	form.Close()
	req := httptest.NewRequest("POST", "/resumes/import", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/resumes" {
		t.Fatalf("browser upload=%d %s", rec.Code, rec.Body.String())
	}
	if err := a.assignResume(1, 9999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("invalid resume accepted: %v", err)
	}
	idForm := url.Values{"resume_id": {"1"}}
	rec = postApp(mux, "/jobs/1/resume", idForm, "", "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("assign=%d %s", rec.Code, rec.Body.String())
	}
	rec = postApp(mux, "/jobs/1/submit", url.Values{"notes": {"Attempt"}}, "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("submission without explicit confirm=%d", rec.Code)
	}
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	rec = postApp(mux, "/jobs/1/submit", url.Values{"confirm": {"yes"}, "notes": {"Applied with test"}, "submission_token": {token}}, "", "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("submission=%d %s", rec.Code, rec.Body.String())
	}
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest("GET", "/jobs/1", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `name="submission_token"`) || !strings.Contains(get.Body.String(), "Applied with test") || !strings.Contains(get.Body.String(), "test.pdf") {
		t.Fatalf("detail=%d body=%s", get.Code, get.Body.String())
	}
	get = httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest("GET", "/resumes/1/download", nil))
	if get.Code != 200 || !strings.HasPrefix(get.Body.String(), "%PDF-") {
		t.Fatalf("download=%d", get.Code)
	}
	untrusted := postApp(mux, "/jobs/1/submit", url.Values{"confirm": {"yes"}}, "https://bad.test", "")
	if untrusted.Code != http.StatusForbidden {
		t.Fatalf("cross-site submission: %d", untrusted.Code)
	}
	// The picker chooses a local file without sending a blob request through WebKit.
	selectedPath := filepath.Join(t.TempDir(), "chosen.pdf")
	if err := os.WriteFile(selectedPath, []byte("%PDF-1.4\nnative picker"), 0600); err != nil {
		t.Fatal(err)
	}
	a.pickResume = func() (string, error) { return selectedPath, nil }
	native := newDesktopMux(a)
	rec = postApp(native, "/jobs/1/resume", idForm, "wails://localhost", "1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `window.location.replace("/jobs/1")`) {
		t.Fatalf("desktop selection=%d %s", rec.Code, rec.Body.String())
	}
	rec = postApp(native, "/resumes/import-native", url.Values{}, "wails://localhost", "1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `window.location.replace("/resumes")`) {
		t.Fatalf("desktop native import: %d %s", rec.Code, rec.Body.String())
	}
	resumes, err := a.allResumes()
	if err != nil || len(resumes) != 2 {
		t.Fatalf("native imported resumes=%+v %v", resumes, err)
	}
	// Canceling the native picker should be a no-op with a normal navigation.
	a.pickResume = func() (string, error) { return "", nil }
	rec = postApp(native, "/resumes/import-native", url.Values{}, "wails://localhost", "1")
	if rec.Code != http.StatusOK {
		t.Fatalf("desktop picker cancellation=%d", rec.Code)
	}
	rec = postApp(native, "/resumes/import-native", url.Values{}, "https://bad.test", "1")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("untrusted native picker status=%d", rec.Code)
	}
}

func postApp(mux http.Handler, path string, form url.Values, origin, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(form.Encode()))
	r.Host = "localhost"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if id != "" {
		r.Header.Set("X-Wails-Window-Id", id)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestAppliedCannotBeForgedOrReverted(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/job", Host: "example.com", Title: "Example"}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	f := url.Values{"title": {"Changed"}, "company": {"Corp"}, "description": {""}, "salary_text": {""}, "stage": {"applied"}, "priority": {"normal"}}
	w := postApp(mux, "/jobs/1/edit", f, "", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("forged applied stage: %d", w.Code)
	}
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "confirmed", token); err != nil {
		t.Fatal(err)
	}
	f.Set("stage", "review")
	w = postApp(mux, "/jobs/1/edit", f, "", "")
	if w.Code != http.StatusSeeOther {
		t.Fatalf("edit submitted job: %d", w.Code)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if job.Stage != StageApplied || job.Title != "Changed" {
		t.Fatalf("stage reverted or edit lost: %+v", job)
	}
}

func TestSubmissionTokenIdempotenceAndMissingJob(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/job", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "first", token); err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(1, "duplicate", token); err != nil {
		t.Fatal(err)
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 || history[0].Notes != "first" {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	other, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(999, "", other); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing job: %v", err)
	}
	if err := a.recordSubmission(1, "", "bad-token"); err == nil {
		t.Fatal("accepted malformed token")
	}
}

func TestBrowserRejectsCrossSiteResumeDownload(t *testing.T) {
	a, _ := testWorkspace(t)
	if _, err := a.importResume("cv.pdf", bytes.NewReader([]byte("%PDF-1.4\nmock"))); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/resumes/1/download", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("cross-site download status=%d", w.Code)
	}
}

func TestMigrateLegacySQLiteWithoutChangingOriginal(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "old.db")
	db, err := openDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{db: db}
	if err := a.saveJob(Job{URL: "https://example.com/legacy", Host: "example.com", Title: "Old job"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "migrated.db")
	if err := migrateLegacyJobs(dest, legacy); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyJobs(dest, legacy); err != nil {
		t.Fatal(err)
	}
	newDB, err := openDB(dest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := (&App{db: newDB}).jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	newDB.Close()
	if got.Title != "Old job" {
		t.Fatalf("migrated job: %+v", got)
	}
	original, err := openDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if _, err = (&App{db: original}).jobByID(1); err != nil {
		t.Fatalf("original disappeared: %v", err)
	}
}

func TestStoredResumeIntegrityRequiredForDownload(t *testing.T) {
	a, _ := testWorkspace(t)
	r, err := a.importResume("resume.pdf", bytes.NewReader([]byte("%PDF-1.4\noriginal")))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a damaged file in an otherwise valid resume registry.
	if err := os.WriteFile(filepath.Join(a.resumeDir, r.StoredName), []byte("%PDF-1.4\nmodified"), 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resumes/1/download", nil))
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "modified") {
		t.Fatalf("altered artifact served: status=%d", w.Code)
	}
}
