package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

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
	if !browser.OwnsPostingURL(job.URL, job.URL) {
		http.Error(w, "Managed inspection supports saved Ashby, Greenhouse, and Lever postings", 422)
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
				a.browsers.setState(id, "attention", "Could not inspect this page. Return to the employer posting in Chromium, or use Reopen application. No data was uploaded.", 0, false)
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
	target := "/jobs/" + strconv.FormatInt(id, 10) + "/inspection"
	if r.PostForm.Get("return_to") == "prepare" {
		target = "/jobs/" + strconv.FormatInt(id, 10) + "#prepare"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// startResumeHandoff pins and verifies the exact job-selected resume bytes before
// any browser interaction. Selecting the file may immediately send it to the employer.
func (a *App) startResumeHandoff(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	status, detail := a.resumeHandoffCommand(r, id)
	writeTransferResult(w, r, status, detail)
}

// resumeHandoffCommand validates the exact submitted material before arming its existing operation.
func (a *App) resumeHandoffCommand(r *http.Request, id int64) (int, string) {
	returnURL := fmt.Sprintf("/jobs/%d/inspection", id)
	if r.PostForm.Get("return_to") == "prepare" {
		returnURL = fmt.Sprintf("/jobs/%d#prepare", id)
	}
	job, err := a.jobByID(id)
	if err != nil {
		return http.StatusNotFound, "Opportunity or material unavailable"
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
			!browser.OwnsPostingURL(job.URL, job.URL) {
			status = http.StatusUnprocessableEntity
		}
		return status, eligibility.Reason
	}
	if r.PostForm.Get("destination") != job.URL || r.PostForm.Get("resume_id") != strconv.FormatInt(resume.ID, 10) || r.PostForm.Get("resume_hash") != resume.SHA256 {
		return 409, "Confirm the exact selected resume before allowing an employer upload"
	}
	state := a.browsers.state(id)
	if state.Active && state.Destination != job.URL {
		return http.StatusConflict, "The managed browser belongs to an earlier destination. Inspect it before reopening this application."
	}
	if state.Active && state.ResumeStatus != "" {
		return 409, "Current browser already has a resume handoff; finish it before changing files"
	}
	var contacts *browser.ContactValues
	if r.PostForm.Get("transfer_contacts") == "yes" {
		profile, profileErr := a.candidateProfile()
		if profileErr != nil {
			return 500, "Unable to read candidate information"
		}
		if !profile.Approved || r.PostForm.Get("profile_revision") != strconv.FormatInt(profile.Revision, 10) {
			return http.StatusConflict, "Candidate information changed or is not approved. Review it again before transferring."
		}
		contacts = &browser.ContactValues{Revision: profile.Revision, FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email, Phone: profile.Phone}
	}
	data, err := a.readResumeBytes(*resume)
	if err != nil {
		return 409, "Resume file missing or has changed"
	}
	dir, err := os.MkdirTemp("", "yourmove-approved-resume-")
	if err != nil {
		return 500, "Unable to prepare resume"
	}
	if resume.Kind != ".pdf" && resume.Kind != ".docx" {
		os.RemoveAll(dir)
		return 409, "Resume file type invalid"
	}
	profile, err := a.candidateProfile()
	if err != nil {
		os.RemoveAll(dir)
		return 500, "Unable to read candidate information"
	}
	name := attachmentFilename(profile, job, "Resume", resume.Kind)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		os.RemoveAll(dir)
		return 500, "Unable to prepare resume"
	}
	if err := a.browsers.start(r.Context(), id, job.URL); err != nil {
		os.RemoveAll(dir)
		a.browsers.reportLaunchFailure(id, err)
		return http.StatusSeeOther, returnURL
	}
	if err := a.browsers.armResume(id, dir, path, name, contacts); err != nil {
		os.RemoveAll(dir)
		// Existing browser remains available for manual use. Never leave an
		// unowned staged document or attempt a second unauthorized upload.
		log.Printf("resume handoff could not be armed: %v", err)
		a.browsers.setState(id, "attention", "Resume handoff could not be started. Inspect the employer browser or close the session before selecting a new document.", 0, false)
		return http.StatusSeeOther, returnURL
	}
	a.browsers.watch(id, job.URL, func(o browser.Observation) (int64, error) {
		result, e := browser.AsResult(job.URL, o)
		if e != nil {
			return 0, e
		}
		return a.saveInspection(id, result)
	})
	return http.StatusSeeOther, returnURL
}
