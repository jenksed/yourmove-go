package app

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestJobIntelligenceM3ProvidersUseIntakeOverviewAndPersistAcrossRestart(t *testing.T) {
	greenhouseFixture, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", "greenhouse-encoded-posting-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	leverFixture, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", "lever-ordered-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	greenhouseRefreshFixture := bytes.ReplaceAll(greenhouseFixture, []byte("Debug API issues"), []byte("Updated employer source content"))
	leverRefreshFixture := bytes.ReplaceAll(leverFixture, []byte("Design dependable service boundaries."), []byte("Design verifiable service boundaries."))
	leverRefreshFixture = bytes.ReplaceAll(leverRefreshFixture, []byte(`"min": 150000`), []byte(`"min": 160000`))
	leverRefreshFixture = bytes.ReplaceAll(leverRefreshFixture, []byte(`"max": 190000`), []byte(`"max": 200000`))

	const leverID = "11111111-2222-4333-8444-555555555555"
	greenhouseURL := "https://job-boards.greenhouse.io/acme/jobs/1234567?gh_src=email"
	leverURL := "https://jobs.eu.lever.co/lever/" + leverID
	previousTransport := http.DefaultTransport
	transport := &m3SourceTransport{fallback: previousTransport, responses: map[string][][]byte{
		"boards-api.greenhouse.io/v1/boards/acme/jobs/1234567": {greenhouseFixture, greenhouseRefreshFixture},
		"api.eu.lever.co/v0/postings/lever/" + leverID:         {leverFixture, leverRefreshFixture},
	}}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	app, workspaceDir := testWorkspace(t)
	rootTest := t

	providers := []struct {
		name, postingURL, wantTitle, wantCompany, wantSalary, wantLocation, wantWorkplace, wantEmployment string
		wantCanonical, wantIdentityProvider, wantIdentityRegion, wantIdentityBoard, wantIdentityID        string
		wantRefreshedContent, wantRefreshedSalary                                                         string
		wantRefreshedLocation, wantRefreshedWorkplace, wantRefreshedEmployment                            string
		wantOverview                                                                                      []string
	}{
		{
			name: "Greenhouse", postingURL: greenhouseURL, wantTitle: "Support Engineer", wantCompany: "acme",
			wantLocation: "Remote - United States", wantCanonical: "Debug API issues\n• Read logs",
			wantRefreshedContent:  "Updated employer source content",
			wantRefreshedLocation: "Remote - United States",
			wantIdentityProvider:  "greenhouse", wantIdentityBoard: "acme", wantIdentityID: "1234567",
			wantOverview: []string{"Debug API issues", "Read logs", "Remote - United States", "latest successful retrieval"},
		},
		{
			name: "Lever EU", postingURL: leverURL, wantTitle: "Senior Platform Engineer", wantCompany: "lever",
			wantSalary: "CAD 150000–190000 / year", wantLocation: "Toronto, Ontario", wantWorkplace: "hybrid", wantEmployment: "Full-time",
			wantCanonical:        "Opening\nBuild reliable tools for teams that operate important services.\n\nRole overview\nPartner with product and operations to improve the service platform.\n\nResponsibilities\n• Design dependable service boundaries.\n• Improve observability and incident recovery.\n\nQualifications\n• Production Go experience.\n• Clear technical communication.\n\nApplication instructions\nInterview steps will be explained before they begin.",
			wantRefreshedContent: "Design verifiable service boundaries.", wantRefreshedSalary: "CAD 160000–200000 / year",
			wantRefreshedLocation: "Toronto, Ontario", wantRefreshedWorkplace: "hybrid", wantRefreshedEmployment: "Full-time",
			wantIdentityProvider: "lever", wantIdentityRegion: "eu", wantIdentityBoard: "lever", wantIdentityID: leverID,
			wantOverview: []string{"Responsibilities", "Design dependable service boundaries.", "Qualifications", "Production Go experience.", "Application instructions", "Toronto, Ontario", "Hybrid", "Full time", "CAD 150000–190000 / year", "latest successful retrieval"},
		},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			app.enrichmentFetch = fetchPostingDetails
			router := newMux(app)
			response := m3Request(router, http.MethodPost, "/jobs", url.Values{"job_url": {provider.postingURL}})
			if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/jobs" {
				t.Fatalf("production intake = %d Location=%q", response.Code, response.Header().Get("Location"))
			}

			var jobID int64
			if err := app.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, provider.postingURL).Scan(&jobID); err != nil {
				t.Fatalf("load intaken posting: %v", err)
			}
			job, err := app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			if job.Title != provider.wantTitle || job.Company != provider.wantCompany || job.Description != provider.wantCanonical || job.SalaryText != provider.wantSalary {
				t.Fatalf("canonical intake projection changed: %+v", job)
			}
			sourceState, found, err := loadJobPostingSource(app.db, job)
			if err != nil || !found || sourceState.Invalid || sourceState.Source == nil {
				t.Fatalf("intake source persistence: found=%v state=%+v err=%v", found, sourceState, err)
			}
			source := sourceState.Source
			if source.Identity.Provider != provider.wantIdentityProvider || source.Identity.Region != provider.wantIdentityRegion ||
				source.Identity.Board != provider.wantIdentityBoard || source.Identity.PostingID != provider.wantIdentityID {
				t.Fatalf("saved source identity = %+v", source.Identity)
			}
			if source.Location != provider.wantLocation || source.WorkplaceType != provider.wantWorkplace || source.EmploymentType != provider.wantEmployment || source.SalarySummary != provider.wantSalary {
				t.Fatalf("saved source direct facts = %+v", source)
			}

			path := "/jobs/" + fmt.Sprint(jobID)
			viewResponse := m3Request(router, http.MethodGet, path, nil)
			viewBody := viewResponse.Body.String()
			if viewResponse.Code != http.StatusOK {
				t.Fatalf("Overview response: status=%d", viewResponse.Code)
			}
			for _, want := range provider.wantOverview {
				if !strings.Contains(viewBody, want) {
					t.Errorf("actual Overview does not show %q", want)
				}
			}
			if !strings.Contains(viewBody, `data-copy-description`) || !strings.Contains(viewBody, htmlEscape(job.Description)) {
				t.Fatal("Overview is missing the canonical description/copy control")
			}

			manualDescription := "Applicant-owned synthetic correction for " + provider.name + "."
			edit := url.Values{
				"title": {job.Title}, "company": {job.Company}, "description": {manualDescription},
				"salary_text": {"Applicant verified compensation"}, "stage": {job.Stage}, "priority": {job.Priority},
			}
			editResponse := m3Request(router, http.MethodPost, path+"/edit", edit)
			if editResponse.Code != http.StatusSeeOther {
				t.Fatalf("manual edit response=%d body=%s", editResponse.Code, editResponse.Body.String())
			}
			previousSuccessAt := sourceState.LatestSuccessAt
			waitForPostingFreshnessAdvance(t, previousSuccessAt)
			refreshResponse := m3Request(router, http.MethodPost, path+"/refresh", nil)
			if refreshResponse.Code != http.StatusSeeOther {
				t.Fatalf("provider refresh response=%d body=%s", refreshResponse.Code, refreshResponse.Body.String())
			}
			job, err = app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			if !job.DescriptionManual || !job.SalaryManual || job.Description != manualDescription || job.SalaryText != "Applicant verified compensation" {
				t.Fatalf("manual corrections were not authoritative after provider refresh: %+v", job)
			}
			sourceState, found, err = loadJobPostingSource(app.db, job)
			if err != nil || !found || sourceState.Invalid || sourceState.Source == nil || sourceState.Source.Identity.Provider != provider.wantIdentityProvider {
				t.Fatalf("refreshed source was not retained beside manual corrections: found=%v state=%+v err=%v", found, sourceState, err)
			}
			if job.SourceStatus != "retrieved" || job.SourceCheckedAt == "" || sourceState.LatestSuccessAt != job.SourceCheckedAt ||
				!strings.Contains(sourceState.Source.DescriptionHTML+sourceState.Source.DescriptionPlain, provider.wantRefreshedContent) {
				t.Fatalf("provider refresh did not update source content and freshness: job=%+v source=%+v", job, sourceState)
			}
			previousTime, previousErr := time.Parse(time.RFC3339Nano, previousSuccessAt)
			refreshedTime, refreshedErr := time.Parse(time.RFC3339Nano, sourceState.LatestSuccessAt)
			if previousErr != nil || refreshedErr != nil || !refreshedTime.After(previousTime) {
				t.Fatalf("provider freshness did not advance: before=%q after=%q previousErr=%v refreshedErr=%v", previousSuccessAt, sourceState.LatestSuccessAt, previousErr, refreshedErr)
			}
			if sourceState.Source.Location != provider.wantRefreshedLocation || sourceState.Source.WorkplaceType != provider.wantRefreshedWorkplace ||
				sourceState.Source.EmploymentType != provider.wantRefreshedEmployment {
				t.Fatalf("refreshed employer direct facts = location %q workplace %q employment %q", sourceState.Source.Location, sourceState.Source.WorkplaceType, sourceState.Source.EmploymentType)
			}
			if provider.wantRefreshedSalary != "" && sourceState.Source.SalarySummary != provider.wantRefreshedSalary {
				t.Fatalf("refreshed employer salary = %q, want %q", sourceState.Source.SalarySummary, provider.wantRefreshedSalary)
			}
			viewResponse = m3Request(router, http.MethodGet, path, nil)
			viewBody = viewResponse.Body.String()
			if viewResponse.Code != http.StatusOK || !strings.Contains(viewBody, manualDescription) || !strings.Contains(viewBody, "Applicant verified compensation") ||
				!strings.Contains(viewBody, "Employer-provided version") || !strings.Contains(viewBody, provider.wantRefreshedContent) || !strings.Contains(viewBody, "latest successful retrieval") {
				t.Fatalf("Overview did not keep manual values primary beside refreshed source/freshness: status=%d", viewResponse.Code)
			}

			if err := app.db.Close(); err != nil {
				t.Fatal(err)
			}
			reopenedDB, err := storage.OpenDB(filepath.Join(workspaceDir, "jobs.db"))
			if err != nil {
				t.Fatalf("reopen workspace DB: %v", err)
			}
			rootTest.Cleanup(func() { reopenedDB.Close() })
			app = newTestApp(t, reopenedDB, filepath.Join(workspaceDir, "resumes"))
			restartedJob, err := app.jobByID(jobID)
			if err != nil {
				t.Fatalf("load job after restart: %v", err)
			}
			restartedSource, found, err := loadJobPostingSource(app.db, restartedJob)
			if err != nil || !found || restartedSource.Invalid || restartedSource.Source == nil ||
				restartedSource.Source.Identity.Provider != provider.wantIdentityProvider || restartedJob.Description != manualDescription ||
				!restartedJob.DescriptionManual || restartedJob.SalaryText != "Applicant verified compensation" || !restartedJob.SalaryManual ||
				restartedJob.SourceStatus != "retrieved" || restartedSource.LatestSuccessAt != restartedJob.SourceCheckedAt ||
				!strings.Contains(restartedSource.Source.DescriptionHTML+restartedSource.Source.DescriptionPlain, provider.wantRefreshedContent) ||
				restartedSource.Source.Location != provider.wantRefreshedLocation || restartedSource.Source.WorkplaceType != provider.wantRefreshedWorkplace ||
				restartedSource.Source.EmploymentType != provider.wantRefreshedEmployment {
				t.Fatalf("restart persistence failed: job=%+v source=%+v found=%v err=%v", restartedJob, restartedSource, found, err)
			}
			if provider.wantRefreshedSalary != "" && restartedSource.Source.SalarySummary != provider.wantRefreshedSalary {
				t.Fatalf("restart lost refreshed employer salary: %+v", restartedSource.Source)
			}
			restartedView := m3Request(newMux(app), http.MethodGet, path, nil)
			restartedBody := restartedView.Body.String()
			if restartedView.Code != http.StatusOK || !strings.Contains(restartedBody, manualDescription) || !strings.Contains(restartedBody, "Applicant verified compensation") ||
				!strings.Contains(restartedBody, "Employer-provided version") || !strings.Contains(restartedBody, provider.wantRefreshedContent) ||
				!strings.Contains(restartedBody, "latest successful retrieval") {
				t.Fatalf("restarted Overview lost manual values, refreshed source, or freshness: status=%d", restartedView.Code)
			}
		})
	}

	requests := transport.snapshot()
	if len(requests) != 4 {
		t.Fatalf("provider transport saw %d requests, want intake and refresh GETs for Greenhouse and Lever: %v", len(requests), requests)
	}
}

func waitForPostingFreshnessAdvance(t *testing.T, previous string) {
	t.Helper()
	previousTime, err := time.Parse(time.RFC3339Nano, previous)
	if err != nil {
		t.Fatalf("parse prior posting freshness %q: %v", previous, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if time.Now().UTC().Format(time.RFC3339) != previousTime.UTC().Format(time.RFC3339) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for posting freshness timestamp to advance beyond %q", previous)
}

type m3SourceTransport struct {
	mu        sync.Mutex
	fallback  http.RoundTripper
	responses map[string][][]byte
	next      map[string]int
	requests  []string
}

func (transport *m3SourceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	key := request.URL.Host + request.URL.EscapedPath()
	transport.mu.Lock()
	bodies, ok := transport.responses[key]
	var body []byte
	if ok {
		if transport.next == nil {
			transport.next = make(map[string]int, len(transport.responses))
		}
		index := transport.next[key]
		if index >= len(bodies) {
			transport.mu.Unlock()
			return nil, fmt.Errorf("unexpected extra provider request %s %s", request.Method, request.URL)
		}
		body = bodies[index]
		transport.next[key]++
		transport.requests = append(transport.requests, request.Method+" https://"+key)
	}
	transport.mu.Unlock()
	if ok && request.Method != http.MethodGet {
		return nil, fmt.Errorf("unexpected provider request %s %s", request.Method, request.URL)
	}
	if ok {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	}
	if host, _, err := net.SplitHostPort(request.URL.Host); err == nil {
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() && transport.fallback != nil {
			return transport.fallback.RoundTrip(request)
		}
	}
	return nil, fmt.Errorf("unexpected provider request %s %s", request.Method, request.URL)
}

func (transport *m3SourceTransport) snapshot() []string {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return append([]string(nil), transport.requests...)
}

func htmlEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&#34;", "'", "&#39;").Replace(value)
}

func m3Request(handler http.Handler, method, path string, form url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequest(method, "http://localhost"+path, body)
	request.RemoteAddr = "127.0.0.1:12345"
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
