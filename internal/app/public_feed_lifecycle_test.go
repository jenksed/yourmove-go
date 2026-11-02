package app

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// This qualifies the new public-feed adapters at the application's real intake
// and refresh boundaries. All feed bodies are sanitized synthetic fixtures.
func TestPublicFeedProvidersUseProductionIntakeAndRetainSourceAcrossRefreshFailureAndRestart(t *testing.T) {
	readFixture := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "public-feeds", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	providers := []struct {
		name, file, postingURL, requestKey, provider, postingID, oldText, newText string
	}{
		{"Personio", "personio.xml", "https://example.jobs.personio.de/job/12345?language=en", "example.jobs.personio.de/xml", "personio", "12345", "Build reliable applicant-controlled systems.", "Build verifiable applicant-controlled systems."},
		{"Teamtailor", "teamtailor.xml", "https://example.teamtailor.com/jobs/12345-platform-engineer", "example.teamtailor.com/jobs.rss", "teamtailor", "12345", "Build safe services.", "Build verifiable services."},
		{"Recruitee", "recruitee.xml", "https://example.recruitee.com/o/platform-engineer", "example.recruitee.com/api/feeds/offers.xml", "recruitee", "platform-engineer", "Build safe services.", "Build verifiable services."},
	}
	responses := make(map[string][][]byte, len(providers))
	for _, provider := range providers {
		initial := readFixture(provider.file)
		refreshed := bytes.Replace(initial, []byte(provider.oldText), []byte(provider.newText), 1)
		if bytes.Equal(initial, refreshed) {
			t.Fatalf("%s fixture did not contain refresh anchor %q", provider.name, provider.oldText)
		}
		responses[provider.requestKey] = [][]byte{initial, refreshed}
	}
	previousTransport := http.DefaultTransport
	transport := &m3SourceTransport{fallback: previousTransport, responses: responses}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			app, workspaceDir := testWorkspace(t)
			router := newMux(app)
			intake := m3Request(router, http.MethodPost, "/jobs", url.Values{"job_url": {provider.postingURL}})
			if intake.Code != http.StatusSeeOther || intake.Header().Get("Location") != "/jobs" {
				t.Fatalf("production intake = %d Location=%q body=%s", intake.Code, intake.Header().Get("Location"), intake.Body.String())
			}
			var jobID int64
			if err := app.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, provider.postingURL).Scan(&jobID); err != nil {
				t.Fatalf("load intaken posting: %v", err)
			}
			// The saved id is authoritative even when the provider URL has a query.
			path := "/jobs/" + strconv.FormatInt(jobID, 10)
			job, err := app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			if job.Title != "Platform Engineer" || !strings.Contains(job.Description, provider.oldText) {
				t.Fatalf("intake canonical projection = title %q description %q", job.Title, job.Description)
			}
			state, found, err := loadJobPostingSource(app.db, job)
			if err != nil || !found || state.Invalid || state.Source == nil {
				t.Fatalf("intake source persistence: found=%v state=%+v err=%v", found, state, err)
			}
			if state.Source.Identity.Provider != provider.provider || state.Source.Identity.PostingID != provider.postingID || state.LatestSuccessAt == "" {
				t.Fatalf("intake source identity/freshness = %+v, want provider=%s posting=%s", state, provider.provider, provider.postingID)
			}
			brief := m3Request(router, http.MethodGet, path+"/brief", nil)
			if brief.Code != http.StatusOK || !strings.Contains(brief.Body.String(), provider.oldText) {
				t.Fatalf("application brief lost canonical intake description: status=%d", brief.Code)
			}

			manualDescription := "Applicant-owned correction for " + provider.name + "."
			edit := url.Values{"title": {job.Title}, "company": {job.Company}, "description": {manualDescription}, "salary_text": {"Applicant verified compensation"}, "stage": {job.Stage}, "priority": {job.Priority}}
			if response := m3Request(router, http.MethodPost, path+"/edit", edit); response.Code != http.StatusSeeOther {
				t.Fatalf("manual edit response=%d body=%s", response.Code, response.Body.String())
			}
			previousSuccessAt := state.LatestSuccessAt
			waitForPostingFreshnessAdvance(t, previousSuccessAt)
			if response := m3Request(router, http.MethodPost, path+"/refresh", nil); response.Code != http.StatusSeeOther {
				t.Fatalf("provider refresh response=%d body=%s", response.Code, response.Body.String())
			}
			job, err = app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			state, found, err = loadJobPostingSource(app.db, job)
			if err != nil || !found || state.Invalid || state.Source == nil ||
				job.Description != manualDescription || !job.DescriptionManual || job.SalaryText != "Applicant verified compensation" || !job.SalaryManual ||
				state.Source.Identity.Provider != provider.provider || !strings.Contains(state.Source.DescriptionHTML+state.Source.DescriptionPlain, provider.newText) {
				t.Fatalf("successful refresh lost manual values or refreshed source: job=%+v state=%+v found=%v err=%v", job, state, found, err)
			}
			successAt := state.LatestSuccessAt

			// Exercise the normal HTTP refresh handler with a deterministic failed
			// fetch. A failure advances attempt status but keeps the last good source.
			app.enrichmentFetch = func(context.Context, string) (platforms.Metadata, string, string) {
				return platforms.Metadata{}, "failed", "synthetic feed outage"
			}
			waitForPostingFreshnessAdvance(t, job.SourceCheckedAt)
			if response := m3Request(router, http.MethodPost, path+"/refresh", nil); response.Code != http.StatusSeeOther {
				t.Fatalf("failed refresh response=%d body=%s", response.Code, response.Body.String())
			}
			job, err = app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			state, found, err = loadJobPostingSource(app.db, job)
			if err != nil || !found || state.Invalid || state.Source == nil || state.LatestSuccessAt != successAt ||
				job.SourceStatus != "failed" || job.SourceNote != "synthetic feed outage" || job.Description != manualDescription ||
				job.SalaryText != "Applicant verified compensation" || !strings.Contains(state.Source.DescriptionHTML+state.Source.DescriptionPlain, provider.newText) {
				t.Fatalf("failed refresh did not retain last successful source/manual values: job=%+v state=%+v found=%v err=%v", job, state, found, err)
			}
			brief = m3Request(router, http.MethodGet, path+"/brief", nil)
			if brief.Code != http.StatusOK || !strings.Contains(brief.Body.String(), manualDescription) {
				t.Fatalf("brief did not preserve applicant correction after refresh failure: status=%d", brief.Code)
			}

			if err := app.db.Close(); err != nil {
				t.Fatal(err)
			}
			reopenedDB, err := storage.OpenDB(filepath.Join(workspaceDir, "jobs.db"))
			if err != nil {
				t.Fatalf("reopen workspace DB: %v", err)
			}
			t.Cleanup(func() { reopenedDB.Close() })
			restarted := newTestApp(t, reopenedDB, filepath.Join(workspaceDir, "resumes"))
			restartedJob, err := restarted.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			restartedState, found, err := loadJobPostingSource(restarted.db, restartedJob)
			if err != nil || !found || restartedState.Invalid || restartedState.Source == nil ||
				restartedState.Source.Identity.Provider != provider.provider || restartedState.Source.Identity.PostingID != provider.postingID ||
				restartedState.LatestSuccessAt != successAt || restartedJob.SourceStatus != "failed" ||
				restartedJob.Description != manualDescription || !restartedJob.DescriptionManual || restartedJob.SalaryText != "Applicant verified compensation" || !restartedJob.SalaryManual ||
				!strings.Contains(restartedState.Source.DescriptionHTML+restartedState.Source.DescriptionPlain, provider.newText) {
				t.Fatalf("restart lost source provenance or applicant-owned values: job=%+v state=%+v found=%v err=%v", restartedJob, restartedState, found, err)
			}
			restartedBrief := m3Request(newMux(restarted), http.MethodGet, path+"/brief", nil)
			if restartedBrief.Code != http.StatusOK || !strings.Contains(restartedBrief.Body.String(), manualDescription) {
				t.Fatalf("restarted brief lost manual description: status=%d", restartedBrief.Code)
			}
		})
	}
}
