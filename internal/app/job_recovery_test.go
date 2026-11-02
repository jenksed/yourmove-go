package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestUniversalRecoverySavedEditedRefreshedAndRestarted(t *testing.T) {
	for _, posting := range []string{"https://unknown.example/jobs/123?token=private", "https://jobs.ashbyhq.com/unsupported-variant", "https://careers.example.icims.com/jobs/123/role/job", "https://example.taleo.net/careersection/1/jobdetail.ftl?job=123", "https://example.avature.net/careers/JobDetail/123"} {
		t.Run(posting, func(t *testing.T) {
			a, dir := testWorkspace(t)
			// Production intake dispatch: unsupported variants must never need retrieval.
			if strings.Contains(posting, "ashbyhq") {
				a.enrichmentFetch = func(_ context.Context, _ string) (platforms.Metadata, string, string) {
					return platforms.Metadata{}, "failed", "unavailable"
				}
			}
			response := postSyntheticJob(t, a, posting)
			if response.Code != 303 || response.Header().Get("Location") != "/jobs/1#overview" {
				t.Fatalf("save: %d %s", response.Code, response.Body.String())
			}
			description := "Responsibilities\n• Build accessible tools\n\nQualifications\n• Go experience\n<script>hostile()</script>"
			edit := url.Values{"title": {"Engineer"}, "company": {"Synthetic Employer"}, "description": {description}, "stage": {"saved"}, "priority": {"normal"}}
			job, _ := a.jobByID(1)
			edit.Set("stage", string(job.Stage))
			edit.Set("priority", string(job.Priority))
			response = postApp(newMux(a), "/jobs/1/edit", edit, "", "")
			if response.Code != 303 {
				t.Fatalf("edit: %d %s", response.Code, response.Body.String())
			}
			if err := a.applyPostingRefresh(1, platforms.Metadata{}, "failed", "temporary failure", "2026-10-24T12:00:00Z"); err != nil {
				t.Fatal(err)
			}
			job, _ = a.jobByID(1)
			if job.Description != description || !job.DescriptionManual || job.Title != "Engineer" {
				t.Fatalf("lost edit: %+v", job)
			}
			rr := httptest.NewRecorder()
			newMux(a).ServeHTTP(rr, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
			if rr.Code != 200 || strings.Contains(rr.Body.String(), "<script>hostile()") || !strings.Contains(rr.Body.String(), "Your saved description") {
				t.Fatalf("unsafe/unreadable overview %d", rr.Code)
			}
			diagnostic := diagnosticForJob(job)
			raw, _ := json.Marshal(diagnostic)
			for _, secret := range []string{"private", "token", "Synthetic Employer", "hostile", "Engineer", "123"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("diagnostic leaked %q: %s", secret, raw)
				}
			}
			db, err := storage.OpenDB(filepath.Join(dir, "jobs.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			reopened := newTestApp(t, db, a.resumeDir)
			saved, err := reopened.jobByID(1)
			if err != nil || saved.Description != description || !saved.DescriptionManual {
				t.Fatalf("restart: %+v %v", saved, err)
			}
		})
	}
}

func TestRecoveryDiagnosticsRequireReviewAndExcludeContent(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.org/private-path?secret=query-secret-value", Host: "example.org", Description: "private applicant text", SourceNote: "credential detail", SourceStatus: "failed"}); err != nil {
		t.Fatal(err)
	}
	mux := newMux(a)
	denied := postApp(mux, "/jobs/1/diagnostics", url.Values{}, "", "")
	if denied.Code != 400 {
		t.Fatalf("unreviewed export: %d", denied.Code)
	}
	exported := postApp(mux, "/jobs/1/diagnostics", url.Values{"reviewed": {"yes"}}, "", "")
	if exported.Code != 200 || !strings.Contains(exported.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: %d", exported.Code)
	}
	for _, secret := range []string{"private-path", "query-secret-value", "private applicant text", "credential detail"} {
		if strings.Contains(exported.Body.String(), secret) {
			t.Fatalf("leaked %q", secret)
		}
	}
}

func TestRecoveryFirstSaveNeverFetchesAndRetainsDuplicate(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
		t.Fatal("saving must not retrieve")
		return platforms.Metadata{}, "failed", ""
	}
	for _, raw := range []string{"https://unknown.example/job/1", "https://unknown.example/job/1#details"} {
		response := postApp(newMux(a), "/jobs", url.Values{"job_url": {raw}, "save_first": {"1"}}, "", "")
		if response.Code != 303 || response.Header().Get("Location") != "/jobs/1#overview" {
			t.Fatalf("save first: %d %s", response.Code, response.Body.String())
		}
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 1 || jobs[0].SourceStatus != "not_checked" {
		t.Fatalf("jobs: %+v %v", jobs, err)
	}
	large := strings.Repeat("Unicode café • text\n", 4000)
	response := postApp(newMux(a), "/jobs/1/edit", url.Values{"description": {large}, "stage": {jobs[0].Stage}, "priority": {jobs[0].Priority}}, "", "")
	if response.Code != 303 {
		t.Fatalf("large pasted description rejected: %d %s", response.Code, response.Body.String())
	}
	saved, _ := a.jobByID(1)
	if saved.Description != strings.TrimSpace(large) {
		t.Fatal("large input changed")
	}
}
