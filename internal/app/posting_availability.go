package app

// KAN-16 posting availability preflight. Go owns the verdict: only a
// provider-confirmed closure (the existing posting checker returning the
// "closed" outcome) retires a job to History → Closed. Unsupported providers,
// failed lookups, and ambiguous evidence never imply closure and never block
// the existing manual or managed fallbacks.

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/platforms"
)

// Availability states served to React and reused by every server-side launch
// and transfer guard. "unknown" is never a closure: callers proceed with the
// ordinary flow and surface the accompanying truthful warning.
const (
	postingAvailabilityOpen    = "open"
	postingAvailabilityClosed  = "closed"
	postingAvailabilityUnknown = "unknown"
)

// postingClosedNotice is the exact persistent inline copy shown when a
// provider-confirmed closure retires a job. React renders it verbatim; Go
// tests pin it so the wording cannot drift between transports.
const postingClosedNotice = "This job posting is closed. The employer is no longer accepting applications for this posting. We moved it to History → Closed. Your saved work is still here."

func closureNoticeFor(job Job) string {
	if job.Stage == StageClosed && job.SourceStatus == "closed" {
		return postingClosedNotice
	}
	return ""
}

// closedBlockMessage prefers the persistent posting-closure notice for
// provider-confirmed retirements so every boundary reports the same copy;
// other terminal states keep their existing message.
func closedBlockMessage(job Job, fallback string) string {
	if notice := closureNoticeFor(job); notice != "" {
		return notice
	}
	return fallback
}

func (a *App) postingCheckFetch() postingFetcher {
	if a.enrichmentFetch != nil {
		return a.enrichmentFetch
	}
	return fetchPostingDetails
}

// checkPostingAvailability reuses the existing posting checker. Only the
// trusted "closed" outcome reports a closure; every other outcome reports
// open (retrieved) or unknown with a truthful reason. It never writes.
func (a *App) checkPostingAvailability(ctx context.Context, job Job) (state, detail string) {
	if job.SubmissionCount > 0 || job.Stage == StageApplied {
		return postingAvailabilityUnknown, "This application is recorded as complete. Availability checks do not apply."
	}
	if job.Stage == StageClosed {
		if notice := closureNoticeFor(job); notice != "" {
			return postingAvailabilityClosed, notice
		}
		return postingAvailabilityClosed, "This opportunity is closed in YourMove."
	}
	if !browser.OwnsPostingURL(job.URL, job.URL) && !platforms.IsClosedBoardRedirect(job.URL) {
		return postingAvailabilityUnknown, "No automatic posting check supports this site. Availability is unverified; your saved materials are unchanged and manual options remain."
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	_, status, note := a.postingCheckFetch()(fetchCtx, job.URL)
	switch status {
	case "closed":
		if strings.TrimSpace(note) == "" {
			note = "The employer posting reports this job is closed."
		}
		return postingAvailabilityClosed, note
	case "retrieved":
		return postingAvailabilityOpen, "The posting is available."
	case "unsupported":
		return postingAvailabilityUnknown, "No automatic posting check supports this site. Availability is unverified; your saved materials are unchanged and manual options remain."
	default:
		if strings.TrimSpace(note) == "" {
			note = "The posting check did not complete."
		}
		return postingAvailabilityUnknown, note + " Availability is unverified; your saved materials are unchanged and existing options remain."
	}
}

// preflightChecked marks a request context whose posting availability was
// already verified, so a composed continuation does not re-fetch per item.
type preflightCheckedKey struct{}

func withPreflightChecked(ctx context.Context) context.Context {
	return context.WithValue(ctx, preflightCheckedKey{}, true)
}

func preflightAlreadyChecked(ctx context.Context) bool {
	checked, _ := ctx.Value(preflightCheckedKey{}).(bool)
	return checked
}

// ensurePostingOpen is the Go-owned availability gate for browser launches and
// authorized transfers. It returns an empty block message when the job may
// proceed. When the provider confirms a closure it persists the retirement
// (prep-preserving via applyPostingRefresh) and returns a user-facing block
// message. Inconclusive outcomes never block: the ordinary flow proceeds with
// its existing guards.
func (a *App) ensurePostingOpen(ctx context.Context, jobID int64) (Job, string) {
	job, err := a.jobByID(jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, "Unknown job"
		}
		return Job{}, "Unable to load opportunity"
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		return job, closedBlockMessage(job, "This application is no longer active")
	}
	if preflightAlreadyChecked(ctx) {
		return job, ""
	}
	state, detail := a.checkPostingAvailability(ctx, job)
	switch state {
	case postingAvailabilityClosed:
		checkedAt := time.Now().UTC().Format(time.RFC3339)
		if err := a.applyPostingRefresh(jobID, platforms.Metadata{}, "closed", "Provider-confirmed closure during availability preflight.", checkedAt); err != nil {
			log.Printf("persist posting closure %d: %v", jobID, err)
		}
		updated, loadErr := a.jobByID(jobID)
		if loadErr == nil {
			job = updated
		}
		if notice := closureNoticeFor(job); notice != "" {
			return job, notice
		}
		return job, detail
	default:
		return job, ""
	}
}

// GET /api/jobs/{id}/availability is the single-click preflight behind the
// React "Checking posting…" state. It never launches a browser and never
// transfers material. A confirmed closure is persisted (prep-preserving) so a
// restart surfaces the same persistent notice; inconclusive outcomes persist
// nothing and never imply closure.
func (a *App) availabilityAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusNotFound, "Unknown job")
			return
		}
		writeResumeError(w, http.StatusInternalServerError, "Unable to load opportunity")
		return
	}
	state, detail := a.checkPostingAvailability(r.Context(), job)
	notice := ""
	// Only a fresh provider verdict persists. An already-closed stored job
	// reports its standing without rewriting source evidence.
	if state == postingAvailabilityClosed && job.Stage != StageClosed && job.Stage != StageApplied && job.SubmissionCount == 0 {
		checkedAt := time.Now().UTC().Format(time.RFC3339)
		if err := a.applyPostingRefresh(jobID, platforms.Metadata{}, "closed", "Provider-confirmed closure during availability preflight.", checkedAt); err != nil {
			log.Printf("persist posting closure %d: %v", jobID, err)
		}
		if updated, loadErr := a.jobByID(jobID); loadErr == nil {
			job = updated
		}
		notice = closureNoticeFor(job)
		if notice == "" {
			notice = detail
		}
		detail = notice
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"state": state, "detail": detail, "notice": notice,
		"closed":         job.Stage == StageClosed,
		"postingClosed":  closureNoticeFor(job) != "",
		"closureNotice":  closureNoticeFor(job),
		"editingAllowed": job.Stage != StageApplied && job.Stage != StageClosed && job.SubmissionCount == 0,
		"checkedAt":      job.SourceCheckedAt,
		"job":            toWorkspaceJob(job),
	})
}
