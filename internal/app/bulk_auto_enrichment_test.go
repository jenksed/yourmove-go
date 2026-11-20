package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/platforms"
	"github.com/jenksed/yourmove-go/internal/storage"
)

// After a successful bulk import, YourMove automatically checks public
// metadata for the newly added opportunities only: URLs save promptly, new
// jobs gain titles/employers/descriptions without another screen, existing
// jobs are untouched, 50-URL imports respect the two-worker bound across
// successive batches, manual fields survive, failures stay visible and
// retryable, reimports cause no rechecks, restart keeps completed metadata
// with a usable pending path, and no employer browser opens.
func TestBulkImportAutoEnrichesNewJobsOnly(t *testing.T) {
	a, dir := testWorkspace(t)
	// A pre-existing unchecked job must never be refreshed by the automatic run.
	if err := a.saveJob(Job{URL: "https://existing.example.test/role", Host: "existing.example.test", Title: "Existing manual", SourceStatus: "not_checked"}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	calls := map[string]int{}
	countCalls := func() (map[string]int, int) {
		mu.Lock()
		defer mu.Unlock()
		out := make(map[string]int, len(calls))
		total := 0
		for k, v := range calls {
			out[k] = v
			total++
		}
		return out, total
	}
	var inFlight, maxFlight atomic.Int64
	release := make(chan struct{})
	a.enrichmentFetch = func(_ context.Context, raw string) (platforms.Metadata, string, string) {
		mu.Lock()
		calls[raw]++
		mu.Unlock()
		current := inFlight.Add(1)
		for {
			if max := maxFlight.Load(); current <= max || !maxFlight.CompareAndSwap(max, current) {
				break
			}
		}
		defer inFlight.Add(-1)
		<-release
		if strings.HasPrefix(raw, "https://fail.example.test/") {
			return platforms.Metadata{}, "failed", "synthetic failure"
		}
		if strings.HasPrefix(raw, "https://unsupported.example.test/") {
			return platforms.Metadata{}, "unsupported", "synthetic unsupported"
		}
		return platforms.Metadata{
			Title: "Imported " + raw, Company: "Example", Description: "Public details for " + raw,
		}, "retrieved", "synthetic retrieval"
	}

	lines := []string{"https://fail.example.test/a", "https://unsupported.example.test/b"}
	for i := 0; i < 48; i++ {
		lines = append(lines, "https://new.example.test/role-"+strings.Repeat("x", 3)+"/"+string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	raw := strings.Join(lines, "\n")
	results, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if results.Added != 50 || results.Duplicates != 0 || results.Invalid != 0 {
		t.Fatalf("batch=%+v", results)
	}
	var added []int64
	for _, item := range results.Items {
		if item.Status == "added" {
			added = append(added, item.JobID)
		}
	}
	// Import is prompt: nothing is checked yet when the transaction commits.
	for _, id := range added {
		if job, err := a.jobByID(id); err != nil || job.SourceStatus != "not_checked" {
			t.Fatalf("job %d checked before auto start: %+v", id, job)
		}
	}
	snap, started := a.queueAutoEnrichment(added)
	if !started {
		t.Fatalf("auto run did not start: %+v", snap)
	}
	// New jobs keep saving promptly while checks run: a manual edit during the
	// run must survive the retrieved metadata.
	edited, err := a.jobByID(added[2])
	if err != nil {
		t.Fatal(err)
	}
	edited.Title = "Manually kept"
	if err := a.updateJob(edited); err != nil {
		t.Fatal(err)
	}
	close(release)
	deadline := time.Now().Add(30 * time.Second)
	for {
		done := true
		jobs, err := a.allJobs()
		if err != nil {
			t.Fatal(err)
		}
		for _, job := range jobs {
			if job.ID == 1 {
				continue
			}
			if job.SourceStatus == "not_checked" || job.SourceStatus == "" {
				done = false
			}
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("auto enrichment did not finish: %+v", a.currentEnrichment())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if max := maxFlight.Load(); max < 1 || max > 2 {
		t.Fatalf("concurrency bound violated: max in flight %d", max)
	}
	snapshot, total := countCalls()
	if total != 50 {
		t.Fatalf("expected 50 checked URLs, got %d", total)
	}
	for rawURL, n := range snapshot {
		if n != 1 {
			t.Fatalf("URL %s checked %d times", rawURL, n)
		}
		if rawURL == "https://existing.example.test/role" {
			t.Fatal("pre-existing job entered the automatic run")
		}
	}
	kept, err := a.jobByID(1)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Title != "Existing manual" || kept.SourceStatus != "not_checked" {
		t.Fatalf("existing job touched: %+v", kept)
	}
	manual, err := a.jobByID(added[2])
	if err != nil {
		t.Fatal(err)
	}
	if manual.Title != "Manually kept" {
		t.Fatalf("manual edit overwritten: %+v", manual)
	}
	checked, err := a.jobByID(added[3])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(checked.Title, "Imported ") || checked.SourceStatus != "retrieved" {
		t.Fatalf("new job not enriched: %+v", checked)
	}
	// Failures and unsupported results stay visible with an explicit retry path.
	failed, err := a.jobByID(added[0])
	if err != nil || failed.SourceStatus != "failed" {
		t.Fatalf("failure not recorded: %+v", failed)
	}
	retry, err := a.startEnrichment("failed")
	if err != nil {
		t.Fatalf("manual retry unavailable: %v", err)
	}
	if retry.Total == 0 {
		t.Fatal("failed check not retryable")
	}
	waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	// Reimporting the same URLs causes no rechecks.
	mu.Lock()
	before := len(calls)
	mu.Unlock()
	again, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again.Added != 0 || again.Duplicates != 50 {
		t.Fatalf("reimport=%+v", again)
	}
	var redup []int64
	for _, item := range again.Items {
		if item.Status == "added" {
			redup = append(redup, item.JobID)
		}
	}
	a.queueAutoEnrichment(redup)
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	if len(calls) != before {
		t.Fatalf("reimport rechecked %d URLs", len(calls)-before)
	}
	mu.Unlock()
	// No employer browser opened and no applicant information sent.
	for _, id := range added {
		if state := a.browsers.state(id); state.Active {
			t.Fatalf("browser opened for job %d", id)
		}
	}
	// Restart keeps completed metadata and a usable pending path.
	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.OpenDB(dir + "/jobs.db")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	a.db = reopened
	restarted, err := a.jobByID(checked.ID)
	if err != nil || restarted.Title != checked.Title || restarted.SourceStatus != "retrieved" {
		t.Fatalf("completed metadata lost across restart: %+v", restarted)
	}
	still, err := a.jobByID(1)
	if err != nil || still.SourceStatus != "not_checked" {
		t.Fatalf("pending path lost across restart: %+v", still)
	}
	pending, err := a.startEnrichment("pending")
	if err != nil {
		t.Fatalf("manual pending check unavailable after restart: %v", err)
	}
	if pending.Total == 0 {
		t.Fatal("no pending work after restart")
	}
}

// The import API responds promptly with an automatic-check summary while the
// bounded engine works in the background.
func TestBulkImportAPIStartsAutomaticChecks(t *testing.T) {
	a, _ := testWorkspace(t)
	started := make(chan struct{}, 1)
	a.enrichmentFetch = func(_ context.Context, raw string) (platforms.Metadata, string, string) {
		select {
		case started <- struct{}{}:
		default:
		}
		time.Sleep(500 * time.Millisecond)
		return platforms.Metadata{Title: "T", Company: "C", Description: "D"}, "retrieved", "ok"
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/import", strings.NewReader(`{"urls":"https://new.example.test/1\nhttps://new.example.test/2"}`))
	req.Header.Set("Content-Type", "application/json")
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("import status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result BatchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Added != 2 || result.Enrichment == nil || !result.Enrichment.Started || result.Enrichment.Total != 2 {
		t.Fatalf("import response missing auto summary: %+v", result)
	}
	// Prompt: jobs are saved but not yet checked when the response returns.
	jobs, err := a.allJobs()
	if err != nil || len(jobs) != 2 {
		t.Fatalf("jobs=%+v err=%v", jobs, err)
	}
	<-started
	snap := waitForEnrichment(t, a, func(s EnrichmentSnapshot) bool { return s.State == "complete" })
	if snap.Retrieved != 2 {
		t.Fatalf("snapshot=%+v", snap)
	}
}
