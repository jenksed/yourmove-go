//go:build qualification

package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestJobIntelligenceCompetitiveProviderOverviewDemo(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_COMPETITIVE") != "1" {
		t.Skip("opt-in synthetic competitive ATS Overview demonstration")
	}
	evidenceDir := os.Getenv("YOURMOVE_COMPETITIVE_EVIDENCE_DIR")
	if evidenceDir == "" || !filepath.IsAbs(evidenceDir) {
		t.Fatal("YOURMOVE_COMPETITIVE_EVIDENCE_DIR must be an absolute directory")
	}
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	readFixture := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	replaceFixture := func(data []byte, before, after string) []byte {
		t.Helper()
		updated := bytes.Replace(data, []byte(before), []byte(after), 1)
		if bytes.Equal(updated, data) {
			t.Fatalf("refresh marker source text %q was not present in synthetic fixture", before)
		}
		return updated
	}
	ashbyFixture := readFixture("ashby-long-synthetic.json")
	greenhouseFixture := readFixture("greenhouse-encoded-posting-synthetic.json")
	leverFixture := readFixture("lever-ordered-synthetic.json")
	workdayFixture := readFixture("workday-detail-synthetic.json")
	smartRecruitersFixture := readFixture("smartrecruiters-posting-synthetic.json")
	workableFixture := readFixture("workable-widget-synthetic.json")
	jobviteFixture := readFixture("jobvite-posting-synthetic.html")
	const (
		ashbyID           = "11111111-1111-4111-8111-111111111111"
		greenhouseID      = "1234567"
		leverID           = "11111111-2222-4333-8444-555555555555"
		workdayID         = "Platform-Engineer_R-12345"
		smartRecruitersID = "744000123456789"
		workableID        = "9AB12CD345"
		jobviteID         = "jobvite001"
	)
	type overviewProvider struct {
		name, postingURL, provider, board, region, postingID string
		title, description, refreshMarker                    string
		wantText                                             []string
	}
	providers := []overviewProvider{
		{
			name: "ashby", postingURL: "https://jobs.ashbyhq.com/fable/" + ashbyID,
			provider: "ashby", board: "fable", postingID: ashbyID, title: "Senior Platform Engineer",
			refreshMarker: "Updated employer source: Fable builds dependable tools.",
			wantText:      []string{"What you will do", "What you will bring", "Location and workplace", "Interview process", "European Union", "Remote"},
		},
		{
			name: "greenhouse", postingURL: "https://job-boards.greenhouse.io/acme/jobs/" + greenhouseID + "?gh_src=email",
			provider: "greenhouse", board: "acme", postingID: greenhouseID, title: "Support Engineer", description: "Debug API issues\n• Read logs",
			refreshMarker: "Refreshed debug API issues",
			wantText:      []string{"Debug API issues", "Read logs", "Remote - United States"},
		},
		{
			name: "lever-eu", postingURL: "https://jobs.eu.lever.co/lever/" + leverID,
			provider: "lever", board: "lever", region: "eu", postingID: leverID, title: "Senior Platform Engineer",
			refreshMarker: "Refreshed build reliable tools",
			wantText:      []string{"Responsibilities", "Design dependable service boundaries.", "Qualifications", "Production Go experience.", "Application instructions", "Toronto, Ontario", "Hybrid", "Full time", "CAD 150000–190000 / year"},
		},
		{
			name: "workday", postingURL: "https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle-WA/" + workdayID,
			provider: "workday", board: "acme.wd5.myworkdayjobs.com/acme/External", region: "en-US", postingID: "Seattle-WA/" + workdayID,
			title: "Platform Engineer", refreshMarker: "Build refreshed safe systems.",
			description: "Role overview\nBuild safe systems.\nWhat you will do\n• Design APIs.\n• Review code.\nQualifications\nExperience shipping Go services.",
			wantText:    []string{"Build safe systems.", "Qualifications", "Experience shipping Go services.", "Seattle, WA; Portland, OR", "Hybrid", "Full time"},
		},
		{
			name: "smartrecruiters", postingURL: "https://jobs.smartrecruiters.com/sampleco/" + smartRecruitersID,
			provider: "smartrecruiters", board: "sampleco", postingID: smartRecruitersID,
			title: "Senior API Engineer", refreshMarker: "Own the API after refresh.",
			description: "Company Description\nWe build tools & services.\nJob Description\nOwn the API.\n• Ship safely\n• Keep latency low\nQualifications\nThree years of Go.\nAdditional Information\nRemote within Canada.",
			wantText:    []string{"Company Description", "Own the API.", "Qualifications", "Additional Information", "Montreal, QC, Canada", "Remote", "Full time"},
		},
		{
			name: "workable", postingURL: "https://apply.workable.com/northstar/j/" + workableID,
			provider: "workable", board: "northstar", postingID: workableID,
			title: "Senior Site Reliability Engineer", refreshMarker: "Help keep Northstar's platform verifiably reliable.",
			description: "Help keep Northstar's platform reliable and secure.\nResponsibilities\n• Improve service health checks.\n• Review incident patterns.\nQualifications\nExperience operating production services.\nBenefits\nFlexible leave and learning time.",
			wantText:    []string{"Responsibilities", "Improve service health checks.", "Qualifications", "Benefits", "Detroit, Michigan, United States", "Remote", "Full time"},
		},
		{
			name: "jobvite", postingURL: "https://jobs.jobvite.com/syntheticco/job/" + jobviteID + "?source=fixture",
			provider: "jobvite", board: "syntheticco", postingID: jobviteID, title: "Synthetic Engineering Lead",
			refreshMarker: "Build the refreshed example platform with care.",
			description:   "Role summary\nBuild the example platform with care.\nResponsibilities\n• Design reliable services.\n• Review changes.\nQualifications\nExperience with Go.",
			wantText:      []string{"Role summary", "Design reliable services.", "Qualifications", "Denver, Colorado, United States", "Remote", "Full time"},
		},
	}
	// New public-feed/classic variants reuse the same controlled Overview gate.
	personioFixture := readFixture("../public-feeds/personio.xml")
	teamtailorFixture := readFixture("../public-feeds/teamtailor.xml")
	recruiteeFixture := readFixture("../public-feeds/recruitee.xml")
	icimsFixture := readFixture("../public-feeds/icims.html")
	providers = append(providers,
		overviewProvider{"personio", "https://example.jobs.personio.de/job/12345?language=en", "personio", "example.jobs.personio.de", "en", "12345", "Platform Engineer", "", "Build verifiable applicant-controlled systems.", []string{"Your responsibilities", "Build reliable applicant-controlled systems."}},
		overviewProvider{"teamtailor", "https://example.teamtailor.com/jobs/12345-platform-engineer", "teamtailor", "example.teamtailor.com", "", "12345", "Platform Engineer", "", "Build verifiable services.", []string{"Responsibilities", "Build safe services."}},
		overviewProvider{"recruitee", "https://example.recruitee.com/o/platform-engineer", "recruitee", "example.recruitee.com", "", "platform-engineer", "Platform Engineer", "", "Build verifiable services.", []string{"Responsibilities", "Build safe services."}},
		overviewProvider{"icims", "https://example.icims.com/jobs/12345/platform-engineer/job", "icims", "example.icims.com", "", "12345", "Platform Engineer", "", "Build verifiable services.", []string{"Responsibilities", "Build safe services."}},
	)
	previousTransport := http.DefaultTransport
	transport := &m3SourceTransport{fallback: previousTransport, responses: map[string][][]byte{
		"api.ashbyhq.com/posting-api/job-board/fable": {
			ashbyFixture,
			replaceFixture(ashbyFixture, "Fable builds dependable tools for teams that coordinate complex work.", "Updated employer source: Fable builds dependable tools."),
		},
		"boards-api.greenhouse.io/v1/boards/acme/jobs/" + greenhouseID: {
			greenhouseFixture,
			replaceFixture(greenhouseFixture, "Debug API issues", "Refreshed debug API issues"),
		},
		"api.eu.lever.co/v0/postings/lever/" + leverID: {
			leverFixture,
			replaceFixture(leverFixture, "Build reliable tools for teams that operate important services.", "Refreshed build reliable tools for teams that operate important services."),
		},
		"acme.wd5.myworkdayjobs.com/wday/cxs/acme/External/job/Seattle-WA/" + workdayID: {
			workdayFixture,
			replaceFixture(workdayFixture, "Build safe systems.", "Build refreshed safe systems."),
		},
		"api.smartrecruiters.com/v1/companies/sampleco/postings/" + smartRecruitersID: {
			smartRecruitersFixture,
			replaceFixture(smartRecruitersFixture, "Own the API.", "Own the API after refresh."),
		},
		"apply.workable.com/api/v1/widget/accounts/northstar": {
			workableFixture,
			replaceFixture(workableFixture, "Help keep Northstar's platform reliable and secure.", "Help keep Northstar's platform verifiably reliable."),
		},
		"jobs.jobvite.com/syntheticco/job/" + jobviteID: {
			jobviteFixture,
			replaceFixture(jobviteFixture, "Build the example platform with care.", "Build the refreshed example platform with care."),
		},
	}}
	transport.responses["example.jobs.personio.de/xml"] = [][]byte{personioFixture, replaceFixture(personioFixture, "Build reliable applicant-controlled systems.", "Build verifiable applicant-controlled systems.")}
	transport.responses["example.teamtailor.com/jobs.rss"] = [][]byte{teamtailorFixture, replaceFixture(teamtailorFixture, "Build safe services.", "Build verifiable services.")}
	transport.responses["example.recruitee.com/api/feeds/offers.xml"] = [][]byte{recruiteeFixture, replaceFixture(recruiteeFixture, "Build safe services.", "Build verifiable services.")}
	transport.responses["example.icims.com/jobs/12345/job"] = [][]byte{icimsFixture, replaceFixture(icimsFixture, "Build safe services.", "Build verifiable services.")}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	app, workspaceDir := testWorkspace(t)
	app.enrichmentFetch = fetchPostingDetails
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on IPv4 loopback for controlled Chromium: %v", err)
	}
	appHandler := newMux(app)
	router := http.NewServeMux()
	router.HandleFunc("GET /jobs/{id}/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/"+r.PathValue("id")+"#overview", http.StatusSeeOther)
	})
	router.Handle("/", appHandler)
	server := httptest.NewUnstartedServer(router)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	jobIDs := make(map[string]int64, len(providers))
	manualDescriptions := make(map[string]string, len(providers))

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			response, err := client.PostForm(server.URL+"/jobs", url.Values{"job_url": {provider.postingURL}})
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/jobs" {
				t.Fatalf("production intake = %d Location=%q", response.StatusCode, response.Header.Get("Location"))
			}
			var jobID int64
			if err := app.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, provider.postingURL).Scan(&jobID); err != nil {
				t.Fatalf("load intaken %s posting: %v", provider.name, err)
			}
			jobIDs[provider.name] = jobID
			job, err := app.jobByID(jobID)
			if err != nil || job.Title != provider.title || job.Description == "" ||
				(provider.description != "" && job.Description != provider.description) {
				t.Fatalf("canonical intake mismatch: title=%q description=%q want title=%q description=%q err=%v", job.Title, job.Description, provider.title, provider.description, err)
			}
			sourceState, found, err := loadJobPostingSource(app.db, job)
			if err != nil || !found || sourceState.Invalid || sourceState.Source == nil {
				t.Fatalf("posting source was not retained for %s: found=%v state=%+v err=%v", provider.name, found, sourceState, err)
			}
			identity := sourceState.Source.Identity
			if identity.Provider != provider.provider || identity.Board != provider.board || identity.Region != provider.region || identity.PostingID != provider.postingID {
				t.Fatalf("%s posting identity = %+v", provider.name, identity)
			}

			pageURL := server.URL + "/jobs/" + strconv.FormatInt(jobID, 10)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			chrome, err := chromium.LaunchFixture(ctx, pageURL)
			if err != nil {
				t.Fatal(err)
			}
			defer chrome.Close()
			waitM1Session(t, ctx, chrome, pageURL, `document.readyState==='complete' && !!document.querySelector('[data-job-document]') && location.hash==='#overview'`)
			page, err := chrome.FixturePage(ctx, pageURL)
			if err != nil {
				t.Fatal(err)
			}
			defer page.Close()
			if err := page.Call(ctx, "Runtime.enable", map[string]any{}, nil); err != nil {
				t.Fatal(err)
			}
			waitM1Browser(t, ctx, page, `document.readyState==='complete'`)
			visibleText := evalM1Browser[string](t, ctx, page, `document.body?.innerText||''`)
			for _, token := range append(provider.wantText, "Employer-provided description", "latest successful retrieval") {
				if !strings.Contains(visibleText, token) {
					t.Errorf("%s Overview is missing %q", provider.name, token)
				}
			}
			if !evalM1Browser[bool](t, ctx, page, `(()=>{const select=document.querySelector('[data-theme-choice]');if(!select)return false;select.value='light';select.dispatchEvent(new Event('change'));document.activeElement?.blur();return document.documentElement.dataset.theme==='light'})()`) {
				t.Fatal("could not set a reproducible light theme before capturing Overview")
			}
			for _, width := range []int{1280, 390} {
				if err := page.Call(ctx, "Runtime.evaluate", map[string]any{"expression": "window.scrollTo(0,0)"}, nil); err != nil {
					t.Fatal(err)
				}
				if !evalM1Browser[bool](t, ctx, page, `(()=>{document.activeElement?.blur();return document.activeElement?.tagName==='BODY'||document.activeElement===document.body})()`) {
					t.Fatal("could not clear keyboard focus before screenshot")
				}
				if err := page.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": 900, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
					t.Fatalf("set %dpx viewport: %v", width, err)
				}
				layout := evalM1Browser[map[string]float64](t, ctx, page, `(()=>({innerWidth,scrollWidth:document.documentElement.scrollWidth}))()`)
				if layout["scrollWidth"] > layout["innerWidth"] {
					t.Fatalf("%s Overview overflows %dpx viewport: %+v", provider.name, width, layout)
				}
				var capture struct {
					Data string `json:"data"`
				}
				if err := page.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &capture); err != nil {
					t.Fatalf("capture %s Overview at %dpx: %v", provider.name, width, err)
				}
				png, err := base64.StdEncoding.DecodeString(capture.Data)
				if err != nil {
					t.Fatalf("decode %s screenshot: %v", provider.name, err)
				}
				path := filepath.Join(evidenceDir, "competitive-"+provider.name+"-overview-"+strconv.Itoa(width)+".png")
				if err := os.WriteFile(path, png, 0600); err != nil {
					t.Fatalf("write screenshot: %v", err)
				}
				t.Logf("captured actual %s Overview at %dpx: %s", provider.name, width, path)
			}
			if !evalM1Browser[bool](t, ctx, page, `!!document.querySelector('[data-copy-description]:not([hidden])')`) {
				t.Fatal("canonical copy control is unavailable")
			}
			if !evalM1Browser[bool](t, ctx, page, `(()=>{const button=document.querySelector('[data-copy-description]');Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.__competitiveCopied=text}}});button.click();return true})()`) {
				t.Fatal("could not invoke canonical copy control")
			}
			waitM1Browser(t, ctx, page, `window.__competitiveCopied!==undefined && document.querySelector('[data-copy-description-status]')?.textContent==='Job description copied.'`)
			if copied := evalM1Browser[string](t, ctx, page, "window.__competitiveCopied"); copied != strings.TrimSpace(job.Description) {
				t.Fatalf("canonical copy changed: got %q want %q", copied, strings.TrimSpace(job.Description))
			}

			manualDescription := "Applicant-owned synthetic edit retained for " + provider.name + "."
			manualDescriptions[provider.name] = manualDescription
			editExpression := `(()=>{const details=document.querySelector('#edit-details');const form=document.querySelector('.job-edit-form');const field=document.querySelector('#description');if(!details||!form||!field)return false;details.open=true;field.value=` + strconv.Quote(manualDescription) + `;form.requestSubmit();return true})()`
			if !evalM1Browser[bool](t, ctx, page, editExpression) {
				t.Fatalf("%s could not submit the existing Edit details form", provider.name)
			}
			waitManual := `document.readyState==='complete'&&document.querySelector('#description')?.value===` + strconv.Quote(manualDescription)
			waitM1Browser(t, ctx, page, waitManual)
			if got := evalM1Browser[string](t, ctx, page, `document.querySelector('[data-job-document]')?.innerText.trim()||''`); got != manualDescription {
				t.Fatalf("%s saved manual edit is not the primary reading: %q", provider.name, got)
			}
			waitForPostingFreshnessAdvance(t, sourceState.LatestSuccessAt)
			if !evalM1Browser[bool](t, ctx, page, `(()=>{const form=document.querySelector('form[action$="/refresh"]');if(!form)return false;form.requestSubmit();return true})()`) {
				t.Fatalf("%s could not submit the existing Refresh from posting form", provider.name)
			}
			waitM1Browser(t, ctx, page, waitManual)
			refreshedView := evalM1Browser[map[string]string](t, ctx, page, `(()=>({primary:document.querySelector('[data-job-document]')?.innerText.trim()||'',employer:document.querySelector('.employer-description-details .job-document')?.innerText||'',freshness:document.querySelector('#source-title')?.parentElement?.innerText||''}))()`)
			if refreshedView["primary"] != manualDescription || !strings.Contains(refreshedView["employer"], provider.refreshMarker) {
				t.Fatalf("%s refresh did not preserve the manual primary and update employer evidence: %+v", provider.name, refreshedView)
			}
			if !strings.Contains(refreshedView["freshness"], "latest successful retrieval") {
				t.Fatalf("%s refresh freshness is not shown in the Overview: %q", provider.name, refreshedView["freshness"])
			}
			job, err = app.jobByID(jobID)
			if err != nil || !job.DescriptionManual || job.Description != manualDescription || job.SubmissionCount != 0 {
				t.Fatalf("%s refresh lost applicant edits or changed submission state: job=%+v err=%v", provider.name, job, err)
			}
			sourceState, found, err = loadJobPostingSource(app.db, job)
			if err != nil || !found || sourceState.Invalid || sourceState.Source == nil ||
				sourceState.Source.Identity.Provider != provider.provider ||
				!strings.Contains(sourceState.Source.DescriptionHTML+sourceState.Source.DescriptionPlain, provider.refreshMarker) ||
				sourceState.LatestSuccessAt != job.SourceCheckedAt || job.SourceStatus != "retrieved" {
				t.Fatalf("%s refresh source/freshness was not retained: job=%+v source=%+v found=%v err=%v", provider.name, job, sourceState, found, err)
			}
			if !evalM1Browser[bool](t, ctx, page, `(()=>{const button=document.querySelector('[data-copy-description]:not([hidden])');if(!button)return false;Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.__competitiveManualCopied=text}}});button.click();return true})()`) {
				t.Fatalf("%s canonical copy control became unavailable after manual edit", provider.name)
			}
			waitM1Browser(t, ctx, page, `window.__competitiveManualCopied!==undefined`)
			if copied := evalM1Browser[string](t, ctx, page, "window.__competitiveManualCopied"); copied != manualDescription {
				t.Fatalf("%s canonical copy did not preserve the manual text: got %q want %q", provider.name, copied, manualDescription)
			}
			if got := evalM1Browser[string](t, ctx, page, `location.host`); !strings.HasPrefix(got, "127.0.0.1:") {
				t.Fatalf("browser left the controlled local app: %s", got)
			}
			job, err = app.jobByID(jobID)
			if err != nil || job.SubmissionCount != 0 {
				t.Fatalf("qualification changed application submission state: job=%+v err=%v", job, err)
			}
			t.Logf("%s controlled lifecycle PASS: manual edit, refreshed source, freshness and canonical copy preserved; no employer application opened", provider.name)
		})
	}
	if requests := transport.snapshot(); len(requests) != 2*len(providers) {
		t.Fatalf("provider source transport saw %d requests, want intake+refresh for %d providers: %v", len(requests), len(providers), requests)
	}

	server.Close()
	if err := app.db.Close(); err != nil {
		t.Fatalf("close qualification workspace before restart: %v", err)
	}
	reopenedDB, err := storage.OpenDB(filepath.Join(workspaceDir, "jobs.db"))
	if err != nil {
		t.Fatalf("reopen qualification workspace: %v", err)
	}
	restartedApp := newTestApp(t, reopenedDB, filepath.Join(workspaceDir, "resumes"))
	listener, err = net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on IPv4 loopback after SQLite restart: %v", err)
	}
	restartedRouter := http.NewServeMux()
	restartedRouter.HandleFunc("GET /jobs/{id}/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/"+r.PathValue("id")+"#overview", http.StatusSeeOther)
	})
	restartedRouter.Handle("/", newMux(restartedApp))
	restartedServer := httptest.NewUnstartedServer(restartedRouter)
	restartedServer.Listener = listener
	restartedServer.Start()
	t.Cleanup(restartedServer.Close)
	for _, provider := range providers {
		jobID := jobIDs[provider.name]
		job, err := restartedApp.jobByID(jobID)
		if err != nil {
			t.Fatalf("restart load %s: %v", provider.name, err)
		}
		state, found, err := loadJobPostingSource(restartedApp.db, job)
		if err != nil || !found || state.Invalid || state.Source == nil ||
			state.Source.Identity.Provider != provider.provider || !strings.Contains(state.Source.DescriptionHTML+state.Source.DescriptionPlain, provider.refreshMarker) ||
			job.Description != manualDescriptions[provider.name] || !job.DescriptionManual || job.SourceStatus != "retrieved" || state.LatestSuccessAt != job.SourceCheckedAt || job.SubmissionCount != 0 {
			t.Fatalf("restart persistence failed for %s: job=%+v source=%+v found=%v err=%v", provider.name, job, state, found, err)
		}
		pageURL := restartedServer.URL + "/jobs/" + strconv.FormatInt(jobID, 10)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		chrome, err := chromium.LaunchFixture(ctx, pageURL)
		if err != nil {
			cancel()
			t.Fatalf("launch restored %s Overview: %v", provider.name, err)
		}
		waitM1Session(t, ctx, chrome, pageURL, `document.readyState==='complete'&&!!document.querySelector('[data-job-document]')&&location.hash==='#overview'`)
		page, err := chrome.FixturePage(ctx, pageURL)
		if err != nil {
			_ = chrome.Close()
			cancel()
			t.Fatalf("open restored %s Overview: %v", provider.name, err)
		}
		restored := evalM1Browser[map[string]string](t, ctx, page, `(()=>{const employerDetails=document.querySelector('.employer-description-details');if(employerDetails)employerDetails.open=true;return {primary:document.querySelector('[data-job-document]')?.innerText.trim()||'',employer:document.querySelector('.employer-description-details .job-document')?.innerText||'',freshness:document.querySelector('#source-title')?.parentElement?.innerText||'',canonical:document.querySelector('#canonical-description-copy')?.value||''}})()`)
		if restored["primary"] != manualDescriptions[provider.name] || !strings.Contains(restored["employer"], provider.refreshMarker) ||
			!strings.Contains(restored["freshness"], "latest successful retrieval") || restored["canonical"] != manualDescriptions[provider.name] {
			_ = page.Close()
			_ = chrome.Close()
			cancel()
			t.Fatalf("restored %s Overview lost manual/source/freshness/canonical state: %+v", provider.name, restored)
		}
		if got := evalM1Browser[string](t, ctx, page, `location.host`); !strings.HasPrefix(got, "127.0.0.1:") {
			_ = page.Close()
			_ = chrome.Close()
			cancel()
			t.Fatalf("restored %s browser left the local app: %s", provider.name, got)
		}
		_ = page.Close()
		_ = chrome.Close()
		cancel()
		t.Logf("%s SQLite restart PASS: manual description, employer source, freshness and canonical copy persist", provider.name)
	}
}
