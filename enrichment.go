package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"example.com/yourmove-lite/internal/platforms"
)

const (
	maxEnrichmentBatch = 30
	enrichmentWorkers  = 2
)

var (
	ErrEnrichmentBusy  = errors.New("a metadata check is already running or paused")
	ErrEnrichmentState = errors.New("the metadata check is not in that state")
)

// A fetcher is injectable in tests. Production uses the existing supported ATS adapters.
type postingFetcher func(context.Context, string) (platforms.Metadata, string, string)

type EnrichmentItem struct {
	JobID  int64
	Title  string
	URL    string
	State  string // waiting, checking, retrieved, unsupported, failed, removed
	Detail string
}

type EnrichmentSnapshot struct {
	State       string // idle, running, paused, complete
	Mode        string
	Total       int
	Done        int
	Checking    int
	Retrieved   int
	Unsupported int
	Failed      int
	Removed     int
	Items       []EnrichmentItem
}

type enrichmentRun struct {
	mu     sync.Mutex
	cond   *sync.Cond
	state  string
	mode   string
	next   int
	active int
	items  []EnrichmentItem
}

func newEnrichmentRun(mode string, jobs []Job) *enrichmentRun {
	r := &enrichmentRun{state: "running", mode: mode}
	for _, job := range jobs {
		if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount != 0 {
			continue
		}
		switch mode {
		case "pending":
			if job.SourceStatus != "not_checked" && job.SourceStatus != "" {
				continue
			}
		case "failed":
			if job.SourceStatus != "failed" {
				continue
			}
		default:
			continue
		}
		title := job.Title
		if title == "" {
			title = job.Host
		}
		r.items = append(r.items, EnrichmentItem{JobID: job.ID, Title: title, URL: job.URL, State: "waiting"})
		if len(r.items) == maxEnrichmentBatch {
			break
		}
	}
	r.cond = sync.NewCond(&r.mu)
	if len(r.items) == 0 {
		r.state = "complete"
	}
	return r
}

func (r *enrichmentRun) snapshot() EnrichmentSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := EnrichmentSnapshot{State: r.state, Mode: r.mode, Total: len(r.items), Items: append([]EnrichmentItem(nil), r.items...)}
	for _, item := range r.items {
		switch item.State {
		case "checking":
			s.Checking++
		case "retrieved":
			s.Retrieved++
			s.Done++
		case "unsupported":
			s.Unsupported++
			s.Done++
		case "failed":
			s.Failed++
			s.Done++
		case "removed":
			s.Removed++
			s.Done++
		}
	}
	return s
}

func (a *App) currentEnrichment() EnrichmentSnapshot {
	a.enrichmentMu.Lock()
	run := a.enrichment
	a.enrichmentMu.Unlock()
	if run == nil {
		return EnrichmentSnapshot{State: "idle"}
	}
	return run.snapshot()
}

func (a *App) startEnrichment(mode string) (EnrichmentSnapshot, error) {
	if mode != "pending" && mode != "failed" {
		return EnrichmentSnapshot{}, fmt.Errorf("invalid metadata check mode")
	}
	a.enrichmentMu.Lock()
	defer a.enrichmentMu.Unlock()
	if a.enrichment != nil {
		existing := a.enrichment.snapshot()
		if existing.State == "running" || existing.State == "paused" {
			return existing, ErrEnrichmentBusy
		}
	}
	jobs, err := a.allJobs()
	if err != nil {
		return EnrichmentSnapshot{}, err
	}
	run := newEnrichmentRun(mode, jobs)
	a.enrichment = run
	if run.state == "running" {
		fetch := a.enrichmentFetch
		if fetch == nil {
			fetch = fetchPostingDetails
		}
		for i := 0; i < enrichmentWorkers; i++ {
			go a.enrichmentWorker(run, fetch)
		}
	}
	return run.snapshot(), nil
}

func (a *App) enrichmentWorker(r *enrichmentRun, fetch postingFetcher) {
	for {
		r.mu.Lock()
		for r.state == "paused" && r.next < len(r.items) {
			r.cond.Wait()
		}
		if r.next >= len(r.items) {
			if r.active == 0 {
				r.state = "complete"
				r.cond.Broadcast()
			}
			r.mu.Unlock()
			return
		}
		idx := r.next
		r.next++
		r.active++
		r.items[idx].State = "checking"
		item := r.items[idx]
		r.mu.Unlock()

		// The source adapter receives a bounded context as well as its HTTP timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		metadata, state, note := fetch(ctx, item.URL)
		cancel()
		if state != "retrieved" && state != "unsupported" && state != "failed" {
			state, note = "failed", "Extractor returned an unexpected outcome."
		}
		if len([]rune(note)) > 240 {
			note = string([]rune(note)[:240])
		}
		err := a.applyPostingRefresh(item.JobID, metadata, state, note, time.Now().UTC().Format(time.RFC3339))
		if errors.Is(err, sql.ErrNoRows) {
			state, note = "removed", "Job deleted while check was running."
		} else if err != nil {
			state, note = "failed", "Unable to save source result; try again."
		}

		r.mu.Lock()
		r.items[idx].State = state
		r.items[idx].Detail = note
		r.active--
		if r.next >= len(r.items) && r.active == 0 {
			r.state = "complete"
			r.cond.Broadcast()
		}
		r.mu.Unlock()
	}
}

func (a *App) pauseEnrichment() error {
	a.enrichmentMu.Lock()
	r := a.enrichment
	a.enrichmentMu.Unlock()
	if r == nil {
		return ErrEnrichmentState
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != "running" {
		return ErrEnrichmentState
	}
	r.state = "paused"
	return nil
}

func (a *App) resumeEnrichment() error {
	a.enrichmentMu.Lock()
	r := a.enrichment
	a.enrichmentMu.Unlock()
	if r == nil {
		return ErrEnrichmentState
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state != "paused" {
		return ErrEnrichmentState
	}
	r.state = "running"
	r.cond.Broadcast()
	return nil
}

// Avoid new storage models: the recorded source result itself is the durable
// checkpoint. After a restart, pending and failed checks can be started again.
func (a *App) refreshPosting(ctx context.Context, job Job) error {
	fetch := a.enrichmentFetch
	if fetch == nil {
		fetch = fetchPostingDetails
	}
	metadata, status, note := fetch(ctx, job.URL)
	return a.applyPostingRefresh(job.ID, metadata, status, note, time.Now().UTC().Format(time.RFC3339))
}
