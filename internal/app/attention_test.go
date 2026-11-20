package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/platforms"
)

// KAN-15: attention placement is independent of stage lifecycle.

// New single intake defaults to Saved; explicit focus opts into Focus.
func TestAttentionSingleIntakeDefaultSaved(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(ctx context.Context, rawURL string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "not_checked", ""
	}
	id, _, err := a.savePostingURL(context.Background(), "https://example.com/saved-default", "")
	if err != nil {
		t.Fatal(err)
	}
	job, err := a.jobByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if job.Attention() != "saved" {
		t.Fatalf("new single should default Saved, got %q parked=%q", job.Attention(), job.ParkedAt)
	}
	focusID, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/focus-explicit", "", true)
	if err != nil {
		t.Fatal(err)
	}
	focused, err := a.jobByID(focusID)
	if err != nil {
		t.Fatal(err)
	}
	if focused.Attention() != "focus" {
		t.Fatalf("explicit focus should land in Focus, got %q", focused.Attention())
	}
}

// Bulk imports land new jobs in Saved; duplicates never move placement.
func TestAttentionBulkSavedAndDuplicatePreserved(t *testing.T) {
	a, _ := testWorkspace(t)
	first, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/keep-focus", "", true)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.importJobBatch("https://example.com/keep-focus\nhttps://example.com/bulk-new-1\nhttps://example.com/bulk-new-2")
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 2 || result.Duplicates != 1 {
		t.Fatalf("bad batch: %+v", result)
	}
	kept, err := a.jobByID(first)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Attention() != "focus" {
		t.Fatalf("duplicate import moved placement: %q", kept.Attention())
	}
	for _, item := range result.Items {
		if item.Status != "added" {
			continue
		}
		job, err := a.jobByID(item.JobID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Attention() != "saved" {
			t.Fatalf("bulk new %d should be Saved, got %q", item.JobID, job.Attention())
		}
	}
}

// Moves are reversible and never change stage or preparation.
func TestAttentionMovePreservesStage(t *testing.T) {
	a, _ := testWorkspace(t)
	id, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/move-me", "", true)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := a.jobByID(id)
	if err := a.setJobAttention(id, "saved", "2026-11-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	moved, _ := a.jobByID(id)
	if moved.Attention() != "saved" || moved.Stage != before.Stage {
		t.Fatalf("saved move changed lifecycle: %+v was %+v", moved, before)
	}
	if err := a.setJobAttention(id, "focus", ""); err != nil {
		t.Fatal(err)
	}
	back, _ := a.jobByID(id)
	if back.Attention() != "focus" || back.Stage != before.Stage {
		t.Fatalf("focus move changed lifecycle: %+v was %+v", back, before)
	}
}

// Recommendation and dashboard count only Focus; History overrides placement.
func TestAttentionRecommendationIgnoresSavedAndHistory(t *testing.T) {
	jobs := []Job{
		{ID: 1, Stage: StageReview, ParkedAt: "2026-11-17T00:00:00Z"},
		{ID: 2, Stage: StageReview},
		{ID: 3, Stage: StageReview, SubmissionCount: 1, ParkedAt: ""},
		{ID: 4, Stage: StageClosed},
	}
	next := recommendNext(jobs)
	if next == nil || next.JobID != 2 {
		t.Fatalf("recommendation should pick only Focus job 2, got %+v", next)
	}
	page := summarizeJobs(jobs)
	if page.Remaining != 1 || page.Focus != 1 || page.Saved != 1 || page.History != 2 {
		t.Fatalf("summary miscounts attention: %+v", page)
	}
}

// API attention moves round-trip; terminal jobs stay History.
func TestAttentionAPIAndHistoryContract(t *testing.T) {
	a, _ := testWorkspace(t)
	id, _, err := a.savePostingURL(context.Background(), "https://example.com/api-move", "")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"ids":[` + strconv.FormatInt(id, 10) + `],"attention":"focus"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/attention", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("attention move: %d %s", rec.Code, rec.Body.String())
	}
	job, _ := a.jobByID(id)
	if job.Attention() != "focus" || job.Stage != StageReview {
		t.Fatalf("API move changed lifecycle: %+v", job)
	}
	// Record a submission; History must win even though placement is Focus.
	if err := a.recordSubmission(id, "manual", reviewedSubmissionToken(t, a, id)); err != nil {
		t.Fatal(err)
	}
	after, _ := a.jobByID(id)
	if after.Attention() != "history" {
		t.Fatalf("submitted job should be History, got %q", after.Attention())
	}
	// Applied label persists through attention endpoint.
	var list struct {
		Jobs []opportunityJobJSON `json:"jobs"`
		Next *NextAction          `json:"next"`
	}
	listRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "http://localhost/api/opportunities", nil))
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Jobs) != 1 || list.Jobs[0].Attention != "history" {
		t.Fatalf("list should surface History: %s", listRec.Body.String())
	}
	if list.Next != nil {
		t.Fatalf("submitted job must not be recommended: %+v", list.Next)
	}
}

// Bulk moves are atomic: one missing ID rolls the whole batch back.
func TestAttentionMoveAtomicRollbackOnMissing(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(ctx context.Context, rawURL string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "not_checked", ""
	}
	first, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/atomic-one", "", true)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/atomic-two", "", true)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"ids":[` + strconv.FormatInt(first, 10) + `,` + strconv.FormatInt(second, 10) + `,999999],"attention":"saved"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/attention", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict && rec.Code != http.StatusNotFound {
		t.Fatalf("missing ID should conflict, got %d %s", rec.Code, rec.Body.String())
	}
	for _, id := range []int64{first, second} {
		job, err := a.jobByID(id)
		if err != nil {
			t.Fatal(err)
		}
		if job.Attention() != "focus" {
			t.Fatalf("partial move committed for %d: %q", id, job.Attention())
		}
	}
}

// Invalid IDs move nothing.
func TestAttentionMoveRejectsInvalidID(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(ctx context.Context, rawURL string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "not_checked", ""
	}
	valid, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/atomic-valid", "", true)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"ids":[` + strconv.FormatInt(valid, 10) + `,0],"attention":"saved"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/attention", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID should be 400, got %d %s", rec.Code, rec.Body.String())
	}
	job, _ := a.jobByID(valid)
	if job.Attention() != "focus" {
		t.Fatalf("valid job moved despite invalid batchmate: %+v", job)
	}
}

// Terminal enforcement lives inside the move transaction: stale History
// requests (a submission landing between list and move) move nothing.
func TestAttentionMoveRejectsTerminalAtomically(t *testing.T) {
	a, _ := testWorkspace(t)
	a.enrichmentFetch = func(ctx context.Context, rawURL string) (platforms.Metadata, string, string) {
		return platforms.Metadata{}, "not_checked", ""
	}
	valid, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/stale-valid", "", true)
	if err != nil {
		t.Fatal(err)
	}
	stale, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/stale-terminal", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.recordSubmission(stale, "manual", reviewedSubmissionToken(t, a, stale)); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReader(`{"ids":[` + strconv.FormatInt(valid, 10) + `,` + strconv.FormatInt(stale, 10) + `],"attention":"saved"}`)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/attention", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMux(a).ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("terminal selection should conflict, got %d %s", rec.Code, rec.Body.String())
	}
	kept, _ := a.jobByID(valid)
	if kept.Attention() != "focus" {
		t.Fatalf("valid job moved despite terminal batchmate: %+v", kept)
	}
	history, _ := a.jobByID(stale)
	if history.Attention() != "history" {
		t.Fatalf("terminal job left History: %+v", history)
	}
	// Single stale move also refuses without touching placement.
	single := strings.NewReader(`{"ids":[` + strconv.FormatInt(stale, 10) + `],"attention":"focus"}`)
	singleReq := httptest.NewRequest(http.MethodPost, "http://localhost/api/opportunities/attention", single)
	singleReq.Header.Set("Content-Type", "application/json")
	singleRec := httptest.NewRecorder()
	newMux(a).ServeHTTP(singleRec, singleReq)
	if singleRec.Code != http.StatusConflict {
		t.Fatalf("single terminal move should conflict, got %d %s", singleRec.Code, singleRec.Body.String())
	}
	// Closed stage without submissions is History too: close an unfinished
	// job via updateJob and confirm the move refuses.
	closable, _, err := a.savePostingURLWithFocus(context.Background(), "https://example.com/stale-closed", "", true)
	if err != nil {
		t.Fatal(err)
	}
	closing, err := a.jobByID(closable)
	if err != nil {
		t.Fatal(err)
	}
	closing.Stage = StageClosed
	if err := a.updateJob(closing); err != nil {
		t.Fatal(err)
	}
	if _, err := a.moveJobsAttention([]int64{closable}, "saved", ""); err != ErrAttentionTerminal {
		t.Fatalf("closed job move should refuse as terminal, got %v", err)
	}
}


