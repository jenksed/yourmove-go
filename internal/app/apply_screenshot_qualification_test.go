//go:build qualification

package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

// shotBrowser is a deterministic stand-in for the employer browser behind
// Apply screenshots. It exercises the real React UI and the real Go transfer
// decisions with exact saved materials, without depending on form timing.
type shotBrowser struct {
	*fakeBrowser
	uncertainAnswers bool
	controls         []browser.Control
}

func shotControls() []browser.Control {
	return []browser.Control{
		{ID: "resume", Label: "Resume", Kind: "file"},
		{ID: "cover", Label: "Cover Letter", Kind: "file"},
		{ID: "first_name", Label: "First name", Kind: "text"},
		{ID: "last_name", Label: "Last name", Kind: "text"},
		{ID: "email", Label: "Email", Kind: "text"},
		{ID: "why", Label: "Why this team?", Kind: "textarea"},
		{ID: "site", Label: "Website", Kind: "text"},
	}
}

func (s *shotBrowser) Observe(context.Context) (browser.Observation, error) {
	return browser.Observation{URL: s.url, Heading: "Application", Forms: 1, Controls: s.controls}, nil
}

func (s *shotBrowser) AttachResume(context.Context, string) (browser.AttachmentResult, error) {
	return browser.AttachmentResult{Selected: true, Verified: true, Detail: "Selected for the application"}, nil
}

func (s *shotBrowser) FillContacts(_ context.Context, approved browser.ContactValues) (browser.ContactResult, error) {
	fields := []browser.ContactOutcome{
		{Key: "first_name", Status: "filled", Detail: "Filled and verified"},
		{Key: "last_name", Status: "filled", Detail: "Filled and verified"},
		{Key: "email", Status: "filled", Detail: "Filled and verified"},
	}
	_ = approved
	return browser.ContactResult{Ready: true, Fields: fields}, nil
}

func (s *shotBrowser) FillAnswer(_ context.Context, v browser.AnswerValue) (browser.AnswerOutcome, error) {
	if s.uncertainAnswers {
		return browser.AnswerOutcome{Label: v.Field.Label, Status: "uncertain", Detail: "This answer could not be verified. Review the live form."}, nil
	}
	return browser.AnswerOutcome{Label: v.Field.Label, Status: "filled", Detail: "Filled and independently observed in the live form"}, nil
}

func (s *shotBrowser) AttachCoverLetter(context.Context, string, string) (browser.AttachmentResult, error) {
	return browser.AttachmentResult{Selected: true, Verified: true, Detail: "Selected for the application"}, nil
}

func (s *shotBrowser) AttachCoverLetterAuto(context.Context, string, string) (browser.AttachmentResult, error) {
	return browser.AttachmentResult{Selected: true, Verified: true, Detail: "Selected for the application"}, nil
}

func (s *shotBrowser) FillCoverLetter(context.Context, string) (browser.AttachmentResult, error) {
	return browser.AttachmentResult{Selected: true, Verified: true, Detail: "Selected for the application"}, nil
}

func seedShotJob(t *testing.T, a *App) {
	t.Helper()
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support role", Company: "Fixture"}); err != nil {
		t.Fatal(err)
	}
	fields := []schema.Field{
		{ID: "browser:why", Label: "Why this team?", Kind: "textarea"},
		{ID: "browser:site", Label: "Website", Kind: "text"},
	}
	if _, err := a.saveInspection(1, schema.Result{Platform: "ashby", Source: "browser", Completeness: "rendered_partial", ApplicationURL: testAshbyPosting + "/application", Fields: fields}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO application_answers(job_id,destination,provider,question_key,label,answer,revision) VALUES(1,?,?,'`+questionKey(testAshbyPosting, "ashby", fields[0])+`',?,'I enjoy helping customers solve challenging support problems with clear follow-up.',1)`, testAshbyPosting, "ashby", fields[0].Label); err != nil {
		t.Fatal(err)
	}
	if err := a.saveCandidateProfile(CandidateProfile{FirstName: "Casey", LastName: "Fixture", Email: "casey@example.test", Approved: false}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("role.pdf", strings.NewReader("%PDF-1.4\nsynthetic resume bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	letterBody := "Dear Hiring Team, I am interested in this synthetic support role and would welcome the opportunity to contribute."
	if _, err := a.db.Exec(`INSERT INTO cover_letters(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at,edited_at) VALUES(1,?,'manual','none',0,'',0,'','','2026-11-07T00:00:00Z','2026-11-07T00:00:00Z')`, letterBody); err != nil {
		t.Fatal(err)
	}
}

func shotServe(t *testing.T, a *App) (*httptest.Server, *chromium.Session, context.Context, context.CancelFunc) {
	t.Helper()
	a.resources.frontendDist = os.DirFS("../..")
	a.resources.frontendIndex = loadFrontendIndex(a.resources.frontendDist)
	if len(a.resources.frontendIndex) == 0 {
		t.Fatal("build frontend first")
	}
	router := http.NewServeMux()
	router.HandleFunc("GET /application", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/jobs/1", http.StatusSeeOther) })
	router.Handle("/", newMux(a))
	app := httptest.NewServer(router)
	t.Cleanup(app.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	ui, err := chromium.LaunchFixture(ctx, app.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ui.Close() })
	return app, ui, ctx, cancel
}

func shotEval(t *testing.T, ctx context.Context, ui *chromium.Session, page, expression string, out any) {
	t.Helper()
	var reply struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	_ = reply
	if err := ui.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &reply); err != nil {
		t.Fatal(err)
	}
	if len(reply.Exception) > 0 {
		t.Fatalf("JS %s", reply.Exception)
	}
	if out != nil {
		if err := json.Unmarshal(reply.Result.Value, out); err != nil {
			t.Fatal(err)
		}
	}
}

func shotWait(t *testing.T, ctx context.Context, ui *chromium.Session, page, expression string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var ok bool
		if shotEvalNoFail(ctx, ui, page, expression, &ok) && ok {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	var body string
	_ = shotEvalNoFail(ctx, ui, page, `document.body.innerText.slice(0,1500)`, &body)
	t.Fatalf("timeout %s\n%s", expression, body)
}

func shotEvalNoFail(ctx context.Context, ui *chromium.Session, page, expression string, out any) bool {
	var reply struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := ui.FixtureCall(ctx, page, "Runtime.evaluate", map[string]any{"expression": expression, "returnByValue": true, "awaitPromise": true}, &reply); err != nil {
		return false
	}
	if len(reply.Exception) > 0 {
		return false
	}
	if out != nil {
		if err := json.Unmarshal(reply.Result.Value, out); err != nil {
			return false
		}
	}
	return true
}

func shotClick(t *testing.T, ctx context.Context, ui *chromium.Session, page, label string) {
	t.Helper()
	shotEval(t, ctx, ui, page, fmt.Sprintf(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent.trim()==="%s" && x.offsetParent!==null).click()`, label), nil)
}

func shotCapture(t *testing.T, ctx context.Context, ui *chromium.Session, page string, width, height int, theme, name string) {
	t.Helper()
	shotCaptureAt(t, ctx, ui, page, width, height, "#employer", theme, name)
}

func shotCaptureOutcomes(t *testing.T, ctx context.Context, ui *chromium.Session, page string, width, height int, theme, name string) {
	t.Helper()
	shotCaptureAt(t, ctx, ui, page, width, height, ".continuation-outcomes", theme, name)
}

func shotCaptureAt(t *testing.T, ctx context.Context, ui *chromium.Session, page string, width, height int, selector, theme, name string) {
	t.Helper()
	data, err := ui.FixtureScreenshotViewport(ctx, page, width, height, selector, theme)
	if err != nil {
		t.Fatalf("screenshot %s: %v", name, err)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(os.TempDir(), "s3-apply")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".png")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("screenshot %s", path)
}

func TestApplyScreenshotReadyAndReview(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	seedShotJob(t, a)
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		return &shotBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}, controls: shotControls()}, nil
	}
	defer a.browsers.closeAll()
	app, ui, ctx, _ := shotServe(t, a)
	page := app.URL + "/jobs/1"
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Support role')`)
	shotClick(t, ctx, ui, page, "Apply")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Fill Application')`)
	shotCapture(t, ctx, ui, page, 1280, 900, "light", "ready-desktop-light")
	shotCapture(t, ctx, ui, page, 1280, 900, "dark", "ready-desktop-dark")
	shotCapture(t, ctx, ui, page, 700, 550, "light", "ready-narrow-light")
	shotCapture(t, ctx, ui, page, 700, 550, "dark", "ready-narrow-dark")
	shotClick(t, ctx, ui, page, "Fill Application")
	shotWait(t, ctx, ui, page, `document.querySelector('.continuation-outcomes')?.innerText.includes('Résumé')`)
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Review in the employer browser')`)
	shotCapture(t, ctx, ui, page, 1280, 900, "light", "review-desktop-light")
	shotCapture(t, ctx, ui, page, 700, 550, "dark", "review-narrow-dark")
	var confirmSet bool
	shotEval(t, ctx, ui, page, `(()=>{window.confirm=()=>true;return true})()`, &confirmSet)
	shotClick(t, ctx, ui, page, "Close browser")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Browser closed')`)
	shotCapture(t, ctx, ui, page, 1280, 900, "light", "closed-desktop-light")
	shotCapture(t, ctx, ui, page, 700, 550, "light", "closed-narrow-light")
}

func TestApplyScreenshotUncertain(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_BROWSER") != "1" {
		t.Skip("opt-in controlled browser")
	}
	a, _ := testWorkspace(t)
	seedShotJob(t, a)
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		return &shotBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting + "/application"}, uncertainAnswers: true, controls: shotControls()}, nil
	}
	defer a.browsers.closeAll()
	app, ui, ctx, _ := shotServe(t, a)
	page := app.URL + "/jobs/1"
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Support role')`)
	shotClick(t, ctx, ui, page, "Apply")
	shotWait(t, ctx, ui, page, `document.body.innerText.includes('Fill Application')`)
	shotClick(t, ctx, ui, page, "Fill Application")
	shotWait(t, ctx, ui, page, `document.querySelector('.continuation-outcomes')?.innerText.includes('Résumé')`)
	// The uncertain answer surfaces through the React poll after the
	// transfer completes; wait for the rendered exception, not the Go state.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var outcomesText string
		if shotEvalNoFail(ctx, ui, page, `document.querySelector('.continuation-outcomes')?.innerText ?? 'MISSING'`, &outcomesText) {
			if strings.Contains(outcomesText, "Uncertain") {
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	shotCapture(t, ctx, ui, page, 1280, 900, "light", "uncertain-desktop-light")
	shotCapture(t, ctx, ui, page, 1280, 900, "dark", "uncertain-desktop-dark")
	shotCaptureOutcomes(t, ctx, ui, page, 1280, 900, "light", "uncertain-outcomes-light")
}
