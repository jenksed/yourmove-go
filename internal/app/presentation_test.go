package app

import (
	"bytes"
	"html"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHumanDateRetainsUnknownAndFormatsRecordedDates(t *testing.T) {
	stamp := "2026-10-25T15:22:41.123456789Z"
	parsed, _ := time.Parse(time.RFC3339Nano, stamp)
	if got := humanDate(stamp); got != parsed.Local().Format("Jan 2, 2006") {
		t.Fatal(got)
	}
	if humanDate("") != "Not recorded" || humanDate("earlier record") != "earlier record" {
		t.Fatal("unknown dates must remain truthful")
	}
}

func TestSharedMaterialReturnContextIsLocalValidatedAndOptional(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/return", Title: "Return role"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/candidate", "/resumes"} {
		for _, query := range []string{"", "?return_job=1", "?return_job=01", "?return_job=-1", "?return_job=999", "?return_job=https%3A%2F%2Fevil.test", "?return_job=1&return_job=1"} {
			w := httptest.NewRecorder()
			newMux(a).ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+path+query, nil))
			want := query == "?return_job=1"
			if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "data-return-job") != want {
				t.Fatalf("%s%s status=%d contextual=%t", path, query, w.Code, !want)
			}
			if want && (!strings.Contains(w.Body.String(), `href="/jobs/1#prepare"`) || !strings.Contains(w.Body.String(), `name="return_job" value="1"`)) {
				t.Fatalf("return context missing on %s", path)
			}
		}
	}
	job, _ := a.jobByID(1)
	if job.SelectedResumeID != 0 || job.SubmissionCount != 0 {
		t.Fatal("read-only shared navigation changed application materials")
	}
}

func TestProfileValidationRetainsSubmittedFactsConsentAndApprovedRevision(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/profile", Title: "Profile role"}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Previously", LastName: "Approved", Email: "prior@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := a.candidateProfile()
	form := url.Values{
		"first_name": {"Alex <Fixture>"}, "last_name": {""}, "email": {""}, "phone": {"555-0100"},
		"city": {"Montréal"}, "region": {"Québec"}, "country": {"Canada"}, "linkedin": {"https://example.test/alex"},
		"github": {"https://example.test/code"}, "portfolio": {"https://example.test/work"}, "pronouns": {"They/them"},
		"approve": {"yes"}, "return_job": {"1"},
	}
	failed := postApp(newMux(a), "/candidate", form, "", "")
	if failed.Code != http.StatusBadRequest {
		t.Fatal(failed.Code)
	}
	for _, want := range []string{html.EscapeString(form.Get("first_name")), "Montréal", "Québec", form.Get("phone"), form.Get("linkedin"), form.Get("github"), form.Get("portfolio"), `name="approve" value="yes" checked`, `name="return_job" value="1"`, `href="#last_name"`, `href="#email"`, `aria-describedby="last-name-error"`, `data-profile-validation-summary`} {
		if !strings.Contains(failed.Body.String(), want) {
			t.Errorf("validation recovery missing %s", want)
		}
	}
	after, _ := a.candidateProfile()
	if after != before {
		t.Fatalf("invalid approval altered saved revision: before=%+v after=%+v", before, after)
	}
	form.Set("last_name", "Example")
	form.Set("email", "invalid")
	failed = postApp(newMux(a), "/candidate", form, "", "")
	if failed.Code != http.StatusBadRequest || !strings.Contains(failed.Body.String(), `id="email-error"`) || !strings.Contains(failed.Body.String(), `value="invalid"`) {
		t.Fatal("email correction was not retained and associated")
	}
	form.Set("email", "alex@example.test")
	saved := postApp(newMux(a), "/candidate", form, "", "")
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/candidate?notice=saved&return_job=1" {
		t.Fatal(saved.Code, saved.Header(), saved.Body.String())
	}
	approved, _ := a.candidateProfile()
	if !approved.Approved || approved.Revision != before.Revision+1 {
		t.Fatal("correction did not save one approved revision")
	}
	form.Del("approve")
	form.Set("last_name", "")
	form.Set("email", "")
	saved = postApp(newMux(a), "/candidate", form, "", "")
	draft, _ := a.candidateProfile()
	if saved.Code != http.StatusSeeOther || draft.Approved || draft.Revision != approved.Revision+1 {
		t.Fatal("incomplete draft must save without approval")
	}
}

func TestContextualResumeImportAndNativeCancelRetainApplication(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/import", Title: "Import role"}); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("return_job", "1")
	file, err := form.CreateFormFile("resume", "Alex—Résumé.pdf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("%PDF-1.4\nsynthetic resume"))
	_ = form.Close()
	request := httptest.NewRequest("POST", "http://localhost/resumes/import", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, request)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/resumes?notice=imported&return_job=1" {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
	a.pickResume = func() (string, error) { return "", nil }
	canceled := postApp(newDesktopMux(a), "/resumes/import-native", url.Values{"return_job": {"1"}}, "wails://localhost", "1")
	if canceled.Code != http.StatusOK || !strings.Contains(canceled.Body.String(), `window.location.replace("/resumes?notice=canceled\u0026return_job=1")`) {
		t.Fatal(canceled.Code, canceled.Body.String())
	}
	resumes, _ := a.allResumes()
	job, _ := a.jobByID(1)
	if len(resumes) != 1 || resumes[0].Name != "Alex—Résumé.pdf" || job.SelectedResumeID != 0 || job.SubmissionCount != 0 {
		t.Fatal("import/cancel changed application assignment or bytes inventory")
	}
}

func TestSharedMaterialDesktopRedirectsRejectUntrustedContext(t *testing.T) {
	for _, path := range []string{"/candidate?notice=saved", "/candidate?return_job=1", "/candidate?notice=saved&return_job=1", "/resumes?notice=imported&return_job=1", "/resumes?notice=canceled", "/resumes?notice=archived&return_job=1"} {
		if !safeDesktopRedirectTarget(path) {
			t.Errorf("rejected safe shared context %s", path)
		}
	}
	for _, path := range []string{"https://evil.test/candidate?return_job=1", "//evil.test/resumes?notice=imported", "/candidate?return_job=01", "/candidate?return_job=0", "/candidate?return_job=1&return_job=2", "/candidate?return_job=1&next=https://evil.test", "/candidate?notice=imported", "/resumes?notice=saved", "/candidate?return_job=1#prepare", "/candidate?return_job=%31", "/resumes?return_job=-1"} {
		if safeDesktopRedirectTarget(path) {
			t.Errorf("accepted untrusted shared context %s", path)
		}
	}
}

func TestDocumentMaintenanceKeepsReturnContextAndExistingJobSelection(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/document", Title: "Document role"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("Synthetic résumé.pdf", strings.NewReader("%PDF-1.4\nsynthetic document"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []struct {
		path string
		form url.Values
		want string
	}{
		{"/resumes/1/family", url.Values{"return_job": {"1"}, "family_id": {"0"}}, "/resumes?return_job=1"},
		{"/resumes/1/master", url.Values{"return_job": {"1"}, "master": {"no"}}, "/resumes?return_job=1"},
		{"/resumes/1/archive", url.Values{"return_job": {"1"}, "action": {"archive"}, "confirm": {"yes"}}, "/resumes?notice=archived&return_job=1"},
		{"/resumes/1/archive", url.Values{"return_job": {"1"}, "action": {"restore"}}, "/resumes?notice=restored&return_job=1"},
	} {
		response := postApp(newMux(a), operation.path, operation.form, "", "")
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != operation.want || !safeDesktopRedirectTarget(operation.want) {
			t.Fatalf("maintenance %s lost context: %d %s", operation.path, response.Code, response.Header().Get("Location"))
		}
		job, _ := a.jobByID(1)
		if job.SelectedResumeID != resume.ID || job.SubmissionCount != 0 {
			t.Fatal("document maintenance silently reassigned or completed an application")
		}
	}
}
