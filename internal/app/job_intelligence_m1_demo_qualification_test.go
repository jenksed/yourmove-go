//go:build qualification

package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

// TestJobIntelligenceM1ConnectedOverviewDemo is the early working-app review
// checkpoint. It uses the frozen M0 provider fixture and routes it through the
// production Ashby fetcher, intake, Overview, edit, and refresh handlers.
func TestJobIntelligenceM1ConnectedOverviewDemo(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_M1_DEMO") != "1" {
		t.Skip("opt-in connected Ashby Job Intelligence demonstration")
	}
	evidenceDir := os.Getenv("YOURMOVE_M1_EVIDENCE_DIR")
	if evidenceDir == "" || !filepath.IsAbs(evidenceDir) {
		t.Fatal("YOURMOVE_M1_EVIDENCE_DIR must be an absolute directory")
	}
	if err := os.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}

	fixture, err := os.ReadFile(filepath.Join("..", "platforms", "testdata", "job-intelligence", "ashby-long-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	m1Fixture := strings.Replace(string(fixture), "<h2>Interview process</h2>", "<h2>Interview process</h2><p>Read <a href=\\\"/fable/engineering-handbook\\\">the engineering handbook</a>.</p>", 1)
	if m1Fixture == string(fixture) {
		t.Fatal("M0 Ashby fixture heading was not found for the safe relative-link demo")
	}
	updatedFixture := strings.Replace(m1Fixture, "<h1>Senior Platform Engineer</h1>", "<h1>Senior Platform Engineer · Updated employer source</h1>", 1)
	if updatedFixture == m1Fixture {
		t.Fatal("M0 Ashby fixture heading was not found for the refresh response")
	}
	previousTransport := http.DefaultTransport
	transport := &m1AshbyTransport{bodies: [][]byte{[]byte(m1Fixture), []byte(updatedFixture)}, fallback: previousTransport}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	app, workspaceDir := testWorkspace(t)
	app.enrichmentFetch = fetchPostingDetails
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on IPv4 loopback for controlled Chromium: %v", err)
	}
	router := http.NewServeMux()
	var noAppScripts atomic.Bool
	var noAppScriptResponses atomic.Int32
	router.HandleFunc("GET /jobs/{id}/application", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/jobs/"+r.PathValue("id")+"#overview", http.StatusSeeOther)
	})
	appHandler := newMux(app)
	for _, script := range []string{"/static/ui.js", "/static/inspection.js"} {
		router.HandleFunc("GET "+script, func(w http.ResponseWriter, r *http.Request) {
			if noAppScripts.Load() {
				noAppScriptResponses.Add(1)
				w.WriteHeader(http.StatusNoContent)
				return
			}
			appHandler.ServeHTTP(w, r)
		})
	}
	router.Handle("/", appHandler)
	server := httptest.NewUnstartedServer(router)
	server.Listener = listener
	server.Start()
	defer server.Close()

	intakeClient := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := intakeClient.PostForm(server.URL+"/jobs", url.Values{"job_url": {"https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111"}})
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/jobs" {
		t.Fatalf("production intake response = %d Location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	var jobID int64
	if err := app.db.QueryRow(`SELECT id FROM jobs WHERE url=?`, "https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111").Scan(&jobID); err != nil {
		t.Fatalf("load intaken synthetic posting: %v", err)
	}
	job, err := app.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	canonicalHash := sha256.Sum256([]byte(job.Description))
	if got := hex.EncodeToString(canonicalHash[:]); got != "35c9e859b907f7e9043ba282564fe4d83b8da62c53e6c75b9eb06ab5735458c6" {
		t.Fatalf("canonical Ashby fixture digest = %s", got)
	}
	if job.SalaryText != "€120K - €150K" || job.SalaryOrigin != "employer" {
		t.Fatalf("canonical salary projection changed: %q (%s)", job.SalaryText, job.SalaryOrigin)
	}
	source, found, err := loadJobPostingSource(app.db, job)
	if err != nil || !found || source.Invalid || source.Source == nil || source.Source.DescriptionHTML == "" || source.Source.DescriptionPlain == "" {
		t.Fatalf("intake did not persist both selected Ashby alternatives: found=%v source=%+v err=%v", found, source, err)
	}

	jobPath := "/jobs/" + strconv.FormatInt(jobID, 10)
	pageURL := server.URL + jobPath
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	chrome, err := chromium.LaunchFixture(ctx, pageURL)
	if err != nil {
		t.Fatalf("launch controlled Chromium against app Overview (workspace %s): %v", workspaceDir, err)
	}
	defer chrome.Close()
	waitM1Session(t, ctx, chrome, pageURL, `document.readyState==='complete' && document.querySelector('[data-job-document] h3') && location.hash==='#overview'`)
	page, err := chrome.FixturePage(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()
	if err := page.Call(ctx, "Runtime.enable", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && document.querySelector('[data-job-document] h3') && location.hash==='#overview'`)

	type overviewSnapshot struct {
		Headings     []string `json:"headings"`
		FullText     string   `json:"fullText"`
		SectionJumps []struct {
			Label string `json:"label"`
			Href  string `json:"href"`
		} `json:"sectionJumps"`
		ParagraphCount    int               `json:"paragraphCount"`
		ListCount         int               `json:"listCount"`
		LinkHref          string            `json:"linkHref"`
		LinkCount         int               `json:"linkCount"`
		ForeignLinkCount  int               `json:"foreignLinkCount"`
		LinkTarget        string            `json:"linkTarget"`
		LinkRel           string            `json:"linkRel"`
		Location          string            `json:"location"`
		Workplace         string            `json:"workplace"`
		Employment        string            `json:"employment"`
		Compensation      string            `json:"compensation"`
		Freshness         string            `json:"freshness"`
		InlineScripts     int               `json:"inlineScripts"`
		InlineEventAttrs  int               `json:"inlineEventAttrs"`
		CanonicalCopyText string            `json:"canonicalCopyText"`
		Facts             map[string]string `json:"facts"`
	}
	snapshot := evalM1Browser[overviewSnapshot](t, ctx, page, `(()=>{
	 const doc=document.querySelector('[data-job-document]');
	 const facts={}; document.querySelectorAll('.fact-list>div').forEach(row=>{const dt=row.querySelector('dt'),dd=row.querySelector('dd');if(dt&&dd)facts[dt.textContent.trim()]=dd.innerText.trim()});
	 const links=[...doc.querySelectorAll('a')],link=links[0];
	 return {headings:[...doc.querySelectorAll('h3,h4,h5,h6')].map(x=>x.textContent.trim()),fullText:doc.innerText,sectionJumps:[...document.querySelectorAll('.job-section-navigation a')].map(a=>({label:a.textContent.trim(),href:a.getAttribute('href')})),paragraphCount:doc.querySelectorAll('p').length,listCount:doc.querySelectorAll('ul,ol').length,linkHref:link?.href||'',linkCount:links.length,foreignLinkCount:links.filter(a=>a.hostname==='fable.example').length,linkTarget:link?.target||'',linkRel:link?.rel||'',location:facts['Primary location']||'',workplace:facts['Workplace arrangement']||'',employment:facts['Employment type']||'',compensation:facts['Compensation']||'',freshness:document.querySelector('#source-title')?.parentElement?.innerText||'',inlineScripts:doc.querySelectorAll('script,iframe,object,embed').length,inlineEventAttrs:doc.querySelectorAll('*').length?[...doc.querySelectorAll('*')].filter(el=>[...el.attributes].some(a=>/^on/i.test(a.name))).length:0,canonicalCopyText:document.querySelector('#canonical-description-copy')?.value||'',facts};
	})()`)
	if len(snapshot.Headings) < 3 || snapshot.ParagraphCount < 3 || snapshot.ListCount < 2 {
		t.Fatalf("Overview lost employer document structure: %+v", snapshot)
	}
	if snapshot.LinkHref != "https://jobs.ashbyhq.com/fable/engineering-handbook" || snapshot.LinkCount != 1 || snapshot.ForeignLinkCount != 0 || snapshot.LinkTarget != "_blank" || !strings.Contains(snapshot.LinkRel, "noopener") || !strings.Contains(snapshot.LinkRel, "noreferrer") {
		t.Fatalf("display-only employer link contract changed: %+v", snapshot)
	}
	if snapshot.Location != "Remote - European Union" || snapshot.Workplace != "Remote" || snapshot.Employment != "Full time" || !strings.Contains(snapshot.Compensation, "€120K - €150K") {
		t.Fatalf("direct Ashby facts are missing or changed: %+v", snapshot)
	}
	if !strings.Contains(snapshot.Freshness, "latest successful retrieval") {
		t.Fatalf("freshness is not tied to the successful source retrieval: %q", snapshot.Freshness)
	}
	if snapshot.InlineScripts != 0 || snapshot.InlineEventAttrs != 0 || snapshot.CanonicalCopyText != job.Description {
		t.Fatalf("unsafe content or canonical copy projection mismatch: scripts=%d events=%d canonical_bytes=%d", snapshot.InlineScripts, snapshot.InlineEventAttrs, len(snapshot.CanonicalCopyText))
	}
	wantJumps := []string{"Responsibilities", "Qualifications", "Compensation", "Hiring process"}
	if len(snapshot.SectionJumps) != len(wantJumps) {
		t.Fatalf("priority navigation has %d links, want %d: %+v", len(snapshot.SectionJumps), len(wantJumps), snapshot.SectionJumps)
	}
	for i, label := range wantJumps {
		category := []string{"responsibilities", "qualifications", "compensation", "hiring"}[i]
		wantHref := "#job-section-" + category + "-1"
		if snapshot.SectionJumps[i].Label != label || snapshot.SectionJumps[i].Href != wantHref {
			t.Fatalf("priority jump %d = %+v, want %q at %q", i, snapshot.SectionJumps[i], label, wantHref)
		}
	}
	if !strings.Contains(snapshot.FullText, "Location and workplace") ||
		!strings.Contains(snapshot.FullText, "We value different paths into engineering.") ||
		!strings.Contains(snapshot.FullText, "the engineering handbook") {
		t.Fatalf("unclassified source content is not visible in the complete original-order reading: %q", snapshot.FullText)
	}
	orderedContent := []string{"What you will do", "What you will bring", "Location and workplace", "Compensation", "Interview process"}
	lastPosition := -1
	for _, content := range orderedContent {
		position := strings.Index(snapshot.FullText, content)
		if position <= lastPosition {
			t.Fatalf("original source order changed at %q: position=%d previous=%d", content, position, lastPosition)
		}
		lastPosition = position
	}
	t.Logf("connected Ashby Overview PASS: canonical_bytes=%d headings=%d paragraphs=%d lists=%d facts=%v", len(job.Description), len(snapshot.Headings), snapshot.ParagraphCount, snapshot.ListCount, snapshot.Facts)
	t.Logf("focused reading PASS: jumps=%v; unknown sections and source order remain visible", wantJumps)
	if err := page.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width": 700, "height": 550, "deviceScaleFactor": 1, "mobile": false,
	}, nil); err != nil {
		t.Fatalf("set compact reading viewport: %v", err)
	}
	for _, section := range []string{"responsibilities", "qualifications", "compensation", "hiring"} {
		href := "#job-section-" + section + "-1"
		expression := `(()=>{const a=document.querySelector('nav.job-section-navigation a[href="` + href + `"]');if(!a)return false;a.click();return true})()`
		if !evalM1Browser[bool](t, ctx, page, expression) {
			t.Fatalf("focused-reading link %q is unavailable", href)
		}
		targetID := strings.TrimPrefix(href, "#")
		waitM1Browser(t, ctx, page, `(()=>{const target=document.getElementById(`+strconv.Quote(targetID)+`),rect=target?.getBoundingClientRect();return location.hash===`+strconv.Quote(href)+` && document.activeElement?.id===`+strconv.Quote(targetID)+` && !!rect && rect.top>=0 && rect.top<innerHeight})()`)
	}
	if err := page.Call(ctx, "Emulation.clearDeviceMetricsOverride", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if err := page.Call(ctx, "Runtime.evaluate", map[string]any{"expression": "window.scrollTo(0,0)"}, nil); err != nil {
		t.Fatal(err)
	}

	// Exercise the same rendered page with the application's scripts unavailable.
	// Native HTML disclosures and the complete selectable canonical text remain.
	noAppScripts.Store(true)
	noJSChrome, err := chromium.LaunchFixture(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer noJSChrome.Close()
	waitM1Session(t, ctx, noJSChrome, pageURL, `document.readyState==='complete' && document.querySelector('[data-job-document] h3') && location.hash==='#overview'`)
	noJSPage, err := noJSChrome.FixturePage(ctx, pageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer noJSPage.Close()
	if err := noJSPage.Call(ctx, "Runtime.enable", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	if noAppScriptResponses.Load() < 2 {
		t.Fatalf("no-script run did not block both app scripts: responses=%d", noAppScriptResponses.Load())
	}
	if !evalM1Browser[bool](t, ctx, noJSPage, `(()=>{const a=document.querySelector('nav.job-section-navigation a[href="#job-section-hiring-1"]');if(!a)return false;a.focus();return document.activeElement===a})()`) {
		t.Fatal("priority section links are not keyboard reachable without app scripts")
	}
	for _, eventType := range []string{"keyDown", "keyUp"} {
		if err := noJSPage.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": eventType, "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13}, nil); err != nil {
			t.Fatalf("keyboard-follow section link (%s): %v", eventType, err)
		}
	}
	waitM1Browser(t, ctx, noJSPage, `(()=>{const target=document.getElementById('job-section-hiring-1'),rect=target?.getBoundingClientRect();return location.hash==='#job-section-hiring-1' && document.activeElement?.id==='job-section-hiring-1' && !!rect && rect.top>=0 && rect.top<innerHeight})()`)
	if !evalM1Browser[bool](t, ctx, noJSPage, `document.querySelector('[data-job-document]')?.innerText.includes('Location and workplace') && document.querySelector('[data-job-document]')?.innerText.includes('We value different paths into engineering.')`) {
		t.Fatal("unrecognized full-posting content disappeared without app scripts")
	}
	if opened := evalM1Browser[bool](t, ctx, noJSPage, `(()=>{const details=document.querySelector('.canonical-description-details'),summary=details?.querySelector('summary');if(!details||!summary)return false;summary.focus();return document.activeElement===summary})()`); !opened {
		t.Fatal("complete canonical text disclosure is missing or not keyboard focusable without app scripts")
	}
	for _, eventType := range []string{"keyDown", "keyUp"} {
		if err := noJSPage.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": eventType, "key": " ", "code": "Space", "windowsVirtualKeyCode": 32}, nil); err != nil {
			t.Fatalf("keyboard-toggle canonical disclosure (%s): %v", eventType, err)
		}
	}
	selectable := evalM1Browser[map[string]any](t, ctx, noJSPage, `(()=>{const details=document.querySelector('.canonical-description-details'),text=document.querySelector('#job-description'),range=document.createRange(),selection=getSelection();if(!details?.open||!text)return {open:!!details?.open,length:0};range.selectNodeContents(text);selection.removeAllRanges();selection.addRange(range);return {open:details.open,length:selection.toString().trim().length,contains:selection.toString().includes('Fable builds dependable tools')}})()`)
	if selectable["open"] != true || selectable["length"].(float64) < 1000 || selectable["contains"] != true {
		t.Fatalf("no-script canonical text is not complete and selectable: %+v", selectable)
	}
	t.Logf("no-script PASS: app scripts blocked=%d; keyboard disclosure opened; selected canonical text length=%.0f", noAppScriptResponses.Load(), selectable["length"].(float64))
	noJSChrome.Close()
	noJSPage.Close()
	noAppScripts.Store(false)

	for _, width := range []int{1280, 390} {
		if err := page.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
			"width": width, "height": 900, "deviceScaleFactor": 1, "mobile": false,
		}, nil); err != nil {
			t.Fatalf("set %dpx viewport: %v", width, err)
		}
		layout := evalM1Browser[map[string]float64](t, ctx, page, `(()=>({innerWidth,scrollWidth:document.documentElement.scrollWidth,headingHeight:document.querySelector('.read-panel .section-heading h2')?.getBoundingClientRect().height||0}))()`)
		if layout["scrollWidth"] > layout["innerWidth"] || (width == 390 && layout["headingHeight"] > 60) {
			t.Fatalf("Overview layout does not fit %dpx viewport: %+v", width, layout)
		}
		for _, theme := range []string{"light", "dark"} {
			themeSet := evalM1Browser[bool](t, ctx, page, `(()=>{const select=document.querySelector('[data-theme-choice]');if(!select)return false;select.value=`+strconv.Quote(theme)+`;select.dispatchEvent(new Event('change'));return document.documentElement.dataset.theme===`+strconv.Quote(theme)+`})()`)
			if !themeSet {
				t.Fatalf("application theme control did not set %s appearance", theme)
			}
			var capture struct {
				Data string `json:"data"`
			}
			if err := page.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &capture); err != nil {
				t.Fatalf("capture %dpx %s Overview: %v", width, theme, err)
			}
			png, err := base64.StdEncoding.DecodeString(capture.Data)
			if err != nil {
				t.Fatalf("decode %dpx %s Overview screenshot: %v", width, theme, err)
			}
			name := "ashby-m1-overview-" + strconv.Itoa(width) + "-" + theme + ".png"
			path := filepath.Join(evidenceDir, name)
			if err := os.WriteFile(path, png, 0600); err != nil {
				t.Fatalf("write Overview screenshot %s: %v", path, err)
			}
			t.Logf("captured actual Overview %dx900 theme=%s: %s", width, theme, path)
		}
	}
	if err := page.Call(ctx, "Emulation.clearDeviceMetricsOverride", map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}

	copied := evalM1Browser[bool](t, ctx, page, `(()=>{const button=document.querySelector('[data-copy-description]');if(!button||button.hidden)return false;Object.defineProperty(navigator,'clipboard',{configurable:true,value:{writeText:async text=>{window.__m1Copied=text}}});button.click();return true})()`)
	if !copied {
		t.Fatal("the actual Overview copy control was not available")
	}
	waitM1Browser(t, ctx, page, `window.__m1Copied!==undefined && document.querySelector('[data-copy-description-status]')?.textContent==='Job description copied.'`)
	if got := evalM1Browser[string](t, ctx, page, "window.__m1Copied"); got != strings.TrimSpace(job.Description) {
		t.Fatalf("Copy control payload differs from the complete canonical description: got %d bytes want %d", len(got), len(strings.TrimSpace(job.Description)))
	}

	const manualEdit = "Applicant-owned synthetic edit retained through refresh."
	editSubmitted := evalM1Browser[bool](t, ctx, page, `(()=>{const details=document.querySelector('#edit-details');if(!details)return false;details.open=true;const description=document.querySelector('#description');const form=document.querySelector('.job-edit-form');if(!description||!form)return false;description.value="Applicant-owned synthetic edit retained through refresh.";form.requestSubmit();return true})()`)
	if !editSubmitted {
		t.Fatal("could not submit the existing Edit details form")
	}
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && document.querySelector('#description')?.value==='Applicant-owned synthetic edit retained through refresh.'`)
	if got := evalM1Browser[string](t, ctx, page, `document.querySelector('[data-job-document]')?.innerText.trim()||''`); got != manualEdit {
		t.Fatalf("saved applicant edit is not the primary reading: %q", got)
	}
	if !evalM1Browser[bool](t, ctx, page, `!!document.querySelector('.employer-description-details .job-document h3')`) {
		t.Fatal("the validated employer document was not retained as a comparison after manual edit")
	}

	refreshSubmitted := evalM1Browser[bool](t, ctx, page, `(()=>{const form=document.querySelector('form[action$="/refresh"]');if(!form)return false;form.requestSubmit();return true})()`)
	if !refreshSubmitted {
		t.Fatal("could not submit the existing Refresh from posting form")
	}
	waitM1Browser(t, ctx, page, `document.readyState==='complete' && document.querySelector('#description')?.value==='Applicant-owned synthetic edit retained through refresh.'`)
	refreshedView := evalM1Browser[map[string]string](t, ctx, page, `(()=>({primary:document.querySelector('[data-job-document]')?.innerText.trim()||'',employer:document.querySelector('.employer-description-details .job-document')?.innerText||'',freshness:document.querySelector('#source-title')?.parentElement?.innerText||''}))()`)
	if refreshedView["primary"] != manualEdit || !strings.Contains(refreshedView["employer"], "Updated employer source") {
		t.Fatalf("refresh did not retain applicant text and update the secondary employer source: %+v", refreshedView)
	}
	if !strings.Contains(refreshedView["freshness"], "latest successful retrieval") {
		t.Fatalf("refresh state is not honestly presented: %q", refreshedView["freshness"])
	}
	job, err = app.jobByID(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if !job.DescriptionManual || job.Description != manualEdit || job.SalaryText != "€120K - €150K" || job.SubmissionCount != 0 {
		t.Fatalf("refresh changed applicant-owned/canonical state or recorded a submission: %+v", job)
	}
	if n := transport.requestCount(); n != 2 {
		t.Fatalf("expected only the intake and explicit refresh provider requests, got %d", n)
	}
	t.Logf("edit and refresh PASS: applicant text retained; updated source secondary; salary=%q; submissions=%d", job.SalaryText, job.SubmissionCount)
	t.Log("no employer page was opened and no submission, browser transfer, or form mutation was performed")
}

type m1AshbyTransport struct {
	mu       sync.Mutex
	bodies   [][]byte
	requests []string
	fallback http.RoundTripper
}

func (t *m1AshbyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Hostname() == "127.0.0.1" || request.URL.Hostname() == "localhost" {
		return t.fallback.RoundTrip(request)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requests = append(t.requests, request.Method+" "+request.URL.String())
	if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "api.ashbyhq.com" || request.URL.Path != "/posting-api/job-board/fable" {
		return nil, errors.New("unexpected synthetic provider request")
	}
	index := len(t.requests) - 1
	if index >= len(t.bodies) {
		return nil, errors.New("unexpected extra provider request")
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(t.bodies[index]))),
		Request:    request,
	}, nil
}

func (t *m1AshbyTransport) requestCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.requests)
}

func evalM1Browser[T any](t *testing.T, ctx context.Context, page *chromium.FixturePage, expression string) T {
	t.Helper()
	var result struct {
		Result struct {
			Value T `json:"value"`
		} `json:"result"`
	}
	if err := page.Call(ctx, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &result); err != nil {
		t.Fatalf("evaluate controlled Overview: %v", err)
	}
	return result.Result.Value
}

func waitM1Session(t *testing.T, ctx context.Context, session *chromium.Session, pageURL, expression string) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		var result struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		if err := session.FixtureCall(ctx, pageURL, "Runtime.evaluate", map[string]any{
			"expression": expression, "returnByValue": true,
		}, &result); err == nil && result.Result.Value {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("saved-job Overview did not settle: %s", expression)
}

func waitM1Browser(t *testing.T, ctx context.Context, page *chromium.FixturePage, expression string) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		var result struct {
			Result struct {
				Value bool `json:"value"`
			} `json:"result"`
		}
		if err := page.Call(ctx, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true}, &result); err == nil && result.Result.Value {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("controlled Overview did not satisfy browser condition: %s", expression)
}
