package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/yourmove-lite/internal/platforms"
)

func waitForEnrichment(t *testing.T, a *App, condition func(EnrichmentSnapshot) bool) EnrichmentSnapshot {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state := a.currentEnrichment()
		if condition(state) {
			return state
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("metadata run did not reach condition: %+v", a.currentEnrichment())
	return EnrichmentSnapshot{}
}

func TestEnrichmentEligibilityAndManualOverrides(t *testing.T) {
	a, _ := testWorkspace(t)
	for i := 0; i < 4; i++ {
		if err := a.saveJob(Job{URL: "https://example.com/job" + string(rune('a'+i)), Host: "example.com", Title: "Title", TitleManual: i == 0, SourceStatus: "not_checked"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.recordSubmission(3, "test", reviewedSubmissionToken(t, a, 3)); err != nil {
		t.Fatal(err)
	}
	if err := a.updateJob(Job{ID: 4, Title: "Closed", Stage: StageClosed, Priority: PriorityNormal}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	count := 0
	a.enrichmentFetch = func(ctx context.Context, raw string) (platforms.Metadata, string, string) {
		mu.Lock()
		count++
		mu.Unlock()
		return platforms.Metadata{Title: "Employer title", Company: "Official", SalaryText: "$100,000"}, "retrieved", "ok"
	}
	result, err := a.startEnrichment("pending")
	if err != nil || result.Total != 2 {
		t.Fatalf("pending: %+v %v", result, err)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	mu.Lock()
	n := count
	mu.Unlock()
	if n != 2 {
		t.Fatalf("fetched %d expected 2", n)
	}
	job, err := a.jobByID(1)
	if err != nil || job.Title != "Title" || job.Company != "Official" || job.SalaryText != "$100,000" || job.SourceStatus != "retrieved" {
		t.Fatalf("manual preservation: %+v %v", job, err)
	}
	if _, err := a.startEnrichment("pending"); err != nil {
		t.Fatal(err)
	}
	if got := a.currentEnrichment(); got.Total != 0 || got.State != "complete" {
		t.Fatalf("repeated pending checks: %+v", got)
	}
}

func TestEnrichmentPauseResumeAndRetryFailures(t *testing.T) {
	a, _ := testWorkspace(t)
	for i := 0; i < 4; i++ {
		if err := a.saveJob(Job{URL: "https://example.com/job" + string(rune('a'+i)), Host: "example.com", SourceStatus: "not_checked"}); err != nil {
			t.Fatal(err)
		}
	}
	arrived := make(chan string, 4)
	release := make(chan struct{})
	a.enrichmentFetch = func(ctx context.Context, raw string) (platforms.Metadata, string, string) {
		arrived <- raw
		<-release
		return platforms.Metadata{}, "failed", "temporary network failure"
	}
	_, err := a.startEnrichment("pending")
	if err != nil {
		t.Fatal(err)
	}
	// Both bounded workers start; the other two jobs must remain queued.
	for i := 0; i < 2; i++ {
		select {
		case <-arrived:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	if err := a.pauseEnrichment(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.startEnrichment("pending"); !errors.Is(err, ErrEnrichmentBusy) {
		t.Fatalf("accepted duplicate run: %v", err)
	}
	close(release)
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.Done == 2 })
	snap := a.currentEnrichment()
	if snap.State != "paused" || snap.Done != 2 {
		t.Fatalf("pause ignored: %+v", snap)
	}
	select {
	case <-arrived:
		t.Fatal("new work started while paused")
	default:
	}
	if err := a.resumeEnrichment(); err != nil {
		t.Fatal(err)
	}
	complete := waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	if complete.Total != 4 || complete.Failed != 4 || complete.Done != 4 {
		t.Fatalf("result: %+v", complete)
	}
	a.enrichmentFetch = func(ctx context.Context, raw string) (platforms.Metadata, string, string) {
		return platforms.Metadata{Title: "Recovered"}, "retrieved", "ok"
	}
	retry, err := a.startEnrichment("failed")
	if err != nil || retry.Total != 4 {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	job, _ := a.jobByID(1)
	if job.SourceStatus != "retrieved" || job.Title != "Recovered" {
		t.Fatalf("retry not persisted: %+v", job)
	}
}

func TestEnrichmentSkipsDeletedJobAndNeverRecreatesIt(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/test", Host: "example.com", SourceStatus: "not_checked"}); err != nil {
		t.Fatal(err)
	}
	begin := make(chan struct{})
	release := make(chan struct{})
	a.enrichmentFetch = func(ctx context.Context, raw string) (platforms.Metadata, string, string) {
		close(begin)
		<-release
		return platforms.Metadata{Title: "Should never reappear"}, "retrieved", "ok"
	}
	if _, err := a.startEnrichment("pending"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-begin:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	if _, err := a.deleteJobs([]int64{1}, false); err != nil {
		t.Fatal(err)
	}
	close(release)
	result := waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	if result.Removed != 1 {
		t.Fatalf("deleted in-flight job: %+v", result)
	}
	if _, err := a.jobByID(1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("job recreated: %v", err)
	}
}

func TestEnrichmentHTTPDesktopNavigationAndNoAutomaticFetch(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(ctx context.Context, raw string) (platforms.Metadata, string, string) {
		t.Error("unrequested fetch")
		return platforms.Metadata{}, "failed", ""
	}
	get := httptest.NewRecorder()
	newMux(a).ServeHTTP(get, httptest.NewRequest(http.MethodGet, "http://localhost/enrichment", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), "Fill in the details") {
		t.Fatalf("render: %d %s", get.Code, get.Body.String())
	}
	// Starting with no pending jobs is a no-op and returns browser PRG / desktop navigation.
	browser := postApp(newMux(a), "/enrichment/start", url.Values{"mode": {"pending"}}, "", "")
	if browser.Code != 303 || browser.Header().Get("Location") != "/enrichment" {
		t.Fatalf("browser: %d", browser.Code)
	}
	desktop := postApp(newDesktopMux(a), "/enrichment/start", url.Values{"mode": {"pending"}}, "wails://localhost", "1")
	if desktop.Code != 200 || !strings.Contains(desktop.Body.String(), `window.location.replace("/enrichment")`) {
		t.Fatalf("desktop: %d %s", desktop.Code, desktop.Body.String())
	}
}

func mustToken(t *testing.T) string {
	t.Helper()
	token, err := newSubmissionToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestEnrichmentBatchCapsAtThirty(t *testing.T) {
	a, _ := testWorkspace(t)
	for i := 0; i < 31; i++ {
		if err := a.saveJob(Job{URL: fmt.Sprintf("https://example.com/job%d", i), Host: "example.com", SourceStatus: "not_checked"}); err != nil {
			t.Fatal(err)
		}
	}
	a.enrichmentFetch = func(ctx context.Context, raw string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "unsupported", "no extractor"
	}
	first, err := a.startEnrichment("pending")
	if err != nil || first.Total != 30 {
		t.Fatalf("expected 30-job cap: %+v %v", first, err)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	second, err := a.startEnrichment("pending")
	if err != nil || second.Total != 1 {
		t.Fatalf("expected remaining job: %+v %v", second, err)
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	empty, err := a.startEnrichment("pending")
	if err != nil || empty.Total != 0 {
		t.Fatalf("expected fully processed workspace: %+v %v", empty, err)
	}
}
