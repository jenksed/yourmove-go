package app

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestJobIntelligenceCompetitiveProvidersUseProductionLifecycle(t *testing.T) {
	readFixture := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	workdayFixture := readFixture("workday-detail-synthetic.json")
	smartRecruitersFixture := readFixture("smartrecruiters-posting-synthetic.json")
	workableFixture := readFixture("workable-widget-synthetic.json")
	jobviteFixture := readFixture("jobvite-posting-synthetic.html")

	workdayRefresh := bytes.Replace(workdayFixture, []byte("Build safe systems."), []byte("Build verifiable systems."), 1)
	smartRecruitersRefresh := bytes.Replace(smartRecruitersFixture, []byte("Own the API."), []byte("Own the verified API."), 1)
	workableRefresh := bytes.Replace(workableFixture, []byte("Help keep Northstar's platform reliable and secure."), []byte("Help keep Northstar's platform verifiably reliable."), 1)
	jobviteRefresh := bytes.Replace(jobviteFixture, []byte("Design reliable services."), []byte("Design auditable services."), 1)

	const (
		workdayID         = "Platform-Engineer_R-12345"
		smartRecruitersID = "744000123456789"
		workableID        = "9AB12CD345"
		jobviteID         = "jobvite001"
	)
	providers := []struct {
		name, postingURL, requestKey, provider, board, region, postingID string
		title, company, canonical, refreshedContent                      string
		location, workplace, employment                                  string
	}{
		{
			name: "Workday", postingURL: "https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle-WA/" + workdayID,
			requestKey: "acme.wd5.myworkdayjobs.com/wday/cxs/acme/External/job/Seattle-WA/" + workdayID,
			provider:   "workday", board: "acme.wd5.myworkdayjobs.com/acme/External", region: "en-US", postingID: "Seattle-WA/" + workdayID,
			title: "Platform Engineer", company: "Fixture Systems",
			canonical:        "Role overview\nBuild safe systems.\nWhat you will do\n• Design APIs.\n• Review code.\nQualifications\nExperience shipping Go services.",
			refreshedContent: "Build verifiable systems.", location: "Seattle, WA; Portland, OR", workplace: "Hybrid", employment: "Full time",
		},
		{
			name: "SmartRecruiters", postingURL: "https://jobs.smartrecruiters.com/sampleco/" + smartRecruitersID,
			requestKey: "api.smartrecruiters.com/v1/companies/sampleco/postings/" + smartRecruitersID,
			provider:   "smartrecruiters", board: "sampleco", postingID: smartRecruitersID,
			title: "Senior API Engineer", company: "Sample Company",
			canonical:        "Company Description\nWe build tools & services.\nJob Description\nOwn the API.\n• Ship safely\n• Keep latency low\nQualifications\nThree years of Go.\nAdditional Information\nRemote within Canada.",
			refreshedContent: "Own the verified API.", location: "Montreal, QC, Canada", workplace: "Remote", employment: "Full-time",
		},
		{
			name: "Workable", postingURL: "https://apply.workable.com/northstar/j/" + workableID,
			requestKey: "apply.workable.com/api/v1/widget/accounts/northstar",
			provider:   "workable", board: "northstar", postingID: workableID,
			title: "Senior Site Reliability Engineer", company: "Northstar Systems",
			canonical:        "Help keep Northstar's platform reliable and secure.\nResponsibilities\n• Improve service health checks.\n• Review incident patterns.\nQualifications\nExperience operating production services.\nBenefits\nFlexible leave and learning time.",
			refreshedContent: "Help keep Northstar's platform verifiably reliable.", location: "Detroit, Michigan, United States", workplace: "remote", employment: "Full-time",
		},
		{
			name: "Jobvite", postingURL: "https://jobs.jobvite.com/syntheticco/job/" + jobviteID + "?source=fixture",
			requestKey: "jobs.jobvite.com/syntheticco/job/" + jobviteID,
			provider:   "jobvite", board: "syntheticco", postingID: jobviteID,
			title: "Synthetic Engineering Lead", company: "Example Company",
			canonical:        "Role summary\nBuild the example platform with care.\nResponsibilities\n• Design reliable services.\n• Review changes.\nQualifications\nExperience with Go.",
			refreshedContent: "Design auditable services.", location: "Denver, Colorado, United States; Boston, Massachusetts, United States", workplace: "remote", employment: "FULL_TIME",
		},
	}

	responses := make(map[string][][]byte, len(providers))
	for _, provider := range providers {
		initial := map[string][]byte{
			"Workday":         workdayFixture,
			"SmartRecruiters": smartRecruitersFixture,
			"Workable":        workableFixture,
			"Jobvite":         jobviteFixture,
		}[provider.name]
		refreshed := map[string][]byte{
			"Workday":         workdayRefresh,
			"SmartRecruiters": smartRecruitersRefresh,
			"Workable":        workableRefresh,
			"Jobvite":         jobviteRefresh,
		}[provider.name]
		responses[provider.requestKey] = [][]byte{initial, refreshed}
	}
	previousTransport := http.DefaultTransport
	transport := &m3SourceTransport{fallback: previousTransport, responses: responses}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	app, workspaceDir := testWorkspace(t)
	app.enrichmentFetch = fetchPostingDetails
	router := newMux(app)
	jobIDs := make(map[string]int64, len(providers))
	manualDescriptions := make(map[string]string, len(providers))
	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			response := m3Request(router, http.MethodPost, "/jobs", url.Values{"job_url": {provider.postingURL}})
			if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/jobs" {
				t.Fatalf("production intake = %d Location=%q body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
			}
			var jobID int64
			if err := app.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, provider.postingURL).Scan(&jobID); err != nil {
				t.Fatalf("load intaken posting: %v", err)
			}
			jobIDs[provider.name] = jobID
			job, err := app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			if job.Title != provider.title || job.Company != provider.company || job.Description != provider.canonical {
				t.Fatalf("canonical intake projection changed: got title=%q company=%q description=%q; want title=%q company=%q description=%q", job.Title, job.Company, job.Description, provider.title, provider.company, provider.canonical)
			}
			sourceState, found, err := loadJobPostingSource(app.db, job)
			if err != nil || !found || sourceState.Invalid || sourceState.Source == nil {
				t.Fatalf("intake source persistence: found=%v state=%+v err=%v", found, sourceState, err)
			}
			source := sourceState.Source
			if source.Identity.Provider != provider.provider || source.Identity.Board != provider.board || source.Identity.Region != provider.region || source.Identity.PostingID != provider.postingID {
				t.Fatalf("posting identity = %+v", source.Identity)
			}
			if source.Location != provider.location || source.WorkplaceType != provider.workplace || source.EmploymentType != provider.employment {
				t.Fatalf("direct source facts = location %q workplace %q employment %q", source.Location, source.WorkplaceType, source.EmploymentType)
			}

			path := "/jobs/" + fmt.Sprint(jobID)
			view := m3Request(router, http.MethodGet, path, nil)
			if view.Code != http.StatusOK || !strings.Contains(view.Body.String(), "Employer-provided description") ||
				!strings.Contains(view.Body.String(), "latest successful retrieval") || !strings.Contains(view.Body.String(), html.EscapeString(job.Description)) {
				t.Fatalf("Overview did not render source reading, freshness, and canonical copy text: status=%d", view.Code)
			}
			for _, want := range []string{provider.location, provider.refreshedContent} {
				if want == provider.refreshedContent { // shown after refresh below
					continue
				}
				if !strings.Contains(view.Body.String(), want) {
					t.Errorf("Overview omitted direct fact %q", want)
				}
			}

			manualDescription := "Applicant-owned correction for " + provider.name + "."
			manualDescriptions[provider.name] = manualDescription
			edit := url.Values{
				"title": {job.Title}, "company": {job.Company}, "description": {manualDescription},
				"salary_text": {"Applicant verified compensation"}, "stage": {job.Stage}, "priority": {job.Priority},
			}
			if edited := m3Request(router, http.MethodPost, path+"/edit", edit); edited.Code != http.StatusSeeOther {
				t.Fatalf("manual edit response=%d body=%s", edited.Code, edited.Body.String())
			}
			waitForPostingFreshnessAdvance(t, sourceState.LatestSuccessAt)
			if refreshed := m3Request(router, http.MethodPost, path+"/refresh", nil); refreshed.Code != http.StatusSeeOther {
				t.Fatalf("provider refresh response=%d body=%s", refreshed.Code, refreshed.Body.String())
			}
			job, err = app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			if !job.DescriptionManual || !job.SalaryManual || job.Description != manualDescription || job.SalaryText != "Applicant verified compensation" {
				t.Fatalf("manual corrections lost after refresh: %+v", job)
			}
			sourceState, found, err = loadJobPostingSource(app.db, job)
			if err != nil || !found || sourceState.Invalid || sourceState.Source == nil ||
				sourceState.Source.Identity.Provider != provider.provider || !strings.Contains(sourceState.Source.DescriptionHTML+sourceState.Source.DescriptionPlain, provider.refreshedContent) {
				t.Fatalf("refreshed source or content missing: found=%v state=%+v err=%v", found, sourceState, err)
			}
			if job.SourceStatus != "retrieved" || sourceState.LatestSuccessAt != job.SourceCheckedAt {
				t.Fatalf("refresh lost freshness: job=%+v state=%+v", job, sourceState)
			}
			view = m3Request(router, http.MethodGet, path, nil)
			if view.Code != http.StatusOK || !strings.Contains(view.Body.String(), manualDescription) ||
				!strings.Contains(view.Body.String(), "Applicant verified compensation") ||
				!strings.Contains(view.Body.String(), html.EscapeString(provider.refreshedContent)) || !strings.Contains(view.Body.String(), "latest successful retrieval") {
				t.Fatalf("Overview did not preserve manual priority alongside refreshed evidence: status=%d manual=%v salary=%v source=%v freshness=%v body=%s", view.Code,
					strings.Contains(view.Body.String(), manualDescription), strings.Contains(view.Body.String(), "Applicant verified compensation"),
					strings.Contains(view.Body.String(), provider.refreshedContent), strings.Contains(view.Body.String(), "latest successful retrieval"), view.Body.String())
			}
		})
	}

	if err := app.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(workspaceDir, "jobs.db"))
	if err != nil {
		t.Fatalf("reopen workspace DB: %v", err)
	}
	t.Cleanup(func() { _ = reopenedDB.Close() })
	restarted := newTestApp(t, reopenedDB, filepath.Join(workspaceDir, "resumes"))
	for _, provider := range providers {
		jobID := jobIDs[provider.name]
		job, err := restarted.jobByID(jobID)
		if err != nil {
			t.Fatalf("restart load %s: %v", provider.name, err)
		}
		state, found, err := loadJobPostingSource(restarted.db, job)
		if err != nil || !found || state.Invalid || state.Source == nil || state.Source.Identity.Provider != provider.provider || state.Source.Identity.Region != provider.region ||
			job.Description != manualDescriptions[provider.name] || !job.DescriptionManual || job.SalaryText != "Applicant verified compensation" ||
			!job.SalaryManual || job.SourceStatus != "retrieved" || state.LatestSuccessAt != job.SourceCheckedAt ||
			!strings.Contains(state.Source.DescriptionHTML+state.Source.DescriptionPlain, provider.refreshedContent) {
			t.Fatalf("restart persistence failed for %s: job=%+v source=%+v found=%v err=%v", provider.name, job, state, found, err)
		}
		view := m3Request(newMux(restarted), http.MethodGet, "/jobs/"+fmt.Sprint(jobID), nil)
		if view.Code != http.StatusOK || !strings.Contains(view.Body.String(), manualDescriptions[provider.name]) ||
			!strings.Contains(view.Body.String(), html.EscapeString(provider.refreshedContent)) || !strings.Contains(view.Body.String(), "latest successful retrieval") {
			t.Fatalf("restart Overview lost manual source ordering for %s: status=%d", provider.name, view.Code)
		}
	}
	if got := len(transport.snapshot()); got != 8 {
		t.Fatalf("provider requests = %d, want intake+refresh for four providers", got)
	}
}
