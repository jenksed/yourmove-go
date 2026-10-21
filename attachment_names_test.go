package main

import (
	"bytes"
	"mime"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAttachmentFilename(t *testing.T) {
	p := CandidateProfile{FirstName: "Casey", LastName: "Smith", Approved: true}
	j := Job{Title: "Customer Support Engineer (Tier 1)", Company: "Tailscale"}
	for _, doc := range []string{"Resume", "Cover_Letter"} {
		got := attachmentFilename(p, j, doc, ".pdf")
		want := "Casey_Smith_" + doc + "_Customer_Support_Engineer_Tier_1_Tailscale.pdf"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
	p.Approved = false
	if got := attachmentFilename(p, Job{}, "Resume", ".docx"); got != "Resume.docx" {
		t.Fatalf("unapproved identity included: %q", got)
	}
	j = Job{Title: "../../Support\r\nEngineer: \"Tier 1\"", Company: strings.Repeat("x", 300)}
	got := attachmentFilename(CandidateProfile{}, j, "Resume", ".pdf")
	if filepath.Base(got) != got || strings.ContainsAny(got, "/\\\r\n\"") || len(got) > 150 || !strings.HasSuffix(got, ".pdf") {
		t.Fatalf("unsafe or excessive filename: %q", got)
	}
	if got := attachmentFilename(p, Job{Title: strings.Repeat("界", 100), Company: strings.Repeat("界", 100)}, "Resume", ".pdf"); len(got) > 200 {
		t.Fatalf("multibyte filename exceeds filesystem limit: %q", got)
	}
}

func TestResumeDownloadUsesPublicFilenameAndExactBytes(t *testing.T) {
	a, _ := testWorkspace(t)
	data := sampleDOCXForPreview(t, "Verified resume contents")
	resume, err := a.importResume("internal-draft-v27.docx", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Smith", Email: "casey@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/resumes/1/download", nil))
	_, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	if rec.Code != 200 || err != nil || params["filename"] != "Casey_Smith_Resume.docx" || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("download: code=%d disposition=%q err=%v", rec.Code, rec.Header().Get("Content-Disposition"), err)
	}
	saved, err := a.resumeByID(resume.ID)
	if err != nil || saved.Name != "internal-draft-v27.docx" || saved.SHA256 != resume.SHA256 {
		t.Fatalf("source document changed: %+v %v", saved, err)
	}
	if err := a.saveJob(Job{URL: "https://example.test/role", Host: "example.test", Title: "Support Engineer", Company: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/resumes/1/download?job_id=1", nil))
	_, params, err = mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	if err != nil || params["filename"] != "Casey_Smith_Resume_Support_Engineer_Acme.docx" || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("job download: %q %v", rec.Header().Get("Content-Disposition"), err)
	}
}

func TestSavedCoverLetterDownload(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.test/role", Host: "example.test", Title: "Support Engineer", Company: "Acme"}); err != nil {
		t.Fatal(err)
	}
	body := "Dear Hiring Team, I bring verified technical support experience to this role."
	if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at) VALUES(1,?,'ollama','test',1,'master',1,'role','job','now')`, body); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Smith", Email: "casey@example.test", Approved: true}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost/jobs/1/cover-letter/1/download", nil))
	_, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	expected, pdfErr := coverLetterPDF(body)
	if rec.Code != 200 || err != nil || pdfErr != nil || params["filename"] != "Casey_Smith_Cover_Letter_Support_Engineer_Acme.pdf" || !bytes.Equal(rec.Body.Bytes(), expected) {
		t.Fatalf("letter download: %d %q %v", rec.Code, rec.Header().Get("Content-Disposition"), err)
	}
	for _, path := range []string{"/jobs/1/cover-letter/2/download", "/jobs/2/cover-letter/1/download"} {
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost"+path, nil))
		if rec.Code != 404 {
			t.Fatalf("draft identity not enforced: %s %d", path, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost/jobs/1/cover-letter/1/download", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatal("cross-site private download allowed")
	}
}
