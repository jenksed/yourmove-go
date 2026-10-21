package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNormalizePostingURL(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{" HTTPS://JOBS.ASHBYHQ.COM/board/123/#overview ", "https://jobs.ashbyhq.com/board/123"},
		{"https://example.com/jobs?a=1&posting=abc#details", "https://example.com/jobs?a=1&posting=abc"},
		{"https://example.com/", "https://example.com/"},
		{"https://example.com/job%2Fone/", "https://example.com/job%2Fone/"},
	} {
		got, _, err := normalizePostingURL(test.input)
		if err != nil || got != test.want {
			t.Errorf("normalize(%q)=%q, %v; want %q", test.input, got, err, test.want)
		}
	}
	for _, invalid := range []string{"", "file:///tmp/test", "https://user:pass@host.example/path", "javascript:alert(1)", "https://", "https://example.com/a b", "https://example.com/line\nmore"} {
		if _, _, err := normalizePostingURL(invalid); err == nil {
			t.Errorf("accepted invalid URL %q", invalid)
		}
	}
}

func TestBatchIntakeKeepsExistingDataAndDoesNotFetch(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://jobs.example.org/role/1/", Host: "jobs.example.org", Title: "Manual title", Company: "Original Company"}); err != nil {
		t.Fatal(err)
	}
	one, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	one.Title = "Protected edit"
	one.Priority = PriorityHigh
	one.Stage = StageReady
	if err := a.updateJob(one); err != nil {
		t.Fatal(err)
	}

	raw := strings.Join([]string{
		"https://jobs.example.org/role/1#posting",  // existing normalized duplicate
		"https://jobs.example.org/new?posting=123", // new, query must survive
		"not-a-url", // invalid isolated to this line
		"https://jobs.example.org/new?posting=123#details", // duplicate within pasted batch
	}, "\n")
	result, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Duplicates != 2 || result.Invalid != 1 {
		t.Fatalf("bad summary: %+v", result)
	}
	jobs, err := a.allJobs()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].Title != "Protected edit" || jobs[0].Priority != PriorityHigh || jobs[0].Stage != StageReady {
		t.Fatalf("altered existing job: %+v", jobs)
	}
	if jobs[1].URL != "https://jobs.example.org/new?posting=123" || jobs[1].SourceStatus != "not_checked" || jobs[1].SourceCheckedAt != "" {
		t.Fatalf("unexpected stored batch job: %+v", jobs[1])
	}
	again, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again.Added != 0 || again.Duplicates != 3 || again.Invalid != 1 {
		t.Fatalf("retry added duplicates: %+v", again)
	}
}

func TestBatchRejectsOverLimitWithoutAnyWrites(t *testing.T) {
	a, _ := testWorkspace(t)
	var lines []string
	for i := 0; i < maxBatchJobs+1; i++ {
		lines = append(lines, "https://example.com/job/"+strings.Repeat("x", i+1))
	}
	if _, err := a.importJobBatch(strings.Join(lines, "\n")); err == nil {
		t.Fatal("accepted batch > limit")
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 0 {
		t.Fatalf("large batch partially imported: %+v, %v", jobs, err)
	}
}

func TestBatchWebViewResultsAndCrossSiteProtection(t *testing.T) {
	a, _ := testWorkspace(t)
	send := func(handler http.Handler, origin string) *httptest.ResponseRecorder {
		values := url.Values{"job_urls": {"https://example.org/new\n<script>bad</script>"}}
		req := httptest.NewRequest(http.MethodPost, "http://localhost/jobs/import", strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if origin == "wails://localhost" {
			req.Header.Set("X-Wails-Window-Id", "1")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	deny := send(newMux(a), "https://evil.test")
	if deny.Code != http.StatusForbidden {
		t.Fatalf("cross-origin import was accepted: %d", deny.Code)
	}
	browser := send(newMux(a), "")
	if browser.Code != http.StatusOK || !strings.Contains(browser.Body.String(), "1</strong>") || strings.Contains(browser.Body.String(), "<script>bad</script>") || !strings.Contains(browser.Body.String(), "&lt;script&gt;") {
		t.Fatalf("unexpected browser import results: status=%d, body=%s", browser.Code, browser.Body.String())
	}
	desktop := send(newDesktopMux(a), "wails://localhost")
	if desktop.Code != http.StatusOK || !strings.Contains(desktop.Body.String(), "Already in your workspace") {
		t.Fatalf("desktop import results: %d", desktop.Code)
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("replayed batch inserted duplicate: %+v err=%v", jobs, err)
	}
}

func TestSingleAddDedupesWithExistingAndKeepsWailsRedirect(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/role/", Host: "example.com", Title: "Hand edited"}); err != nil {
		t.Fatal(err)
	}
	makeReq := func() *http.Request {
		values := url.Values{"job_url": {"https://EXAMPLE.com/role/#detail"}, "job_title": {"Would overwrite"}}
		req := httptest.NewRequest(http.MethodPost, "http://localhost/jobs", strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req
	}
	browser := httptest.NewRecorder()
	newMux(a).ServeHTTP(browser, makeReq())
	if browser.Code != http.StatusSeeOther || browser.Header().Get("Location") != "/jobs" {
		t.Fatalf("browser redirect: %d", browser.Code)
	}
	native := makeReq()
	native.Header.Set("Origin", "wails://localhost")
	native.Header.Set("X-Wails-Window-Id", "1")
	desktop := httptest.NewRecorder()
	newDesktopMux(a).ServeHTTP(desktop, native)
	if desktop.Code != http.StatusOK || !strings.Contains(desktop.Body.String(), `window.location.replace("/jobs")`) {
		t.Fatalf("Wails redirect: %d", desktop.Code)
	}
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 1 || jobs[0].Title != "Hand edited" {
		t.Fatalf("single add mutated duplicate: %+v err=%v", jobs, err)
	}
}

func TestDeterministicTriageExcludesCompletedAndClosed(t *testing.T) {
	jobs := []Job{
		{ID: 1, Title: "Old", Priority: PriorityNormal, Stage: StageReady, SelectedResumeID: 4},
		{ID: 2, Title: "Important", Priority: PriorityHigh, Stage: StageReview},
		{ID: 3, Title: "Applied", Priority: PriorityHigh, Stage: StageApplied, SubmissionCount: 1},
		{ID: 4, Title: "Closed", Priority: PriorityHigh, Stage: StageClosed},
	}
	best := recommendNext(jobs)
	if best == nil || best.JobID != 2 || best.Label != "Choose a resume" || best.Href != "/jobs/2#prepare" {
		t.Fatalf("unexpected next job: %+v", best)
	}
	jobs[1].Stage = StageClosed
	best = recommendNext(jobs)
	if best == nil || best.JobID != 1 || best.Label != "Open application workspace" {
		t.Fatalf("failed to choose ready job: %+v", best)
	}
	jobs[0].Stage = StageClosed
	if best = recommendNext(jobs); best != nil {
		t.Fatalf("completed/closed recommended: %+v", best)
	}
}

func TestTriageRouteAndTemplate(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &App{db: db}
	if err := a.saveJob(Job{URL: "https://example.org/role", Host: "example.org", Title: "Opportunity"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/jobs", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "SUGGESTED NEXT MOVE") || !strings.Contains(rec.Body.String(), "Import multiple job URLs") || !strings.Contains(rec.Body.String(), "Choose a resume") {
		t.Fatalf("missing triage or import: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBatchInsertIsAtomicOnSQLiteFailure(t *testing.T) {
	a, _ := testWorkspace(t)
	// An unexpected SQLite failure on the second row must not commit row one.
	_, err := a.db.Exec(`CREATE TRIGGER reject_bad_job BEFORE INSERT ON jobs
        WHEN NEW.url = 'https://example.com/bad'
        BEGIN SELECT RAISE(ABORT, 'synthetic database failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.importJobBatch("https://example.com/good\nhttps://example.com/bad")
	if err == nil {
		t.Fatal("expected SQLite insert failure")
	}
	saved, err := a.allJobs()
	if err != nil || len(saved) != 0 {
		t.Fatalf("partial batch saved: %+v, %v", saved, err)
	}
}

func TestConcurrentIntakeDoesNotDuplicate(t *testing.T) {
	a, _ := testWorkspace(t)
	const workers = 8
	var wg sync.WaitGroup
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := a.storeJobIfNew(Job{URL: "https://example.com/one", Host: "example.com"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	saved, err := a.allJobs()
	if err != nil || len(saved) != 1 {
		t.Fatalf("duplicate after concurrent intake: %+v, %v", saved, err)
	}
}
