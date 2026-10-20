package main

import (
	"context"
	"errors"
	"example.com/yourmove-lite/internal/application/browser/chromium"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	browser "example.com/yourmove-lite/internal/application/browser"
)

const testAshbyPosting = "https://jobs.ashbyhq.com/render/e1f2d6a2-b9e2-4f16-b863-3ca228c83e75"

type fakeBrowser struct {
	mu       sync.Mutex
	url      string
	closed   int
	observes int
}

func (f *fakeBrowser) Observe(context.Context) (browser.Observation, error) {
	f.mu.Lock()
	f.observes++
	observedURL := f.url
	f.mu.Unlock()
	return browser.Observation{URL: observedURL, Heading: "Application", Forms: 1, Controls: []browser.Control{{ID: "candidate", Label: "Name", Kind: "text", Required: true}, {ID: "authorization", Label: "Work authorization", Kind: "select", Sensitive: true, Options: []string{"Yes", "No"}}}, Actions: []browser.Action{{Label: "Continue", Method: "POST"}}}, nil
}
func (f *fakeBrowser) Close() error       { f.mu.Lock(); defer f.mu.Unlock(); f.closed++; return nil }
func (f *fakeBrowser) changeURL(u string) { f.mu.Lock(); defer f.mu.Unlock(); f.url = u }
func (f *fakeBrowser) timesClosed() int   { f.mu.Lock(); defer f.mu.Unlock(); return f.closed }
func TestManagedInspectorBrowserAndWailsWorkflow(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Role"}); err != nil {
		t.Fatal(err)
	}
	mock := &fakeBrowser{url: testAshbyPosting + "/application"}
	launched := 0
	a.browsers.factory = func(_ context.Context, raw string) (readOnlySession, error) {
		launched++
		if raw != testAshbyPosting {
			t.Fatal(raw)
		}
		return mock, nil
	}
	h := newDesktopMux(a)
	post := func(path string) int {
		r := postApp(h, path, url.Values{}, "wails://localhost", "1")
		if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `window.location.replace("/jobs/1/inspection")`) {
			t.Fatalf("bad desktop action %s: %d %s", path, r.Code, r.Body.String())
		}
		return r.Code
	}
	post("/jobs/1/browser/start")
	post("/jobs/1/browser/start")
	if launched != 1 {
		t.Fatal("browser restarted instead of reusing active session")
	}
	post("/jobs/1/browser/observe")
	item, err := a.latestInspectionBySource(1, "browser")
	if err != nil || len(item.Fields) != 2 || item.Source != "browser" || len(item.Steps) != 2 {
		t.Fatalf("browser snapshot %+v %v", item, err)
	}
	mock.changeURL(testAshbyPosting + "/application?step=2")
	post("/jobs/1/browser/observe")
	trail, err := a.inspectionSummaries(1, "browser", 12)
	if err != nil || len(trail) != 2 {
		t.Fatalf("missing step evidence %+v %v", trail, err)
	}
	post("/jobs/1/browser/stop")
	if mock.timesClosed() != 1 || a.browsers.has(1) {
		t.Fatal("browser not closed")
	}
	if _, err := a.latestInspectionBySource(1, "browser"); err != nil {
		t.Fatalf("lost persisted observation: %v", err)
	}
}
func TestManagedInspectorNeverPersistsForeignPage(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	mock := &fakeBrowser{url: "https://example.com/private"}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return mock, nil }
	_ = postApp(newMux(a), "/jobs/1/browser/start", url.Values{}, "", "")
	w := postApp(newMux(a), "/jobs/1/browser/observe", url.Values{}, "", "")
	if w.Code != http.StatusSeeOther {
		t.Fatalf("foreign page: %d", w.Code)
	}
	if _, err := a.latestInspectionBySource(1, "browser"); err == nil {
		t.Fatal("foreign observation persisted")
	}
	if err := a.saveJob(Job{URL: "https://example.com/other", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	unknown := postApp(newMux(a), "/jobs/2/browser/start", url.Values{}, "", "")
	if unknown.Code != 422 {
		t.Fatalf("unsupported provider browser started: %d", unknown.Code)
	}
	if _, err := a.deleteJobs([]int64{1}, false); err != nil {
		t.Fatal(err)
	}
	if mock.timesClosed() != 1 {
		t.Fatal("deletion left browser session alive")
	}
}
func TestSessionHostBusyAndLaunchFailure(t *testing.T) {
	var h sessionHost
	bad := errors.New("missing browser")
	h.factory = func(context.Context, string) (readOnlySession, error) { return nil, bad }
	if err := h.start(context.Background(), 1, testAshbyPosting); !errors.Is(err, bad) {
		t.Fatal(err)
	}
	mock := &fakeBrowser{}
	h.factory = func(context.Context, string) (readOnlySession, error) { return mock, nil }
	if err := h.start(context.Background(), 1, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	if err := h.start(context.Background(), 2, testAshbyPosting); err == nil {
		t.Fatal("multiple inspectors allowed")
	}
	h.closeAll()
	if mock.timesClosed() != 1 {
		t.Fatal("cleanup did not run")
	}
}

func TestInspectionWatchesSettledStepsAndStops(t *testing.T) {
	f := &fakeBrowser{url: testAshbyPosting + "/application"}
	host := sessionHost{pollEvery: 12 * time.Millisecond}
	host.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	if err := host.start(context.Background(), 8, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	var capturesMu sync.Mutex
	captures := 0
	host.watch(8, testAshbyPosting, func(o browser.Observation) (int64, error) {
		capturesMu.Lock()
		defer capturesMu.Unlock()
		captures++
		return int64(captures), nil
	})
	deadline := time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if host.state(8).Captured >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if host.state(8).Captured != 1 {
		t.Fatalf("initial stable form not captured: %+v", host.state(8))
	}
	time.Sleep(55 * time.Millisecond)
	if host.state(8).Captured != 1 {
		t.Fatal("same fields captured repeatedly")
	}
	f.changeURL(testAshbyPosting + "/application?step=2")
	deadline = time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if host.state(8).Captured == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if host.state(8).Captured != 2 {
		t.Fatalf("changed step not saved: %+v", host.state(8))
	}
	f.changeURL("https://elsewhere.example/other")
	deadline = time.Now().Add(700 * time.Millisecond)
	for time.Now().Before(deadline) {
		if host.state(8).Phase == "attention" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if host.state(8).Captured != 2 {
		t.Fatal("foreign page captured")
	}
	if err := host.close(8); err != nil {
		t.Fatal(err)
	}
	if host.state(8).Active || f.timesClosed() != 1 {
		t.Fatal("session not stopped")
	}
	capturesMu.Lock()
	count := captures
	capturesMu.Unlock()
	time.Sleep(35 * time.Millisecond)
	capturesMu.Lock()
	defer capturesMu.Unlock()
	if captures != count {
		t.Fatal("watcher saved after close")
	}
}

func TestBrowserStatusDoesNotExposeCandidateData(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	newMux(a).ServeHTTP(w, httptest.NewRequest("GET", "/jobs/1/browser-status", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"active":false`) {
		t.Fatalf("bad browser status: %d %s", w.Code, w.Body.String())
	}
}

// A handoff uses the job's selected document, never a recommended-but-unselected
// resume or an arbitrary path from an HTTP form.
type attachmentFake struct {
	fakeBrowser
	attachMu sync.Mutex
	paths    []string
	ready    bool
}

func (f *attachmentFake) AttachResume(_ context.Context, path string) (chromium.AttachmentResult, error) {
	f.attachMu.Lock()
	defer f.attachMu.Unlock()
	if !f.ready {
		return chromium.AttachmentResult{}, chromium.ErrNoResumeInput
	}
	f.paths = append(f.paths, path)
	body, err := os.ReadFile(path)
	if err != nil {
		return chromium.AttachmentResult{}, err
	}
	if !strings.Contains(string(body), "role-doc") {
		return chromium.AttachmentResult{}, errors.New("incorrect bytes")
	}
	return chromium.AttachmentResult{Selected: true, Verified: true, Detail: "File selected, employer acceptance unknown"}, nil
}
func (f *attachmentFake) count() int {
	f.attachMu.Lock()
	defer f.attachMu.Unlock()
	return len(f.paths)
}
func TestResumeHandoffPinsSelectedFileAcrossSteps(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com", Title: "Support Engineer"}); err != nil {
		t.Fatal(err)
	}
	other, err := a.importResume("wrong.pdf", strings.NewReader("%PDF-1.4\nwrong-file"))
	if err != nil {
		t.Fatal(err)
	}
	correct, err := a.importResume("Support Resume.pdf", strings.NewReader("%PDF-1.4\nrole-doc"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, correct.ID); err != nil {
		t.Fatal(err)
	}
	f := &attachmentFake{fakeBrowser: fakeBrowser{url: testAshbyPosting + "/application"}}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return f, nil }
	a.browsers.pollEvery = 15 * time.Millisecond
	mux := newDesktopMux(a)
	invalid := postApp(mux, "/jobs/1/handoff", url.Values{"confirm": {"yes"}, "resume_id": {strconv.FormatInt(other.ID, 10)}, "resume_hash": {other.SHA256}}, "wails://localhost", "1")
	if invalid.Code != 409 || a.browsers.has(1) {
		t.Fatalf("wrong resume accepted: %d", invalid.Code)
	}
	good := postApp(mux, "/jobs/1/handoff", url.Values{"confirm": {"yes"}, "resume_id": {strconv.FormatInt(correct.ID, 10)}, "resume_hash": {correct.SHA256}}, "wails://localhost", "1")
	if good.Code != 200 || !strings.Contains(good.Body.String(), `window.location.replace("/jobs/1/inspection")`) {
		t.Fatalf("bad Wails handoff %d %s", good.Code, good.Body.String())
	}
	t.Cleanup(a.browsers.closeAll)
	time.Sleep(60 * time.Millisecond)
	if a.browsers.state(1).ResumeStatus != "waiting" {
		t.Fatalf("missing field should wait %+v", a.browsers.state(1))
	}
	f.attachMu.Lock()
	f.ready = true
	f.attachMu.Unlock()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && a.browsers.state(1).ResumeStatus != "selected" {
		time.Sleep(10 * time.Millisecond)
	}
	if a.browsers.state(1).ResumeStatus != "selected" || f.count() != 1 {
		t.Fatalf("resume not attached exactly once: %+v attempts %d", a.browsers.state(1), f.count())
	}
	f.changeURL(testAshbyPosting + "/application?step=2")
	time.Sleep(70 * time.Millisecond)
	if f.count() != 1 {
		t.Fatal("reuploaded after step change")
	}
	jobs, err := a.jobByID(1)
	if err != nil || jobs.Stage == StageApplied || jobs.SelectedResumeID != correct.ID {
		t.Fatalf("handoff altered job %+v %v", jobs, err)
	}
	if err := a.browsers.close(1); err != nil {
		t.Fatal(err)
	}
	if !f.timesClosedIsOne() {
		t.Fatal("managed browser not closed")
	}
	f.attachMu.Lock()
	attachedPath := f.paths[0]
	f.attachMu.Unlock()
	if _, err := os.Stat(attachedPath); !os.IsNotExist(err) {
		t.Fatalf("temporary approved resume still on disk: %v", err)
	}
}
func (f *attachmentFake) timesClosedIsOne() bool { return f.timesClosed() == 1 }
