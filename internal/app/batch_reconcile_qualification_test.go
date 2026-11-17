package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// KAN-15 batch reconciliation qualification: the server reports per-entry
// line/status/url/detail/jobId so the React client can remove only confirmed
// added/duplicate lines, retain failed/unprocessed lines for retry, and
// preserve all input on unknown outcomes. These tests use synthetic
// workspaces only and cover success, duplicates-only, mixed, unknown, and
// retry after unknown.

func TestBatchReconcileSuccessOnly(t *testing.T) {
	a, _ := testWorkspace(t)
	raw := "https://example.com/a\nhttps://example.com/b"
	result, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 2 || result.Duplicates != 0 || result.Invalid != 0 || len(result.Items) != 2 {
		t.Fatalf("success summary: %+v", result)
	}
	for _, item := range result.Items {
		if item.Status != "added" || item.JobID <= 0 {
			t.Fatalf("success item not confirmed: %+v", item)
		}
	}
}

func TestBatchReconcileDuplicatesOnly(t *testing.T) {
	a, _ := testWorkspace(t)
	if _, _, err := a.storeJobIfNew(Job{URL: "https://example.com/a", Host: "example.com"}); err != nil {
		t.Fatal(err)
	}
	result, err := a.importJobBatch("https://example.com/a\nhttps://example.com/a#frag")
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || result.Duplicates != 2 || result.Invalid != 0 {
		t.Fatalf("duplicates summary: %+v", result)
	}
	if jobs, _ := a.allJobs(); len(jobs) != 1 {
		t.Fatalf("duplicates inserted: %+v", jobs)
	}
}

func TestBatchReconcileMixedAndRetry(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: "https://example.com/keep", Host: "example.com", Title: "Manual"}); err != nil {
		t.Fatal(err)
	}
	raw := "https://example.com/keep#x\nhttps://example.com/new?posting=1\nbad-url"
	first, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if first.Added != 1 || first.Duplicates != 1 || first.Invalid != 1 || len(first.Items) != 3 {
		t.Fatalf("mixed summary: %+v", first)
	}
	// Retry with only the retained (failed) line fixed: everything confirms.
	second, err := a.importJobBatch("https://example.com/new?posting=1\nhttps://example.com/keep")
	if err != nil {
		t.Fatal(err)
	}
	if second.Added != 0 || second.Duplicates != 2 || second.Invalid != 0 {
		t.Fatalf("retry summary: %+v", second)
	}
	if jobs, _ := a.allJobs(); len(jobs) != 2 {
		t.Fatalf("retry changed row count: %+v", jobs)
	}
}

func TestBatchReconcileUnknownPreservesAllThenRetry(t *testing.T) {
	a, _ := testWorkspace(t)
	if _, err := a.db.Exec(`CREATE TRIGGER reject_reconcile BEFORE INSERT ON jobs
        WHEN NEW.url = 'https://example.com/bad'
        BEGIN SELECT RAISE(ABORT, 'synthetic database failure'); END`); err != nil {
		t.Fatal(err)
	}
	raw := "https://example.com/good\nhttps://example.com/bad"
	if _, err := a.importJobBatch(raw); err == nil {
		t.Fatal("expected synthetic failure")
	}
	if jobs, _ := a.allJobs(); len(jobs) != 0 {
		t.Fatalf("unknown outcome partially saved: %+v", jobs)
	}
	// Unknown outcome via HTTP also saves nothing and reports no changes.
	body := strings.NewReader(`{"urls":"https://example.com/good\nhttps://example.com/bad"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/import", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "no changes") {
		t.Fatalf("unknown HTTP outcome: %d %s", rec.Code, rec.Body.String())
	}
	if jobs, _ := a.allJobs(); len(jobs) != 0 {
		t.Fatalf("unknown HTTP outcome partially saved: %+v", jobs)
	}
	// Remove the fault and retry the exact preserved input: it confirms.
	if _, err := a.db.Exec(`DROP TRIGGER reject_reconcile`); err != nil {
		t.Fatal(err)
	}
	retry, err := a.importJobBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if retry.Added != 2 || retry.Duplicates != 0 || retry.Invalid != 0 {
		t.Fatalf("retry-after-unknown summary: %+v", retry)
	}
	var wire map[string]any
	rec2 := httptest.NewRecorder()
	body2 := strings.NewReader(`{"urls":"https://example.com/good"}`)
	req2 := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/import", body2)
	req2.Header.Set("Content-Type", "application/json")
	newMux(a).ServeHTTP(rec2, req2)
	if err := json.Unmarshal(rec2.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"items", "added", "duplicates", "invalid"} {
		if _, ok := wire[key]; !ok {
			t.Fatalf("retry wire missing %q: %s", key, rec2.Body.String())
		}
	}
}
