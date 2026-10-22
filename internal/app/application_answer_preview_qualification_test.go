//go:build qualification

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

func TestControlledAshbyPreviewIsReadOnlyAndSessionIsolated(t *testing.T) {
	if testing.Short() || os.Getenv("YOURMOVE_QUALIFY_PREVIEW") != "1" {
		t.Skip("opt-in isolated Ashby preview qualification")
	}
	var getCount, mutationCount atomic.Int32
	employer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutationCount.Add(1)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		getCount.Add(1)
		if r.URL.Path != "/synthetic/application" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><head><title>Synthetic Ashby Application</title></head><body>
<h1>Apply for Synthetic Role</h1>
<form method="post" action="/synthetic/submission">
 <label>Email address <input id="email" name="email" type="email" required></label>
 <section data-field-entry-id="story"><div class="ashby-application-form-question-title">Describe a time you improved a system</div><textarea id="story" name="story" required></textarea></section>
 <label>Preferred schedule <select id="schedule" name="schedule"><option value="">Choose one</option><option>Remote</option><option>Hybrid</option></select></label>
 <label>Resume <input id="resume" name="resume" type="file" accept=".pdf"></label>
 <button type="button" id="next" onclick="location.href='/synthetic/next'">Next</button>
 <button type="submit">Submit application</button>
</form></body></html>`)
	}))
	defer employer.Close()
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic role"}); err != nil {
		t.Fatal(err)
	}
	job, _ := a.jobByID(1)
	if _, err := a.saveInspection(job.ID, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: job.URL + "/application", Fields: []schema.Field{{ID: "browser:old", Label: "Earlier live field", Kind: "text"}}}); err != nil {
		t.Fatal(err)
	}
	active := &fakeBrowser{url: job.URL + "/application"}
	a.browsers.sessions = map[int64]readOnlySession{job.ID: active}
	a.browsers.states = map[int64]browserRunState{job.ID: {Destination: job.URL, Active: true, Phase: "watching", Message: "existing managed session"}}
	var previewCalls atomic.Int32
	a.previewApplication = func(ctx context.Context, raw string) (schema.Result, error) {
		previewCalls.Add(1)
		if raw != job.URL {
			return schema.Result{}, fmt.Errorf("preview used changed destination: %s", raw)
		}
		return chromium.PreviewFixture(ctx, testAshbyPosting, employer.URL+"/synthetic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	form := url.Values{"return_to": {"prepare"}}
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/jobs/1/answers/preview", strings.NewReader(form.Encode()))
	request.Host = "localhost"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "wails://localhost")
	request.Header.Set("X-Wails-Window-Id", "1")
	response := httptest.NewRecorder()
	newDesktopMux(a).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `window.location.replace("/jobs/1#prepare")`) {
		t.Fatalf("desktop preview navigation: %d %s", response.Code, response.Body.String())
	}
	if previewCalls.Load() != 1 || getCount.Load() == 0 || mutationCount.Load() != 0 {
		t.Fatalf("unexpected fixture activity: preview=%d GET=%d mutations=%d", previewCalls.Load(), getCount.Load(), mutationCount.Load())
	}
	if active.timesClosed() != 0 || active.observes != 0 || !a.browsers.has(job.ID) || a.browsers.state(job.ID).Message != "existing managed session" {
		t.Fatal("read-only preview interfered with the active managed application session")
	}
	preview, err := a.latestInspectionBySource(job.ID, "preview")
	if err != nil || preview.Platform != "ashby" || preview.Source != "preview" || preview.Completeness != "rendered_preview_partial" || len(preview.Fields) < 4 {
		t.Fatalf("preview snapshot: %+v %v", preview, err)
	}
	if _, err := a.latestInspectionBySource(job.ID, "browser"); err != nil {
		t.Fatalf("preview replaced the live browser inspection: %v", err)
	}
	for _, field := range preview.Fields {
		if !strings.HasPrefix(field.ID, "preview:") {
			t.Fatalf("preview field did not use separate identity source: %+v", field)
		}
		if field.ID == "preview:resume" && field.Kind != "file" {
			t.Fatalf("unexpected fixture upload control: %+v", field)
		}
	}
	var applicationAnswers, earlyAnswers, submissions int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM application_answers`).Scan(&applicationAnswers); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM early_application_answers`).Scan(&earlyAnswers); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissions); err != nil {
		t.Fatal(err)
	}
	if applicationAnswers != 0 || earlyAnswers != 0 || submissions != 0 {
		t.Fatalf("preview created answers or submission records: live=%d early=%d submissions=%d", applicationAnswers, earlyAnswers, submissions)
	}
	page := httptest.NewRecorder()
	newMux(a).ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1", nil))
	for _, expected := range []string{"distinct questions identified", "raw visible DOM controls normalized", "rendered preview", "Preview application questions again", "ordinary website network traffic", "No controls were filled"} {
		if !strings.Contains(page.Body.String(), expected) {
			t.Fatalf("Prepare omitted preview state %q", expected)
		}
	}
}

func TestAshbyPreviewFailureAndUnsupportedProviderPreserveSavedData(t *testing.T) {
	a, _ := testWorkspace(t)
	job, _ := earlyAshbyFixture(t, a)
	preview := schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: job.URL + "/application", Fields: []schema.Field{{ID: "preview:story", Label: "Describe a system improvement", Kind: "textarea"}}}
	if _, err := a.saveInspection(job.ID, preview); err != nil {
		t.Fatal(err)
	}
	q, err := a.earlyQuestionForGeneration(job, "preview", preview.Fingerprint(), "preview:story")
	if err != nil {
		t.Fatal(err)
	}
	if rec := postApp(newMux(a), "/jobs/1/answers/early-save", earlySaveValues(job, q, "0", "Saved preview draft"), "", ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("save preview draft: %d %s", rec.Code, rec.Body.String())
	}
	a.previewApplication = func(context.Context, string) (schema.Result, error) {
		return schema.Result{}, errors.New("synthetic CAPTCHA boundary")
	}
	rec := postApp(newMux(a), "/jobs/1/answers/preview", url.Values{"return_to": {"prepare"}}, "", "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/jobs/1?early_preview=failed#prepare" {
		t.Fatalf("preview failure did not return to Prepare: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	saved, err := a.earlyAnswer(job.ID, job.URL, "preview", preview.Fingerprint(), "preview:story")
	if err != nil || saved.Answer != "Saved preview draft" || saved.DiscoveryFingerprint != preview.Fingerprint() {
		t.Fatalf("failed preview removed the saved draft: %+v %v", saved, err)
	}
	if err := a.saveJob(Job{URL: earlyGreenhousePosting, Host: "job-boards.greenhouse.io"}); err != nil {
		t.Fatal(err)
	}
	called := false
	a.previewApplication = func(context.Context, string) (schema.Result, error) { called = true; return schema.Result{}, nil }
	unsupported := postApp(newMux(a), "/jobs/2/answers/preview", url.Values{"return_to": {"prepare"}}, "", "")
	if unsupported.Code != http.StatusUnprocessableEntity || called {
		t.Fatalf("preview ran for unsupported provider: %d called=%v", unsupported.Code, called)
	}
}

func TestControlledAshbyPreviewCancellationRemovesIsolatedProfile(t *testing.T) {
	if testing.Short() || os.Getenv("YOURMOVE_QUALIFY_PREVIEW") != "1" {
		t.Skip("opt-in isolated Ashby preview cancellation qualification")
	}
	requested := make(chan struct{}, 1)
	var mutationCount atomic.Int32
	employer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutationCount.Add(1)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		select {
		case requested <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><head><title>Slow synthetic preview</title></head><body><h1>Preview remains read only</h1></body></html>`)
	}))
	defer employer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := chromium.PreviewFixture(ctx, testAshbyPosting, employer.URL)
		result <- err
	}()
	select {
	case <-requested:
	case <-time.After(12 * time.Second):
		cancel()
		t.Fatal("preview browser did not request the controlled page")
	}
	// The empty page keeps the preview in its bounded observation window after
	// its first DOM snapshot, leaving a deterministic cancellation point.
	time.Sleep(300 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("preview cancellation returned %v", err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("canceled preview did not release its Chromium process")
	}
	if mutationCount.Load() != 0 {
		t.Fatalf("canceled preview made %d non-GET requests", mutationCount.Load())
	}
}

func earlyAshbyFixture(t *testing.T, a *App) (Job, schema.Result) {
	t.Helper()
	job := Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Synthetic Ashby role"}
	if err := a.saveJob(job); err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	return job, schema.Result{Platform: "ashby", Source: "preview", Completeness: "rendered_preview_partial", ApplicationURL: job.URL + "/application"}
}
