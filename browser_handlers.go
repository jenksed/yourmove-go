package main

import (
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

	browser "example.com/yourmove-lite/internal/application/browser"
	"example.com/yourmove-lite/internal/application/browser/chromium"
)

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
	var resume *Resume
	if job.SelectedResumeID > 0 {
		saved, readErr := a.resumeByID(job.SelectedResumeID)
		if readErr == nil {
			resume = &saved
		}
	}

	eligibility := assessResumeHandoff(job, resume)
	if !eligibility.Allowed {
		status := http.StatusConflict
		// Preserve the existing 422 response for unsupported providers.
		if job.Stage != StageClosed && job.Stage != StageApplied &&
			job.SubmissionCount == 0 &&
			!browser.OwnsAshbyURL(job.URL, job.URL) {
			status = http.StatusUnprocessableEntity
		}
		http.Error(w, eligibility.Reason, status)
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
