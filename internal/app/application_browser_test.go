package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/storage"
)

func TestApplicationBrowserDefaultIsChromium(t *testing.T) {
	a, _ := testWorkspace(t)
	t.Setenv("YOURMOVE_APPLICATION_BROWSER", "")
	got, err := a.resolveApplicationBrowser()
	if err != nil {
		t.Fatal(err)
	}
	if got != "chromium" {
		t.Fatalf("default browser = %q, want chromium", got)
	}
}

func TestApplicationBrowserPreferenceRoundTrip(t *testing.T) {
	a, _ := testWorkspace(t)
	t.Setenv("YOURMOVE_APPLICATION_BROWSER", "")
	if err := a.saveApplicationBrowserSetting("firefox"); err != nil {
		t.Fatal(err)
	}
	saved, err := a.applicationBrowserSetting()
	if err != nil {
		t.Fatal(err)
	}
	if saved != "firefox" {
		t.Fatalf("saved preference = %q, want firefox", saved)
	}
	got, err := a.resolveApplicationBrowser()
	if err != nil {
		t.Fatal(err)
	}
	if got != "firefox" {
		t.Fatalf("resolved browser = %q, want firefox", got)
	}
	if err := a.saveApplicationBrowserSetting("chromium"); err != nil {
		t.Fatal(err)
	}
	got, err = a.resolveApplicationBrowser()
	if err != nil {
		t.Fatal(err)
	}
	if got != "chromium" {
		t.Fatalf("resolved browser = %q, want chromium", got)
	}
}

func TestApplicationBrowserRejectsUnsupported(t *testing.T) {
	a, _ := testWorkspace(t)
	for _, bad := range []string{"", "webkit", "safari", "CHROMIUM", "firefox "} {
		if err := a.saveApplicationBrowserSetting(bad); err == nil {
			t.Fatalf("save %q succeeded, want refusal", bad)
		}
	}
	saved, err := a.applicationBrowserSetting()
	if err != nil {
		t.Fatal(err)
	}
	if saved != "" {
		t.Fatalf("refused save changed preference to %q", saved)
	}
}

func TestApplicationBrowserEnvOverridesPreference(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveApplicationBrowserSetting("firefox"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("YOURMOVE_APPLICATION_BROWSER", "chromium")
	got, err := a.resolveApplicationBrowser()
	if err != nil {
		t.Fatal(err)
	}
	if got != "chromium" {
		t.Fatalf("env override resolved %q, want chromium", got)
	}
}

func TestApplicationBrowserUnknownStoredFallsBack(t *testing.T) {
	a, _ := testWorkspace(t)
	t.Setenv("YOURMOVE_APPLICATION_BROWSER", "")
	if _, err := a.db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)`, settingApplicationBrowser, "webkit"); err != nil {
		t.Fatal(err)
	}
	got, err := a.resolveApplicationBrowser()
	if err != nil {
		t.Fatal(err)
	}
	if got != "chromium" {
		t.Fatalf("unknown stored value resolved %q, want chromium", got)
	}
	// The unknown value is preserved, not silently rewritten.
	saved, err := a.applicationBrowserSetting()
	if err != nil {
		t.Fatal(err)
	}
	if saved != "webkit" {
		t.Fatalf("stored value = %q, want preserved webkit", saved)
	}
}

func TestApplicationBrowserPriorLayoutMigratesCleanly(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.db")
	db, err := storage.OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO jobs(url, host, title) VALUES('https://example.test/role', 'example.test', 'Support Engineer')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reopened, err := storage.OpenDB(legacy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	var count int
	if err := reopened.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("legacy jobs after settings migration = %d, want 1", count)
	}
	var table string
	if err := reopened.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='settings'`).Scan(&table); err != nil {
		t.Fatalf("settings table missing after reopen: %v", err)
	}
}

func TestApplicationBrowserMissingBinaryFailsCleanly(t *testing.T) {
	a, _ := testWorkspace(t)
	t.Setenv("YOURMOVE_APPLICATION_BROWSER", "")
	t.Setenv("YOURMOVE_CHROMIUM", "/nonexistent/chrome")
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", t.TempDir())
	if err := a.saveApplicationBrowserSetting("chromium"); err != nil {
		t.Fatal(err)
	}
	result, err := insertJob(a.db, Job{
		URL:  "https://jobs.ashbyhq.com/synthetic/00000000-0000-4000-8000-000000000001",
		Host: "jobs.ashbyhq.com", Title: "Missing Binary Probe",
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.browsers.start(context.Background(), id, "https://jobs.ashbyhq.com/synthetic/00000000-0000-4000-8000-000000000001"); err == nil {
		t.Fatal("session started without a browser binary")
	} else if !strings.Contains(err.Error(), "executable") {
		t.Fatalf("missing-binary error is unclear: %v", err)
	}
	if a.browsers.has(id) {
		t.Fatal("failed launch left a session behind")
	}
	job, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if job.Title != "Missing Binary Probe" {
		t.Fatal("failed launch altered the saved job")
	}
}

func TestUtilitiesBrowserSaveRoundTrip(t *testing.T) {
	a, _ := testWorkspace(t)
	t.Setenv("YOURMOVE_APPLICATION_BROWSER", "")
	form := url.Values{"browser": {"firefox"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/utilities/browser", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.saveApplicationBrowser(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save status = %d, want 303", rec.Code)
	}
	got, err := a.resolveApplicationBrowser()
	if err != nil {
		t.Fatal(err)
	}
	if got != "firefox" {
		t.Fatalf("resolved after POST = %q, want firefox", got)
	}
	page := httptest.NewRecorder()
	a.utilitiesPage(page, httptest.NewRequest(http.MethodGet, "/utilities", nil))
	body := page.Body.String()
	if !strings.Contains(body, `value="firefox" selected`) {
		t.Fatal("utilities page does not mark the saved browser selected")
	}
	if strings.Contains(body, "not available in the interface yet") {
		t.Fatal("utilities page still shows the parked-browser copy")
	}
}

func TestUtilitiesBrowserSaveRejectsInvalid(t *testing.T) {
	a, _ := testWorkspace(t)
	form := url.Values{"browser": {"webkit"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/utilities/browser", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.saveApplicationBrowser(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid save status = %d, want 200 with explanation", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "cannot be selected") {
		t.Fatal("invalid save does not explain the refusal")
	}
}
