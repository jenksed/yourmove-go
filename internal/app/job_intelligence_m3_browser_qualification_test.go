//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"fmt"
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
)

func TestJobIntelligenceM3ProviderOverviewDemo(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_M3_PROVIDERS") != "1" {
		t.Skip("opt-in synthetic Greenhouse and Lever Job Intelligence demonstration")
	}
	evidenceDir := os.Getenv("YOURMOVE_M3_EVIDENCE_DIR")
	if evidenceDir == "" || !filepath.IsAbs(evidenceDir) {
		t.Fatal("YOURMOVE_M3_EVIDENCE_DIR must be an absolute directory")
	}
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}

	greenhouseFixture, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", "greenhouse-encoded-posting-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	leverFixture, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", "lever-ordered-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	const leverID = "11111111-2222-4333-8444-555555555555"
	previousTransport := http.DefaultTransport
	transport := &m3SourceTransport{fallback: previousTransport, responses: map[string][][]byte{
		"boards-api.greenhouse.io/v1/boards/acme/jobs/1234567": {greenhouseFixture},
		"api.eu.lever.co/v0/postings/lever/" + leverID:         {leverFixture},
	}}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	providers := []struct {
		name, url, title, description string
		wantText                      []string
		sectionIDs                    []string
	}{
		{
			name: "greenhouse", url: "https://job-boards.greenhouse.io/acme/jobs/1234567?gh_src=email",
			title: "Support Engineer", description: "Debug API issues\n• Read logs",
			wantText: []string{"Debug API issues", "Read logs", "Remote - United States", "Employer-provided description"},
		},
		{
			name: "lever-eu", url: "https://jobs.eu.lever.co/lever/" + leverID,
			title:       "Senior Platform Engineer",
			description: "Opening\nBuild reliable tools for teams that operate important services.\n\nRole overview\nPartner with product and operations to improve the service platform.\n\nResponsibilities\n• Design dependable service boundaries.\n• Improve observability and incident recovery.\n\nQualifications\n• Production Go experience.\n• Clear technical communication.\n\nApplication instructions\nInterview steps will be explained before they begin.",
			wantText:    []string{"Responsibilities", "Design dependable service boundaries.", "Qualifications", "Production Go experience.", "Application instructions", "Toronto, Ontario", "Hybrid", "Full time", "CAD 150000–190000 / year"},
			sectionIDs:  []string{"job-section-responsibilities-1", "job-section-qualifications-1", "job-section-hiring-1"},
		},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			app, _ := testWorkspace(t)
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
			response, err := client.PostForm(server.URL+"/jobs", url.Values{"job_url": {provider.url}})
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/jobs" {
				t.Fatalf("production intake response = %d Location=%q", response.StatusCode, response.Header.Get("Location"))
			}
			var jobID int64
			if err := app.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, provider.url).Scan(&jobID); err != nil {
				t.Fatalf("load intaken %s fixture: %v", provider.name, err)
			}
			job, err := app.jobByID(jobID)
			if err != nil {
				t.Fatal(err)
			}
			if job.Title != provider.title || job.Description != provider.description {
				t.Fatalf("canonical posting changed: title=%q description=%q", job.Title, job.Description)
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
			if !evalM1Browser[bool](t, ctx, page, `(()=>{const panel=document.querySelector('[data-workspace-panel="overview"]');if(!panel||panel.hidden)return false;history.replaceState(null,'',location.pathname+location.search);document.activeElement?.blur();return true})()`) {
				t.Fatal("Overview activity did not become visible")
			}
			loaded := evalM1Browser[map[string]any](t, ctx, page, `(()=>({url:location.href,title:document.title,hasDocument:!!document.querySelector('[data-job-document]'),body:document.body?.innerText||''}))()`)
			if loaded["hasDocument"] != true || fmt.Sprint(loaded["url"]) != pageURL {
				t.Fatalf("actual provider Overview did not render: %+v", loaded)
			}

			visibleText := evalM1Browser[string](t, ctx, page, `document.body?.innerText||''`)
			for _, token := range provider.wantText {
				if !strings.Contains(visibleText, token) {
					t.Errorf("%s Overview is missing %q", provider.name, token)
				}
			}
			for _, width := range []int{1280, 390} {
				if err := page.Call(ctx, "Runtime.evaluate", map[string]any{"expression": "window.scrollTo(0,0)"}, nil); err != nil {
					t.Fatal(err)
				}
				if err := page.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": 900, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
					t.Fatalf("set %dpx viewport: %v", width, err)
				}
				layout := evalM1Browser[map[string]float64](t, ctx, page, `(()=>({innerWidth,scrollWidth:document.documentElement.scrollWidth}))()`)
				if layout["scrollWidth"] > layout["innerWidth"] {
					t.Fatalf("%s Overview overflows %dpx viewport: %+v", provider.name, width, layout)
				}
				for _, theme := range []string{"light", "dark"} {
					themeSet := evalM1Browser[bool](t, ctx, page, `(()=>{const select=document.querySelector('[data-theme-choice]');if(!select)return false;select.value=`+strconv.Quote(theme)+`;select.dispatchEvent(new Event('change'));return document.documentElement.dataset.theme===`+strconv.Quote(theme)+`})()`)
					if !themeSet {
						t.Fatalf("theme control did not select %s", theme)
					}
					var capture struct {
						Data string `json:"data"`
					}
					if err := page.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &capture); err != nil {
						t.Fatalf("capture %s %dx900 %s Overview: %v", provider.name, width, theme, err)
					}
					png, err := base64.StdEncoding.DecodeString(capture.Data)
					if err != nil {
						t.Fatalf("decode screenshot: %v", err)
					}
					path := filepath.Join(evidenceDir, "m3-"+provider.name+"-overview-"+strconv.Itoa(width)+"-"+theme+".png")
					if err := os.WriteFile(path, png, 0600); err != nil {
						t.Fatalf("write screenshot: %v", err)
					}
					t.Logf("captured actual %s Overview %dx900 theme=%s: %s", provider.name, width, theme, path)
				}
			}
			copyAvailable := evalM1Browser[bool](t, ctx, page, `!!document.querySelector('[data-copy-description]:not([hidden])')`)
			if !copyAvailable {
				t.Fatal("actual Overview canonical-copy control is unavailable")
			}
			copied := evalM1Browser[bool](t, ctx, page, `(()=>{const button=document.querySelector('[data-copy-description]');Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.__m3Copied=text}}});button.click();return true})()`)
			if !copied {
				t.Fatal("could not invoke canonical-copy control")
			}
			waitM1Browser(t, ctx, page, `window.__m3Copied!==undefined && document.querySelector('[data-copy-description-status]')?.textContent==='Job description copied.'`)
			if got := evalM1Browser[string](t, ctx, page, "window.__m3Copied"); got != strings.TrimSpace(job.Description) {
				t.Fatalf("%s canonical copy changed: got %q want %q", provider.name, got, strings.TrimSpace(job.Description))
			}

			for _, id := range provider.sectionIDs {
				linkFound := evalM1Browser[bool](t, ctx, page, `!!document.querySelector('nav.job-section-navigation a[href="#`+id+`"]')`)
				if !linkFound {
					t.Fatalf("priority section navigation is missing target %s", id)
				}
				clicked := evalM1Browser[bool](t, ctx, page, `(()=>{document.querySelector('nav.job-section-navigation a[href="#`+id+`"]')?.click();return true})()`)
				if !clicked {
					t.Fatalf("could not navigate to priority section %s", id)
				}
				waitM1Browser(t, ctx, page, `(()=>{const target=document.getElementById(`+strconv.Quote(id)+`),rect=target?.getBoundingClientRect();return location.hash===`+strconv.Quote("#"+id)+` && document.activeElement?.id===`+strconv.Quote(id)+` && !!rect && rect.top>=0 && rect.top<innerHeight})()`)
			}
			job, err = app.jobByID(jobID)
			if err != nil || job.SubmissionCount != 0 {
				t.Fatalf("qualification changed application submission state: job=%+v err=%v", job, err)
			}
			t.Logf("%s controlled Overview PASS: canonical bytes=%d; copied bytes=%d; source=%s; no employer application opened", provider.name, len(job.Description), len(strings.TrimSpace(job.Description)), provider.url)
		})
	}
	if requests := transport.snapshot(); len(requests) != len(providers) {
		t.Fatalf("provider source transport saw %d requests, want %d: %v", len(requests), len(providers), requests)
	}
}
