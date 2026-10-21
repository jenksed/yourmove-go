package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	browser "example.com/yourmove-lite/internal/application/browser"
	"example.com/yourmove-lite/internal/application/browser/chromium"
)

type lostTabBrowser struct {
	mu     sync.Mutex
	err    error
	closed int
}

func (b *lostTabBrowser) Observe(context.Context) (browser.Observation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return browser.Observation{}, b.err
}
func (b *lostTabBrowser) Close() error     { b.mu.Lock(); defer b.mu.Unlock(); b.closed++; return nil }
func (b *lostTabBrowser) closedCount() int { b.mu.Lock(); defer b.mu.Unlock(); return b.closed }

func TestClosedAshbyTabRecoversWithoutDeadEnd(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Role"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("Selected.pdf", strings.NewReader("%PDF-1.4\nrole-doc"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	lost := &lostTabBrowser{err: chromium.ErrNoPageTargets}
	healthy := &fakeBrowser{url: testAshbyPosting + "/application"}
	launches := 0
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) {
		launches++
		if launches == 1 {
			return lost, nil
		}
		return healthy, nil
	}
	t.Cleanup(a.browsers.closeAll)
	native := newDesktopMux(a)
	start := postApp(native, "/jobs/1/browser/start", url.Values{}, "wails://localhost", "1")
	if start.Code != http.StatusOK {
		t.Fatalf("start %d", start.Code)
	}
	// The button used to strand people on a plain-text 409 after closing a tab.
	rec := postApp(native, "/jobs/1/browser/observe", url.Values{}, "wails://localhost", "1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `window.location.replace("/jobs/1/inspection")`) {
		t.Fatalf("capture recovery: %d %s", rec.Code, rec.Body.String())
	}
	state := a.browsers.state(1)
	if state.Active || state.Phase != "interrupted" || lost.closedCount() != 1 {
		t.Fatalf("lost browser not retired: %+v closed=%d", state, lost.closedCount())
	}
	page := httptest.NewRecorder()
	native.ServeHTTP(page, httptest.NewRequest("GET", "http://localhost/jobs/1/inspection", nil))
	for _, fragment := range []string{"Application browser closed", "Open application (no automatic upload)", "Selected.pdf", "Start a new authorized resume handoff"} {
		if !strings.Contains(page.Body.String(), fragment) {
			t.Errorf("missing recovery control %q", fragment)
		}
	}
	denied := postApp(native, "/jobs/1/browser/restart", url.Values{}, "wails://localhost", "1")
	if denied.Code != http.StatusBadRequest || launches != 1 {
		t.Fatalf("reopened without confirmation %d launches=%d", denied.Code, launches)
	}
	restarted := postApp(native, "/jobs/1/browser/restart", url.Values{"confirm_restart": {"yes"}}, "wails://localhost", "1")
	if restarted.Code != http.StatusOK || !a.browsers.has(1) || launches != 2 {
		t.Fatalf("reopen failed: %d %+v launches=%d", restarted.Code, a.browsers.state(1), launches)
	}
	if a.browsers.state(1).ResumeStatus != "" {
		t.Fatalf("reopened browser inherited upload authorization: %+v", a.browsers.state(1))
	}
	job, err := a.jobByID(1)
	if err != nil || job.SelectedResumeID != resume.ID || job.Stage == StageApplied {
		t.Fatalf("selection corrupted: %+v %v", job, err)
	}
}

func TestObserverNoticesClosedTabAndStopsPolling(t *testing.T) {
	f := &lostTabBrowser{err: chromium.ErrNoPageTargets}
	var h sessionHost
	h.pollEvery = 10 * time.Millisecond
	h.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := h.start(context.Background(), 7, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	h.watch(7, testAshbyPosting, func(browser.Observation) (int64, error) { t.Fatal("should not save"); return 0, nil })
	defer h.closeAll()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && h.state(7).Phase != "interrupted" {
		time.Sleep(5 * time.Millisecond)
	}
	s := h.state(7)
	if s.Active || s.Phase != "interrupted" || f.closedCount() != 1 {
		t.Fatalf("observer did not recover: %+v closed=%d", s, f.closedCount())
	}
}

func TestForeignLoginTabDoesNotDestroyManagedBrowser(t *testing.T) {
	f := &lostTabBrowser{err: chromium.ErrNoMatchingTab}
	h := sessionHost{pollEvery: 10 * time.Millisecond}
	h.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := h.start(context.Background(), 7, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	h.watch(7, testAshbyPosting, func(browser.Observation) (int64, error) { return 0, errors.New("unexpected save") })
	defer h.closeAll()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && h.state(7).Phase != "attention" {
		time.Sleep(5 * time.Millisecond)
	}
	if !h.has(7) || f.closedCount() != 0 {
		t.Fatal("authentication or other tab should stay open for user")
	}
}

type lostHandoffBrowser struct {
	lostTabBrowser
	attempts int
}

func (b *lostHandoffBrowser) AttachResume(context.Context, string) (chromium.AttachmentResult, error) {
	b.attempts++
	return chromium.AttachmentResult{}, errors.New("should never retry")
}

func TestLostTabDiscardsStagedResumeAndDoesNotAttemptUpload(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/Resume.pdf"
	if err := os.WriteFile(path, []byte("%PDF-1.4\nselected"), 0600); err != nil {
		t.Fatal(err)
	}
	browserInstance := &lostHandoffBrowser{lostTabBrowser: lostTabBrowser{err: chromium.ErrNoPageTargets}}
	h := sessionHost{pollEvery: 10 * time.Millisecond}
	h.factory = func(context.Context, string) (readOnlySession, error) { return browserInstance, nil }
	if err := h.start(context.Background(), 7, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	if err := h.armResume(7, dir, path, "Resume.pdf", nil); err != nil {
		t.Fatal(err)
	}
	h.watch(7, testAshbyPosting, func(browser.Observation) (int64, error) { return 0, errors.New("unexpected save") })
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && h.state(7).Phase != "interrupted" {
		time.Sleep(5 * time.Millisecond)
	}
	s := h.state(7)
	if s.Phase != "interrupted" || s.Active || s.ResumeStatus != "waiting" || browserInstance.attempts != 0 {
		t.Fatalf("unsafe recovery: %+v attempts=%d", s, browserInstance.attempts)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("staged resume still exists: %v", err)
	}
}
