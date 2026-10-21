package main

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverLetterSourceFailuresIdentifyExactRecovery(t *testing.T) {
	for _, tc := range []struct{ name, code, message string }{
		{"missing description", "source-description", "No job description is saved"},
		{"missing master", "source-master-missing", "No master resume is selected"},
		{"missing selection", "source-resume-missing", "Choose an available application resume"},
		{"unreadable master", "source-master-unreadable", "The designated master resume"},
		{"unreadable selection", "source-resume-unreadable", "The resume selected under Resume to emphasize"},
		{"master selected as role", "source-resume-invalid", "Choose an active application resume"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := testWorkspace(t)
			if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Description: "Resolve production incidents"}); err != nil {
				t.Fatal(err)
			}
			master, err := a.importResume("master.docx", bytes.NewReader(sampleDOCXForPreview(t, "Complete verified career evidence")))
			if err != nil {
				t.Fatal(err)
			}
			role, err := a.importResume("role.docx", bytes.NewReader(sampleDOCXForPreview(t, "Relevant verified support evidence")))
			if err != nil {
				t.Fatal(err)
			}
			if tc.name != "missing master" {
				if err = a.setMasterResume(master.ID, true); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name != "missing selection" {
				if err = a.assignResume(1, role.ID); err != nil {
					t.Fatal(err)
				}
			}
			sourceID := int64(0)
			switch tc.name {
			case "missing description":
				_, err = a.db.Exec("UPDATE jobs SET description='' WHERE id=1")
			case "unreadable master":
				err = os.WriteFile(filepath.Join(a.resumeDir, master.StoredName), []byte("changed fixture"), 0600)
			case "unreadable selection":
				err = os.WriteFile(filepath.Join(a.resumeDir, role.StoredName), []byte("changed fixture"), 0600)
			case "master selected as role":
				sourceID = master.ID
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = a.letterEvidenceForJob(1, sourceID)
			var source coverSourceError
			if !errors.Is(err, errCoverSources) || !errors.As(err, &source) || string(source) != tc.code {
				t.Fatalf("source error: %v", err)
			}
			calls := 0
			a.coverLetterGenerate = func(context.Context, string) (string, error) {
				calls++
				return "Unexpected generation must not happen on source failure", nil
			}
			values := url.Values{"confirm_local": {"yes"}}
			if sourceID > 0 {
				values.Set("letter_resume_id", formatJobID(sourceID))
			}
			rec := postApp(newMux(a), "/jobs/1/cover-letter/generate", values, "", "")
			if rec.Code != 303 || rec.Header().Get("Location") != coverNoticeURL(1, tc.code) || calls != 0 {
				t.Fatalf("source failure reached generator or wrong redirect: %d %s calls=%d", rec.Code, rec.Header(), calls)
			}
			page := httptest.NewRecorder()
			newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1?cover="+tc.code, nil))
			if page.Code != 200 || !strings.Contains(page.Body.String(), tc.message) {
				t.Fatalf("missing precise recovery: %d %s", page.Code, page.Body.String())
			}
			if tc.name == "missing master" {
				// An imported master-named file is not yet designated. Use the ordinary
				// configuration operation, then confirm the same sources become usable.
				if err = a.setMasterResume(master.ID, true); err != nil {
					t.Fatal(err)
				}
				evidence, err := a.letterEvidenceForJob(1, 0)
				if err != nil || evidence.master.ID != master.ID || evidence.role.ID != role.ID {
					t.Fatalf("master designation did not resolve source blocker: %+v %v", evidence, err)
				}
			}
		})
	}
}
