package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPostingOpensInSystemBrowserOnlyFromDesktop(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.org/job?id=24#details", Host: "example.org", Title: "Posting"}); err != nil {
		t.Fatal(err)
	}
	var opened []string
	a.openPosting = func(link string) error { opened = append(opened, link); return nil }
	desktop := newDesktopMux(a)
	page := httptest.NewRecorder()
	desktop.ServeHTTP(page, httptest.NewRequest("GET", "/jobs/1", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), `action="/jobs/1/open"`) || strings.Contains(page.Body.String(), `href="https://example.org/job?id=24#details"`) {
		t.Fatalf("desktop posting controls not native: status=%d", page.Code)
	}
	list := httptest.NewRecorder()
	desktop.ServeHTTP(list, httptest.NewRequest("GET", "/jobs", nil))
	if list.Code != 200 || !strings.Contains(list.Body.String(), `form="open-posting-1"`) || !strings.Contains(list.Body.String(), `id="open-posting-1"`) {
		t.Fatalf("desktop list posting control missing")
	}
	openedResult := postApp(desktop, "/jobs/1/open", url.Values{}, "wails://localhost", "1")
	if openedResult.Code != http.StatusOK || !strings.Contains(openedResult.Body.String(), `window.location.replace("/jobs/1")`) {
		t.Fatalf("native result: %d %s", openedResult.Code, openedResult.Body.String())
	}
	if len(opened) != 1 || opened[0] != "https://example.org/job?id=24#details" {
		t.Fatalf("unexpected browser URL: %v", opened)
	}
	denied := postApp(desktop, "/jobs/1/open", url.Values{}, "https://evil.test", "1")
	if denied.Code != http.StatusForbidden || len(opened) != 1 {
		t.Fatalf("untrusted request opened browser: %d %v", denied.Code, opened)
	}
	absent := postApp(desktop, "/jobs/999/open", url.Values{}, "wails://localhost", "1")
	if absent.Code != http.StatusNotFound || len(opened) != 1 {
		t.Fatalf("missing job opened: %d %v", absent.Code, opened)
	}
	// Desktop-only capability cannot be invoked through a normal browser-mode app.
	a.openPosting = nil
	if w := postApp(newMux(a), "/jobs/1/open", url.Values{}, "", ""); w.Code != http.StatusNotFound {
		t.Fatalf("browser action unexpectedly enabled: %d", w.Code)
	}
	browser := httptest.NewRecorder()
	newMux(a).ServeHTTP(browser, httptest.NewRequest("GET", "/jobs/1", nil))
	if browser.Code != 200 || !strings.Contains(browser.Body.String(), `href="https://example.org/job?id=24#details"`) {
		t.Fatalf("browser's regular external anchors changed")
	}
}

func TestPostingOpenFailureDoesNotChangeJobs(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.org/fail", Host: "example.org"}); err != nil {
		t.Fatal(err)
	}
	count := 0
	a.openPosting = func(link string) error { count++; return errors.New("system browser unavailable") }
	w := postApp(newDesktopMux(a), "/jobs/1/open", url.Values{}, "wails://localhost", "1")
	if w.Code != http.StatusBadGateway || count != 1 {
		t.Fatalf("failed opener status=%d count=%d", w.Code, count)
	}
	if _, err := a.jobByID(1); err != nil {
		t.Fatalf("failed opener modified job: %v", err)
	}
}
