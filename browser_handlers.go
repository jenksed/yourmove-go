package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	browser "example.com/yourmove-lite/internal/application/browser"
	"example.com/yourmove-lite/internal/application/browser/chromium"
)

// Read-only inspection and explicitly authorized resume attachment are separate
// capabilities. Other application writes and final submission are unsupported.
type readOnlySession interface {
	Observe(context.Context) (browser.Observation, error)
	Close() error
}

type resumeAttachmentSession interface {
	AttachResume(context.Context, string) (chromium.AttachmentResult, error)
}

type resumeHandoff struct {
	preflightFailures int
	retryable         bool
	dir               string
	path              string
	name              string
	status            string // waiting, selected, attention
	message           string
}

type browserRunState struct {
	ResumeStatus    string `json:"resume_status"`
	ResumeRetryable bool   `json:"resume_retryable"`
	ResumeMessage   string `json:"resume_message"`
	ResumeName      string `json:"resume_name"`
	Active          bool   `json:"active"`
	Phase           string `json:"phase"` // opening, watching, detected, attention, idle
	Message         string `json:"message"`
	LatestID        int64  `json:"latest_id"`
	Captured        int    `json:"captured"`
	HandoffTitle    string `json:"handoff_title"`
	HandoffDetail   string `json:"handoff_detail"`
	HandoffRecovery bool   `json:"handoff_recovery"`
}

type sessionHost struct {
	mu        sync.Mutex
	sessions  map[int64]readOnlySession
	starting  bool
	factory   func(context.Context, string) (readOnlySession, error)
	cancels   map[int64]context.CancelFunc
	states    map[int64]browserRunState
	handoffs  map[int64]*resumeHandoff
	pollEvery time.Duration
}

func (h *sessionHost) state(jobID int64) browserRunState {
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.states[jobID]
	state.Active = h.sessions[jobID] != nil
	if up := h.handoffs[jobID]; up != nil {
		state.ResumeStatus = up.status
		state.ResumeMessage = up.message
		state.ResumeName = up.name
		state.ResumeRetryable = up.retryable
	}
	if h.starting && !state.Active {
		state.Phase = "opening"
		state.Message = "Opening the employer application…"
	}
	summary := summarizeHandoff(state)
	state.HandoffTitle = summary.Title
	state.HandoffDetail = summary.Detail
	state.HandoffRecovery = summary.Recovery
	return state
}
func (h *sessionHost) setState(jobID int64, phase, message string, latestID int64, captured bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[jobID] == nil {
		return
	}
	s := h.states[jobID]
	s.Active = true
	s.Phase = phase
	s.Message = message
	if latestID > 0 {
		s.LatestID = latestID
	}
	if captured {
		s.Captured++
	}
	if h.states == nil {
		h.states = map[int64]browserRunState{}
	}
	h.states[jobID] = s
}
func (h *sessionHost) start(ctx context.Context, jobID int64, jobURL string) error {
	h.mu.Lock()
	if h.sessions[jobID] != nil {
		h.mu.Unlock()
		return nil
	}
	if h.starting || len(h.sessions) != 0 {
		h.mu.Unlock()
		return errors.New("close the existing managed browser before opening another job")
	}
	h.starting = true
	factory := h.factory
	h.mu.Unlock()
	if factory == nil {
		factory = func(c context.Context, u string) (readOnlySession, error) { return chromium.Launch(c, u) }
	}
	session, err := factory(ctx, jobURL)
	h.mu.Lock()
	h.starting = false
	if err == nil {
		if h.sessions == nil {
			h.sessions = map[int64]readOnlySession{}
		}
		h.sessions[jobID] = session
		if h.states == nil {
			h.states = map[int64]browserRunState{}
		}
		h.states[jobID] = browserRunState{Active: true, Phase: "watching", Message: "Chromium is opening the application's URL directly. Waiting for the resume field; continue manually if Ashby requires another step."}
	}
	h.mu.Unlock()
	return err
}
func (h *sessionHost) inspect(ctx context.Context, jobID int64) (browser.Observation, error) {
	h.mu.Lock()
	s := h.sessions[jobID]
	h.mu.Unlock()
	if s == nil {
		return browser.Observation{}, errors.New("open the inspection browser first")
	}
	return s.Observe(ctx)
}
func (h *sessionHost) armResume(jobID int64, dir, path, name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[jobID] == nil {
		return errors.New("browser is not open")
	}
	if _, ok := h.sessions[jobID].(resumeAttachmentSession); !ok {
		return errors.New("managed browser cannot attach files")
	}
	if h.handoffs == nil {
		h.handoffs = make(map[int64]*resumeHandoff)
	}
	if h.handoffs[jobID] != nil {
		return errors.New("resume preparation already authorized for this browser; close it to start a different document")
	}
	h.handoffs[jobID] = &resumeHandoff{dir: dir, path: path, name: name, status: "waiting", message: "Opening the application directly and looking for its resume field. The browser remains yours for any later pages."}
	return nil
}
func (h *sessionHost) tryResume(ctx context.Context, jobID int64) {
	h.mu.Lock()
	up := h.handoffs[jobID]
	s := h.sessions[jobID]
	if up == nil || up.status != "waiting" || s == nil {
		h.mu.Unlock()
		return
	}
	// Prevent concurrent upload from manual rechecks; only one watcher owns it.
	up.status = "checking"
	h.mu.Unlock()
	uploader, ok := s.(resumeAttachmentSession)
	if !ok {
		h.resumeResult(jobID, "attention", "This browser cannot attach files")
		return
	}
	result, err := uploader.AttachResume(ctx, up.path)
	if errors.Is(err, chromium.ErrNoResumeInput) {
		h.resumeResult(jobID, "waiting", "No resume field is available on this step yet. Continue in the employer browser; YourMove will check subsequent pages.")
		return
	}
	if err != nil {
		var preflight *chromium.PreflightError
		if errors.As(err, &preflight) {
			h.resumePreflightFailure(jobID)
			return
		}
		h.resumeResult(jobID, "attention", "Unable to confirm whether the employer received the resume. Inspect the attachment in Chromium; do not resend automatically. ("+err.Error()+")")
		return
	}
	if result.Selected {
		msg := result.Detail
		if !result.Verified {
			msg = "File selection was attempted but the browser could not verify it. Review the attachment manually."
		}
		h.resumeResult(jobID, "selected", msg)
		return
	}
	h.resumeResult(jobID, "attention", "Resume selection could not be confirmed; attach manually")
}
func (h *sessionHost) resumeResult(jobID int64, status, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if up := h.handoffs[jobID]; up != nil {
		up.status = status
		up.message = message
		up.retryable = false
		if status == "waiting" || status == "selected" {
			up.preflightFailures = 0
		}
	}
}

// Only transport/form-rerender errors raised before the file selection
// command are safe to retry. Stop after three failures and offer a deliberate
// recheck; do not tell users they must attach manually for a read error.
func (h *sessionHost) resumePreflightFailure(jobID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	up := h.handoffs[jobID]
	if up == nil {
		return
	}
	up.preflightFailures++
	if up.preflightFailures < 3 {
		up.status = "waiting"
		up.retryable = false
		up.message = fmt.Sprintf("Employer form changed or could not be read. Retrying before file selection (%d/3). No file sent.", up.preflightFailures)
		return
	}
	up.status = "attention"
	up.retryable = true
	up.message = "Could not identify a stable resume input after three checks. Nothing was selected or uploaded. Use Retry resume check after the page finishes loading."
}

func (h *sessionHost) retryResumePreflight(jobID int64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	up := h.handoffs[jobID]
	if h.sessions[jobID] == nil || up == nil || up.status != "attention" || !up.retryable {
		return errors.New("no safe pre-attachment retry available; review the current employer file field manually")
	}
	up.preflightFailures = 0
	up.retryable = false
	up.status = "waiting"
	up.message = "Rechecking the resume control without changing your application."
	return nil
}

// interrupt releases a lost Chrome process, cancels the watcher and deletes
// staged resume bytes. It retains only explanatory status, never authority to
// replay an earlier attachment. Existing SQLite inspection history is untouched.
func (h *sessionHost) interrupt(jobID int64, reason string) {
	before := h.state(jobID)
	_ = h.close(jobID)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.states == nil {
		h.states = make(map[int64]browserRunState)
	}
	h.states[jobID] = browserRunState{
		Phase: "interrupted", Message: reason,
		ResumeName: before.ResumeName, ResumeStatus: before.ResumeStatus,
		ResumeMessage: before.ResumeMessage,
		LatestID:      before.LatestID, Captured: before.Captured,
	}
}

// reportLaunchFailure is shown on the normal inspection screen rather than
// leaving the user stuck on a raw HTTP error document.
func (h *sessionHost) reportLaunchFailure(jobID int64, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.states == nil {
		h.states = make(map[int64]browserRunState)
	}
	h.states[jobID] = browserRunState{Phase: "interrupted", Message: "Could not reopen the employer browser: " + err.Error() + ". Check Chromium and try Reopen application."}
}

func (h *sessionHost) has(jobID int64) bool { return h.state(jobID).Active }
func (h *sessionHost) watch(jobID int64, jobURL string, save func(browser.Observation) (int64, error)) {
	h.mu.Lock()
	if h.sessions[jobID] == nil || h.cancels[jobID] != nil {
		h.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	if h.cancels == nil {
		h.cancels = map[int64]context.CancelFunc{}
	}
	h.cancels[jobID] = cancel
	interval := h.pollEvery
	if interval <= 0 {
		interval = 2200 * time.Millisecond
	}
	h.mu.Unlock()
	go func() {
		defer func() { h.mu.Lock(); delete(h.cancels, jobID); h.mu.Unlock() }()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		var previous, pending string
		pendingCount := 0
		unavailable := 0
		missingPages := 0
		for {
			if ctx.Err() != nil {
				return
			}
			scanCtx, cancelScan := context.WithTimeout(ctx, 9*time.Second)
			observation, err := h.inspect(scanCtx, jobID)
			cancelScan()
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				if errors.Is(err, chromium.ErrNoPageTargets) {
					missingPages++
					if missingPages >= 2 {
						h.interrupt(jobID, "The application tab was closed. Reopen the application to continue. Any unsaved employer form values and attachment state may have been lost; no file will be uploaded automatically.")
						return
					}
				} else {
					missingPages = 0
				}
				if errors.Is(err, chromium.ErrBrowserUnavailable) {
					unavailable++
					if unavailable >= 2 {
						h.interrupt(jobID, "Chromium was closed or disconnected. Reopen the application to continue. Unsaved employer changes may have been lost; YourMove will not replay the resume upload.")
						return
					}
				} else {
					unavailable = 0
				}
				if errors.Is(err, chromium.ErrNoMatchingTab) {
					h.setState(jobID, "attention", "This browser is on another page (possibly sign-in). Return to this Ashby application, or use Reopen application to start over. No document was resent.", 0, false)
				} else {
					h.setState(jobID, "attention", "This page cannot be inspected right now. Return to the Ashby application, or use Reopen application to restart from its posting.", 0, false)
				}
				pending = ""
				pendingCount = 0
			} else {
				unavailable = 0
				missingPages = 0
				// Avoid duplicate database writes: require a stable observed form
				// across two successive reads. Posting pages without controls are skipped.
				if !browser.OwnsAshbyURL(jobURL, observation.URL) {
					h.setState(jobID, "attention", "The managed browser is not showing this saved Ashby job. Return to the application; nothing was saved.", 0, false)
					pending = ""
					pendingCount = 0
				} else if len(observation.Controls) == 0 && observation.Forms == 0 && len(observation.Actions) == 0 && len(observation.Frames) == 0 {
					h.setState(jobID, "watching", "Waiting for the application form to finish loading. If this job requires another step, continue in Chromium.", 0, false)
					pending = ""
					pendingCount = 0
				} else {
					// Deterministic non-sensitive snapshot identity; no values read.
					result, conversionErr := browser.AsResult(jobURL, observation)
					if conversionErr != nil {
						h.setState(jobID, "attention", "The browser is no longer showing this saved Ashby job. Return to the correct application.", 0, false)
						pending = ""
						pendingCount = 0
						select {
						case <-ctx.Done():
							return
						case <-tick.C:
							continue
						}
					}
					fp := result.Fingerprint()
					if fp != previous {
						if fp == pending {
							pendingCount++
						} else {
							pending = fp
							pendingCount = 1
						}
						if pendingCount >= 2 {
							id, saveErr := save(observation)
							if saveErr != nil {
								h.setState(jobID, "attention", "This browser view could not be saved. Use Capture now for details.", 0, false)
							} else {
								previous = fp
								h.setState(jobID, "detected", "New application fields captured. Continue on the employer page; further changes are watched.", id, true)
							}
						}
					} else {
						h.setState(jobID, "watching", "Monitoring this application. New steps are captured after they settle.", 0, false)
					}
				}
			}
			if ctx.Err() == nil && err == nil && browser.OwnsAshbyURL(jobURL, observation.URL) {
				upCtx, upCancel := context.WithTimeout(ctx, 12*time.Second)
				h.tryResume(upCtx, jobID)
				upCancel()
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}
func (h *sessionHost) close(jobID int64) error {
	h.mu.Lock()
	s := h.sessions[jobID]
	delete(h.sessions, jobID)
	if c := h.cancels[jobID]; c != nil {
		c()
		delete(h.cancels, jobID)
	}
	delete(h.states, jobID)
	up := h.handoffs[jobID]
	delete(h.handoffs, jobID)
	h.mu.Unlock()
	if up != nil {
		defer os.RemoveAll(up.dir)
	}
	if s == nil {
		return nil
	}
	return s.Close()
}
func (h *sessionHost) closeAll() {
	h.mu.Lock()
	var ids []int64
	for id := range h.sessions {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	for _, id := range ids {
		if err := h.close(id); err != nil {
			log.Printf("close browser for job %d: %v", id, err)
		}
	}
}

func (a *App) browserStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err = a.jobByID(id); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(a.browsers.state(id))
}
func (a *App) browserOperation(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	job, err := a.jobByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load opportunity", 500)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		http.Error(w, "Inspection is limited to active jobs", 409)
		return
	}
	if !browser.OwnsAshbyURL(job.URL, job.URL) {
		http.Error(w, "Managed inspection supports saved Ashby postings only", 422)
		return
	}
	switch r.PathValue("action") {
	case "start":
		if err = a.browsers.start(r.Context(), id, job.URL); err == nil {
			// The background observer is a *read-only* operation. No profile or
			// document is ever sent through this route.
			a.browsers.watch(id, job.URL, func(o browser.Observation) (int64, error) {
				result, e := browser.AsResult(job.URL, o)
				if e != nil {
					return 0, e
				}
				return a.saveInspection(id, result)
			})
		}
		if err != nil {
			log.Printf("browser start: %v", err)
			a.browsers.reportLaunchFailure(id, err)
		}
	case "observe":
		var obs browser.Observation
		obs, err = a.browsers.inspect(r.Context(), id)
		if err == nil {
			res, e := browser.AsResult(job.URL, obs)
			if e != nil {
				err = e
			} else {
				var savedID int64
				savedID, err = a.saveInspection(id, res)
				if err == nil {
					a.browsers.setState(id, "detected", "Form captured manually.", savedID, false)
				}
			}
		}
		if err != nil {
			log.Printf("inspection for job %d: %v", id, err)
			if errors.Is(err, chromium.ErrNoPageTargets) || errors.Is(err, chromium.ErrBrowserUnavailable) {
				a.browsers.interrupt(id, "The application browser or its tab was closed. Reopen the application; the selected resume remains in YourMove, but unsaved form data may be lost.")
			} else {
				a.browsers.setState(id, "attention", "Could not inspect this page. Return to the Ashby posting in Chromium, or use Reopen application. No data was uploaded.", 0, false)
			}
		}
	case "restart":
		if r.PostForm.Get("confirm_restart") != "yes" {
			http.Error(w, "Confirm that reopening may lose unsaved employer form changes", 400)
			return
		}
		// Never carry a previous file authorization into a restarted browser.
		// The selected job resume stays assigned but requires fresh consent.
		_ = a.browsers.close(id)
		if err = a.browsers.start(r.Context(), id, job.URL); err != nil {
			a.browsers.reportLaunchFailure(id, err)
			break
		}
		a.browsers.watch(id, job.URL, func(o browser.Observation) (int64, error) {
			result, e := browser.AsResult(job.URL, o)
			if e != nil {
				return 0, e
			}
			return a.saveInspection(id, result)
		})
	case "retry-resume":
		err = a.browsers.retryResumePreflight(id)
		if err != nil {
			a.browsers.setState(id, "attention", "The resume could not be safely retried. Review the live employer form or start a new authorized handoff.", 0, false)
		} else {
			a.browsers.setState(id, "watching", "Rechecking the current employer form without sending another file.", 0, false)
		}
	case "stop":
		if err = a.browsers.close(id); err != nil {
			http.Error(w, "Could not close inspection browser", 500)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10)+"/inspection", http.StatusSeeOther)
}

// startResumeHandoff pins and verifies the exact job-selected resume bytes before
// any browser interaction. Selecting the file may immediately send it to Ashby.
func (a *App) startResumeHandoff(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage == StageClosed || job.Stage == StageApplied || job.SubmissionCount > 0 {
		http.Error(w, "Active jobs only", 409)
		return
	}
	if !browser.OwnsAshbyURL(job.URL, job.URL) {
		http.Error(w, "Resume handoff currently supports Ashby applications", 422)
		return
	}
	if job.SelectedResumeID == 0 {
		http.Error(w, "Select a resume in this job before continuing", 409)
		return
	}
	resume, err := a.resumeByID(job.SelectedResumeID)
	if err != nil {
		http.Error(w, "Selected resume not available", 409)
		return
	}
	if r.PostForm.Get("confirm") != "yes" || r.PostForm.Get("resume_id") != strconv.FormatInt(resume.ID, 10) || r.PostForm.Get("resume_hash") != resume.SHA256 {
		http.Error(w, "Confirm the exact selected resume before allowing an employer upload", 409)
		return
	}
	if a.browsers.has(id) && a.browsers.state(id).ResumeStatus != "" {
		http.Error(w, "Current browser already has a resume handoff; finish it before changing files", 409)
		return
	}
	if filepath.Base(resume.StoredName) != resume.StoredName {
		http.Error(w, "Invalid stored document", 409)
		return
	}
	file, err := os.Open(filepath.Join(a.resumeDir, resume.StoredName))
	if err != nil {
		http.Error(w, "Resume file missing or has changed", 409)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxResumeSize+1))
	if err != nil || int64(len(data)) != resume.Size || len(data) > maxResumeSize {
		http.Error(w, "Resume file missing or has changed", 409)
		return
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != resume.SHA256 {
		http.Error(w, "Resume integrity check failed", 409)
		return
	}
	dir, err := os.MkdirTemp("", "yourmove-approved-resume-")
	if err != nil {
		http.Error(w, "Unable to prepare resume", 500)
		return
	}
	name := cleanResumeName(resume.Name)
	if name == "" || filepath.Ext(strings.ToLower(name)) != resume.Kind {
		os.RemoveAll(dir)
		http.Error(w, "Resume display name invalid", 409)
		return
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		os.RemoveAll(dir)
		http.Error(w, "Unable to prepare resume", 500)
		return
	}
	if err := a.browsers.start(r.Context(), id, job.URL); err != nil {
		os.RemoveAll(dir)
		a.browsers.reportLaunchFailure(id, err)
		http.Redirect(w, r, fmt.Sprintf("/jobs/%d/inspection", id), http.StatusSeeOther)
		return
	}
	if err := a.browsers.armResume(id, dir, path, name); err != nil {
		os.RemoveAll(dir)
		// Existing browser remains available for manual use. Never leave an
		// unowned staged document or attempt a second unauthorized upload.
		log.Printf("resume handoff could not be armed: %v", err)
		a.browsers.setState(id, "attention", "Resume handoff could not be started. Inspect the employer browser or close the session before selecting a new document.", 0, false)
		http.Redirect(w, r, fmt.Sprintf("/jobs/%d/inspection", id), http.StatusSeeOther)
		return
	}
	a.browsers.watch(id, job.URL, func(o browser.Observation) (int64, error) {
		result, e := browser.AsResult(job.URL, o)
		if e != nil {
			return 0, e
		}
		return a.saveInspection(id, result)
	})
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d/inspection", id), http.StatusSeeOther)
}
