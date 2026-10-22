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

	"github.com/jenksed/yourmove-go/internal/documents"
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

func reviewedSubmissionToken(t *testing.T, a *App, jobID int64) string {
	t.Helper()
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	resumeHash := ""
	if job.SelectedResumeID > 0 {
		if resume, resumeErr := a.resumeByID(job.SelectedResumeID); resumeErr == nil {
			resumeHash = resume.SHA256
		} else if !errors.Is(resumeErr, sql.ErrNoRows) {
			t.Fatal(resumeErr)
		}
	}
	if err := a.recordSubmissionReview(jobID, token, job.SelectedResumeID, resumeHash); err != nil {
		t.Fatal(err)
	}
	return token
}

func submissionTokenFromPage(t *testing.T, page string) string {
	t.Helper()
	const field = `name="submission_token" value="`
	start := strings.Index(page, field)
	if start < 0 {
		t.Fatal("job page is missing its submission token")
	}
	start += len(field)
	end := strings.IndexByte(page[start:], '"')
	if end < 0 {
		t.Fatal("job page has an invalid submission token")
	}
	token := page[start : start+end]
	if !validSubmissionToken(token) {
		t.Fatalf("job page has invalid submission token %q", token)
	}
	return token
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

func TestResumeRevisionArchiveAndInAppNotice(t *testing.T) {
	a, _ := testWorkspace(t)
	old, err := a.importResume("support.pdf", strings.NewReader("%PDF-1.4\nfirst"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/job", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, old.ID); err != nil {
		t.Fatal(err)
	}
	token := reviewedSubmissionToken(t, a, 1)
	if err := a.recordSubmission(1, "submitted", token); err != nil {
		t.Fatal(err)
	}
	fresh, err := a.reviseResume(old.ID, "support-v2.pdf", strings.NewReader("%PDF-1.4\nsecond"))
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Version != 2 || fresh.SeriesID != old.ID || fresh.ArchivedAt != "" {
		t.Fatalf("revision metadata: %+v", fresh)
	}
	prior, err := a.resumeByID(old.ID)
	if err != nil || prior.ArchivedAt == "" {
		t.Fatalf("old resume not retired: %+v %v", prior, err)
	}
	retained, err := a.readResumeBytes(prior)
	if err != nil || string(retained) != "%PDF-1.4\nfirst" {
		t.Fatalf("historical bytes lost or changed: %q %v", retained, err)
	}
	job, err := a.jobByID(1)
	if err != nil || job.SelectedResumeID != old.ID {
		t.Fatalf("job selection changed: %+v %v", job, err)
	}
	history, err := a.jobSubmissions(1)
	if err != nil || len(history) != 1 || history[0].ResumeID != old.ID || history[0].ResumeSHA256 != old.SHA256 {
		t.Fatalf("submission evidence changed: %+v %v", history, err)
	}
	if err := a.assignResume(1, old.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("allowed archived selection: %v", err)
	}
	if err := a.deleteResume(old.ID); !errors.Is(err, ErrResumeInUse) {
		t.Fatalf("allowed deletion of submitted version: %v", err)
	}
	if _, err := a.reviseResume(fresh.ID, "same.pdf", strings.NewReader("%PDF-1.4\nsecond")); !errors.Is(err, ErrDuplicateResumeRevision) {
		t.Fatalf("duplicate revision: %v", err)
	}
	post := postApp(newMux(a), "/resumes/1/delete", url.Values{"confirm": {"yes"}, "confirm_phrase": {"DELETE"}}, "", "")
	if post.Code != http.StatusSeeOther || post.Header().Get("Location") != "/resumes?notice=in-use" {
		t.Fatalf("deletion left document app: %d %s", post.Code, post.Body.String())
	}
	notice := httptest.NewRecorder()
	newMux(a).ServeHTTP(notice, httptest.NewRequest("GET", "http://localhost"+post.Header().Get("Location"), nil))
	if notice.Code != 200 || !strings.Contains(notice.Body.String(), "Archive it") {
		t.Fatalf("in-app explanation absent: %d %s", notice.Code, notice.Body.String())
	}
	desktop := postApp(newDesktopMux(a), "/resumes/1/delete", url.Values{"confirm": {"yes"}, "confirm_phrase": {"DELETE"}}, "wails://localhost", "1")
	if desktop.Code != 200 || !strings.Contains(desktop.Body.String(), `window.location.replace("/resumes?notice=in-use")`) {
		t.Fatalf("desktop notice failed: %d %s", desktop.Code, desktop.Body.String())
	}
}

func TestMasterRevisionTransfersDesignation(t *testing.T) {
	a, _ := testWorkspace(t)
	master, err := a.importResume("master.pdf", strings.NewReader("%PDF-1.4\nmaster one"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	next, err := a.reviseResume(master.ID, "master-v2.pdf", strings.NewReader("%PDF-1.4\nmaster two"))
	if err != nil {
		t.Fatal(err)
	}
	prior, err := a.resumeByID(master.ID)
	if err != nil || prior.IsMaster || prior.ArchivedAt == "" {
		t.Fatalf("old master still active: %+v %v", prior, err)
	}
	now, err := a.masterResume()
	if err != nil || now.ID != next.ID || !next.IsMaster {
		t.Fatalf("new master not designated: %+v %+v %v", now, next, err)
	}
	if err := a.setResumeArchived(next.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.masterResume(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("archived master remains selected: %v", err)
	}
	if err := a.setResumeArchived(next.ID, false); err != nil {
		t.Fatal(err)
	}
	restored, err := a.resumeByID(next.ID)
	if err != nil || restored.IsMaster {
		t.Fatalf("restored version incorrectly re-mastered: %+v %v", restored, err)
	}
}

func TestMasterResumeRoutingAndSafeDeletion(t *testing.T) {
	a, _ := testWorkspace(t)
	first, err := a.importResume("master.docx", bytes.NewReader(sampleDOCX(t)))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nrole"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := a.importResume("other.pdf", strings.NewReader("%PDF-1.4\nother"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(first.ID, true); err != nil {
		t.Fatal(err)
	}
	master, err := a.masterResume()
	if err != nil || master.ID != first.ID || !master.IsMaster {
		t.Fatalf("master: %+v %v", master, err)
	}
	if err := a.assignResumeFamily(first.ID, 0, false); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("master accepted role-family assignment: %v", err)
	}
	if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("master accepted for employer attachment: %v", err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.deleteResume(role.ID); !errors.Is(err, ErrResumeInUse) {
		t.Fatalf("deleted job-selected resume: %v", err)
	}
	if err := a.setMasterResume(role.ID, true); !errors.Is(err, ErrResumeInUse) {
		t.Fatalf("reclassified job resume as master: %v", err)
	}
	if err := a.assignResume(1, 0); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	token := reviewedSubmissionToken(t, a, 1)
	if err := a.recordSubmission(1, "submitted", token); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, 0); err != nil {
		t.Fatal(err)
	}
	if err := a.deleteResume(role.ID); !errors.Is(err, ErrResumeInUse) {
		t.Fatalf("deleted historical resume: %v", err)
	}
	if err := a.deleteResume(first.ID); !errors.Is(err, ErrMasterResume) {
		t.Fatalf("deleted master resume: %v", err)
	}
	if err := a.setMasterResume(first.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(other.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(first.ID, true); err != nil {
		t.Fatal(err)
	}
	noLongerMaster, err := a.resumeByID(other.ID)
	if err != nil || noLongerMaster.IsMaster {
		t.Fatalf("old master still selected: %+v %v", noLongerMaster, err)
	}
	if err := a.deleteResume(other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.resumeByID(other.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted resume still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.resumeDir, other.StoredName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted file still exists: %v", err)
	}
}

func TestResumeLibraryMasterAndDeleteRoutes(t *testing.T) {
	a, _ := testWorkspace(t)
	r, err := a.importResume("career.docx", bytes.NewReader(sampleDOCX(t)))
	if err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/resumes", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), "/resumes/1/delete") || !strings.Contains(get.Body.String(), "Set as master resume") {
		t.Fatalf("resume controls not rendered: %d %s", get.Code, get.Body.String())
	}
	unconfirmed := postApp(mux, "/resumes/1/delete", url.Values{}, "", "")
	if unconfirmed.Code != 303 || unconfirmed.Header().Get("Location") != "/resumes?notice=failed" {
		t.Fatalf("deletion without confirmation should stay in app: %d", unconfirmed.Code)
	}
	crossSite := postApp(mux, "/resumes/1/master", url.Values{"master": {"yes"}}, "https://malicious.test", "")
	if crossSite.Code != 403 {
		t.Fatalf("cross-site master selection: %d", crossSite.Code)
	}
	mark := postApp(mux, "/resumes/1/master", url.Values{"master": {"yes"}}, "", "")
	if mark.Code != 303 {
		t.Fatalf("mark master: %d %s", mark.Code, mark.Body.String())
	}
	denied := postApp(mux, "/resumes/1/delete", url.Values{"confirm": {"yes"}, "confirm_phrase": {"DELETE"}}, "", "")
	if denied.Code != 303 || denied.Header().Get("Location") != "/resumes?notice=master" {
		t.Fatalf("master delete should redirect to library: %d %s", denied.Code, denied.Header().Get("Location"))
	}
	clear := postApp(newDesktopMux(a), "/resumes/1/master", url.Values{"master": {"no"}}, "wails://localhost", "1")
	if clear.Code != 200 || !strings.Contains(clear.Body.String(), `window.location.replace("/resumes")`) {
		t.Fatalf("desktop clear master: %d %s", clear.Code, clear.Body.String())
	}
	deleted := postApp(mux, "/resumes/1/delete", url.Values{"confirm": {"yes"}, "confirm_phrase": {"DELETE"}}, "", "")
	if deleted.Code != 303 {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body.String())
	}
	if _, err := a.resumeByID(r.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("resume survived deletion: %v", err)
	}
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
	token := reviewedSubmissionToken(t, a, 1)
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
		{"oversized.pdf", append([]byte("%PDF-1.4"), bytes.Repeat([]byte("X"), documents.MaxResumeSize)...)},
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
	req := httptest.NewRequest("POST", "http://localhost/resumes/import", &body)
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
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("review page=%d %s", get.Code, get.Body.String())
	}
	token := submissionTokenFromPage(t, get.Body.String())
	rec = postApp(mux, "/jobs/1/submit", url.Values{"confirm": {"yes"}, "notes": {"Applied with test"}, "submission_token": {token}}, "", "")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("submission=%d %s", rec.Code, rec.Body.String())
	}
	get = httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `name="submission_token"`) || !strings.Contains(get.Body.String(), "Applied with test") || !strings.Contains(get.Body.String(), "test.pdf") {
		t.Fatalf("detail=%d body=%s", get.Code, get.Body.String())
	}
	get = httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest("GET", "http://localhost/resumes/1/download", nil))
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

func TestSubmissionConfirmationUsesReviewedResume(t *testing.T) {
	newReviewedJob := func(t *testing.T) (*App, http.Handler, Resume, Resume) {
		t.Helper()
		a, _ := testWorkspace(t)
		if err := a.saveJob(Job{URL: "https://example.com/role", Host: "example.com", Title: "Sample"}); err != nil {
			t.Fatal(err)
		}
		resumeA, err := a.importResume("resume-a.pdf", strings.NewReader("%PDF-1.4\nresume A"))
		if err != nil {
			t.Fatal(err)
		}
		resumeB, err := a.importResume("resume-b.pdf", strings.NewReader("%PDF-1.4\nresume B"))
		if err != nil {
			t.Fatal(err)
		}
		if err := a.assignResume(1, resumeA.ID); err != nil {
			t.Fatal(err)
		}
		return a, newMux(a), resumeA, resumeB
	}
	review := func(t *testing.T, mux http.Handler) string {
		t.Helper()
		page := httptest.NewRecorder()
		mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
		if page.Code != http.StatusOK {
			t.Fatalf("review page=%d %s", page.Code, page.Body.String())
		}
		return submissionTokenFromPage(t, page.Body.String())
	}
	confirm := func(mux http.Handler, token string) *httptest.ResponseRecorder {
		return postApp(mux, "/jobs/1/submit", url.Values{
			"confirm": {"yes"}, "submission_token": {token},
		}, "", "")
	}

	t.Run("same resume after navigation", func(t *testing.T) {
		a, mux, resumeA, _ := newReviewedJob(t)
		token := review(t, mux)
		for _, path := range []string{"/applications", "/jobs/1"} {
			page := httptest.NewRecorder()
			mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
			if page.Code != http.StatusOK {
				t.Fatalf("navigation to %s: %d", path, page.Code)
			}
		}
		if response := confirm(mux, token); response.Code != http.StatusSeeOther {
			t.Fatalf("unchanged review confirmation=%d %s", response.Code, response.Body.String())
		}
		history, err := a.jobSubmissions(1)
		if err != nil || len(history) != 1 || history[0].ResumeID != resumeA.ID || history[0].ResumeSHA256 != resumeA.SHA256 {
			t.Fatalf("confirmed resume history=%+v err=%v", history, err)
		}
	})

	t.Run("selected resume switched", func(t *testing.T) {
		a, mux, _, resumeB := newReviewedJob(t)
		token := review(t, mux)
		if err := a.assignResume(1, resumeB.ID); err != nil {
			t.Fatal(err)
		}
		response := confirm(mux, token)
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "Review the application and resume again") {
			t.Fatalf("stale selection confirmation=%d %s", response.Code, response.Body.String())
		}
		if history, err := a.jobSubmissions(1); err != nil || len(history) != 0 {
			t.Fatalf("stale selection was recorded: %+v err=%v", history, err)
		}
	})

	for _, mutate := range []struct {
		name string
		fn   func(*testing.T, *App, Resume)
	}{
		{name: "resume modified", fn: func(t *testing.T, a *App, resume Resume) {
			if err := os.WriteFile(filepath.Join(a.resumeDir, resume.StoredName), []byte("%PDF-1.4\nchanged resume A"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "resume file deleted", fn: func(t *testing.T, a *App, resume Resume) {
			if err := os.Remove(filepath.Join(a.resumeDir, resume.StoredName)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			a, mux, resumeA, _ := newReviewedJob(t)
			token := review(t, mux)
			mutate.fn(t, a, resumeA)
			response := confirm(mux, token)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "Review the application and resume again") {
				t.Fatalf("stale file confirmation=%d %s", response.Code, response.Body.String())
			}
			if history, err := a.jobSubmissions(1); err != nil || len(history) != 0 {
				t.Fatalf("stale file was recorded: %+v err=%v", history, err)
			}
		})
	}
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
	token := reviewedSubmissionToken(t, a, 1)
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
	token := reviewedSubmissionToken(t, a, 1)
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
	req := httptest.NewRequest(http.MethodGet, "http://localhost/resumes/1/download", nil)
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
	newMux(a).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/resumes/1/download", nil))
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "modified") {
		t.Fatalf("altered artifact served: status=%d", w.Code)
	}
}

func TestAssessResumeHandoff(t *testing.T) {
	const ashby = "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"

	selected := &Resume{ID: 7}
	active := Job{
		URL:              ashby,
		Stage:            StageReview,
		SelectedResumeID: 7,
	}

	tests := []struct {
		name    string
		job     Job
		resume  *Resume
		allowed bool
	}{
		{"active Ashby", active, selected, true},
		{"missing document", active, nil, false},
		{"wrong document", active, &Resume{ID: 8}, false},
		{"closed", Job{URL: ashby, Stage: StageClosed, SelectedResumeID: 7}, selected, false},
		{"applied", Job{URL: ashby, Stage: StageApplied, SelectedResumeID: 7}, selected, false},
		{"no selection", Job{URL: ashby, Stage: StageReview}, nil, false},
		{"unsupported provider", Job{URL: "https://example.com/job", Stage: StageReview, SelectedResumeID: 7}, selected, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assessResumeHandoff(tt.job, tt.resume)

			if got.Allowed != tt.allowed {
				t.Errorf("Allowed = %v, want %v", got.Allowed, tt.allowed)
			}

			if got.Reason == "" {
				t.Error("missing explanation")
			}
		})
	}
}

func TestReadResumeBytesVerifiesIntegrity(t *testing.T) {
	a, _ := testWorkspace(t)
	original := []byte("%PDF-1.4\\nresume content")

	resume, err := a.importResume("source.pdf", bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}

	got, err := a.readResumeBytes(resume)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("verified read: %v", err)
	}

	stored := filepath.Join(a.resumeDir, resume.StoredName)
	if err := os.WriteFile(stored, []byte("%PDF-1.4\\naltered content"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := a.readResumeBytes(resume); err == nil {
		t.Fatal("accepted altered resume")
	}
}

func TestExtractResumeTextUsesVerifiedBytes(t *testing.T) {
	a, _ := testWorkspace(t)

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)

	types, err := archive.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(types, `<Types/>`)

	document, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}

	const xmlBody = `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Support</w:t></w:r><w:r><w:t xml:space="preserve"> Engineer</w:t></w:r></w:p><w:p><w:r><w:t>Linux &amp; MySQL troubleshooting</w:t></w:r></w:p></w:body></w:document>`

	if _, err := io.WriteString(document, xmlBody); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	resume, err := a.importResume("career.docx", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	got, err := a.extractResumeText(resume)
	if err != nil {
		t.Fatal(err)
	}

	if got == "" {
		t.Fatal("extracted text is empty")
	}

	// Extraction must not bypass document integrity verification.
	stored := filepath.Join(a.resumeDir, resume.StoredName)
	if err := os.WriteFile(stored, []byte("modified"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.extractResumeText(resume); err == nil {
		t.Fatal("extraction accepted altered document")
	}

	pdf, err := a.importResume(
		"career.pdf",
		strings.NewReader("%PDF-1.4\nsample"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.extractResumeText(pdf); err == nil {
		t.Fatal("PDF incorrectly reported extraction support")
	}
}

// A transaction failure must not strand a new revision or archive the old one.
func TestResumeRevisionFailureLeavesNoUnlinkedDocument(t *testing.T) {
	a, _ := testWorkspace(t)
	previous, err := a.importResume("support.pdf", strings.NewReader("%PDF-1.4\noriginal"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`CREATE TRIGGER fail_resume_archive BEFORE UPDATE OF archived_at ON resumes
        WHEN OLD.id=1 BEGIN SELECT RAISE(ABORT, 'forced revision failure'); END`); err != nil {
		t.Fatal(err)
	}

	if _, err := a.reviseResume(previous.ID, "support-v2.pdf", strings.NewReader("%PDF-1.4\nnew")); err == nil {
		t.Fatal("revision unexpectedly succeeded")
	}
	files, err := os.ReadDir(a.resumeDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("failed revision left an extra file: %v %v", files, err)
	}
	all, err := a.allResumes()
	if err != nil || len(all) != 1 || all[0].ID != previous.ID || all[0].ArchivedAt != "" {
		t.Fatalf("failed revision changed library: %+v %v", all, err)
	}
	if _, err := a.readResumeBytes(all[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`DROP TRIGGER fail_resume_archive`); err != nil {
		t.Fatal(err)
	}
	next, err := a.reviseResume(previous.ID, "support-v2.pdf", strings.NewReader("%PDF-1.4\nnew"))
	if err != nil || next.Version != 2 {
		t.Fatalf("retry revision failed: %+v %v", next, err)
	}
}

func sampleDOCXForPreview(t *testing.T, phrase string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	types, err := z.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(types, `<Types/>`); err != nil {
		t.Fatal(err)
	}
	doc, err := z.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	xmlBody := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + phrase + `</w:t></w:r></w:p></w:body></w:document>`
	if _, err = io.WriteString(doc, xmlBody); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCoverLetterSourcesPreviewIsReadOnlyAndEscaped(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/support", Host: "example.com", Title: "Support Engineer", Description: "Experience with production incidents"}); err != nil {
		t.Fatal(err)
	}
	master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Career &lt;facts&gt;")))
	if err != nil {
		t.Fatal(err)
	}
	role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Linux &amp; SQL")))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.setMasterResume(master.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, role.ID); err != nil {
		t.Fatal(err)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
	html := page.Body.String()
	if page.Code != 200 || !strings.Contains(html, `id="letter-studio-title"`) || !strings.Contains(html, "Master resume readable") ||
		!strings.Contains(html, "Career &lt;facts&gt;") || strings.Contains(html, "Career <facts>") ||
		!strings.Contains(html, "Linux &amp; SQL") || page.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("bad cover letter source preview: %d %s", page.Code, html)
	}
	// Removing the assigned role resume must leave an explanatory preview,
	// not a broken page or a silently substituted master resume.
	if err := a.assignResume(1, 0); err != nil {
		t.Fatal(err)
	}
	page = httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Selected resume unavailable") ||
		strings.Contains(page.Body.String(), "Selected resume readable") {
		t.Fatal("missing resume was treated as ready")
	}
	pdf, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nrole evidence"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, pdf.ID); err != nil {
		t.Fatal(err)
	}
	page = httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://localhost/jobs/1", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "This PDF has no readable text") ||
		strings.Contains(page.Body.String(), "Selected resume readable") {
		t.Fatal("unsupported PDF was treated as readable")
	}
}
