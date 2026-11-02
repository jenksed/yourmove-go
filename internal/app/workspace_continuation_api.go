// YM-004C Employer continuation JSON transport.
//
// Coherent internal/app implementation (no feature-package extraction): every
// contract below reuses the existing Go-owned browser, transfer, and
// submission decisions. This file adds transport only:
//
//   - GET  /api/jobs/{id}/continuation  read-only transfer scope, token-free
//   - POST /api/jobs/{id}/browser       JSON twin of browserOperation
//     (start/observe/restart/retry-resume/stop), same launch/read-only rules
//   - POST /api/jobs/{id}/continue      JSON twin of continueApplication,
//     composing the same independently guarded commands with the same
//     destination/identity/revision/hash checks
//   - POST /api/jobs/{id}/submission-review  explicit submission-review
//     token mint, bound to the current selection (legacy GET /jobs/{id}
//     issued this as a side effect; ordinary React reads never mint)
//   - POST /api/jobs/{id}/submit         JSON twin of submitJob
//   - GET  /api/jobs/{id}/submissions    durable submission history + next job
//   - POST /api/jobs/{id}/next           JSON twin of nextApplication
//
// React may present scope but never becomes its authority: every command
// revalidates the exact submitted material when it executes. No résumé
// bytes, credentials, paths, or browser internals enter these responses.

package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
)

type continuationAnswerJSON struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Source string `json:"source"`
}

type continuationProfileJSON struct {
	Approved  bool   `json:"approved"`
	Revision  int64  `json:"revision"`
	FullName  string `json:"fullName"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type continuationLetterJSON struct {
	ID          int64  `json:"id"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	ContentHash string `json:"contentHash"`
	EditedAt    string `json:"editedAt"`
	HasBody     bool   `json:"hasBody"`
}

type continuationOtherJSON struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Stage string `json:"stage"`
}

type continuationSavedJSON struct {
	CheckedAt  string `json:"checkedAt"`
	FieldCount int    `json:"fieldCount"`
	URL        string `json:"url"`
}

type continuationSubmissionJSON struct {
	ID           int64  `json:"id"`
	SubmittedAt  string `json:"submittedAt"`
	ResumeName   string `json:"resumeName"`
	ResumeSHA256 string `json:"resumeSha256"`
	Notes        string `json:"notes"`
}

// buildContinuation assembles the read-only transfer scope. It never mints a
// submission-review token and never writes durable state.
func (a *App) buildContinuation(jobID int64) (map[string]any, error) {
	job, err := a.jobByID(jobID)
	if err != nil {
		return nil, err
	}
	resumes, err := a.allResumes()
	if err != nil {
		return nil, err
	}
	var selected *Resume
	for i := range resumes {
		if resumes[i].ID == job.SelectedResumeID {
			selected = &resumes[i]
			break
		}
	}
	resumeValid := false
	resumeReason := ""
	if selected != nil && selected.ArchivedAt == "" {
		if _, fileErr := a.readResumeBytes(*selected); fileErr == nil {
			resumeValid = true
		} else {
			resumeReason = "The selected resume file is missing or changed. Select and save an available resume, or import it again in Documents."
		}
	} else if selected != nil {
		resumeReason = "This archived version remains for history. Select its current version for transfer."
	}
	handoff := assessResumeHandoff(job, selected)
	if resumeReason != "" && !handoff.Allowed {
		handoff.Reason = resumeReason
	}
	profile, err := a.candidateProfile()
	if err != nil {
		return nil, err
	}
	// The explicit Fill action authorizes the displayed profile snapshot, without
	// changing its separate approval state for other workflows.
	profileForFill := profile
	profileForFill.Approved = true
	live, err := a.prepareQuestions(job, profileForFill)
	if err != nil {
		return nil, err
	}
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		return nil, err
	}
	letter, err := a.selectedCoverLetter(jobID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	state := a.browsers.state(jobID)
	prepWorkspace := jobWorkspace{
		Job: job, BrowserState: state, Handoff: handoff,
		Questions: live, EarlyQuestions: early, Letter: letter, Profile: profileForFill,
	}
	// A failed launch with no transfer can be retried by the React fill action.
	// Retain interrupted status for display; never re-arm an attempted upload.
	if !state.Active && state.Phase == "interrupted" && state.ResumeStatus == "" {
		prepWorkspace.BrowserState.Phase = ""
	}
	preparation := summarizePreparation(prepWorkspace)
	snapshot := chooseApplicationQuestionSnapshot(job, state, live, early)
	submissions, err := a.jobSubmissions(jobID)
	if err != nil {
		return nil, err
	}
	pending := make([]continuationAnswerJSON, 0, len(preparation.PendingAnswers))
	for _, q := range preparation.PendingAnswers {
		pending = append(pending, continuationAnswerJSON{Key: q.Key, Label: q.Field.Label, Value: q.Value, Source: q.Source})
	}
	var otherJSON *continuationOtherJSON
	if otherID := a.browsers.otherApplication(jobID); otherID > 0 {
		if other, loadErr := a.jobByID(otherID); loadErr == nil {
			title := other.Title
			if title == "" {
				title = other.Host
			}
			otherJSON = &continuationOtherJSON{ID: other.ID, Title: title, URL: other.URL, Stage: other.Stage}
		}
	}
	var savedJSON *continuationSavedJSON
	if !state.Active {
		if saved, loadErr := a.latestInspectionBySource(jobID, "browser"); loadErr == nil && browser.OwnsPostingURL(job.URL, saved.ApplicationURL) {
			savedJSON = &continuationSavedJSON{CheckedAt: saved.CheckedAt, FieldCount: len(saved.Fields), URL: saved.ApplicationURL}
		}
	}
	submissionJSON := make([]continuationSubmissionJSON, 0, len(submissions))
	for _, s := range submissions {
		submissionJSON = append(submissionJSON, continuationSubmissionJSON{ID: s.ID, SubmittedAt: s.SubmittedAt, ResumeName: s.ResumeName, ResumeSHA256: s.ResumeSHA256, Notes: s.Notes})
	}
	fullName := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
	editingAllowed := job.Stage != StageApplied && job.Stage != StageClosed && job.SubmissionCount == 0
	var letterJSON *continuationLetterJSON
	if letter.ID > 0 {
		letterJSON = &continuationLetterJSON{ID: letter.ID, Provider: letter.Provider, Model: letter.Model, ContentHash: letter.ContentHash(), EditedAt: letter.EditedAt, HasBody: true}
	}
	var resumeJSON *map[string]any
	if selected != nil {
		resumeJSON = &map[string]any{
			"id": selected.ID, "name": selected.Name, "kind": selected.Kind,
			"version": selected.Version, "sha256": selected.SHA256, "valid": resumeValid,
		}
	}
	return map[string]any{
		"destination":    job.URL,
		"ownsPosting":    browser.OwnsPostingURL(job.URL, job.URL),
		"desktop":        a.openPosting != nil,
		"applied":        job.Stage == StageApplied || job.SubmissionCount > 0,
		"closed":         job.Stage == StageClosed,
		"editingAllowed": editingAllowed,
		"preparation": map[string]any{
			"title": preparation.Title, "detail": preparation.Detail,
			"recovery": preparation.Recovery, "action": preparation.Action,
			"external":      preparation.OpenExternally,
			"includeResume": preparation.IncludeResume, "includeLetter": preparation.IncludeLetter,
			"includeAnswers":   preparation.IncludeAnswers,
			"answerSummary":    preparation.AnswerSummary,
			"answerReviewHash": preparation.AnswerReviewHash(),
		},
		"handoff":      map[string]any{"allowed": handoff.Allowed, "reason": handoff.Reason},
		"browser":      state,
		"otherBrowser": otherJSON,
		"savedBrowser": savedJSON,
		"resume":       resumeJSON,
		"profile": continuationProfileJSON{
			Approved: profile.Approved, Revision: profile.Revision,
			FullName: fullName, FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email, Phone: profile.Phone,
		},
		"letter":                 letterJSON,
		"pendingAnswers":         pending,
		"previousCount":          len(live.Previous),
		"awaitingReconciliation": len(early.AwaitingReconciliation()),
		"questionsAvailable":     snapshot.Available,
		"submissions":            submissionJSON,
		"submissionCount":        len(submissions),
		"legacyHref":             workspaceLegacyHref(jobID),
	}, nil
}

// GET /api/jobs/{id}/continuation is strictly read-only.
func (a *App) continuationAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	payload, err := a.buildContinuation(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		log.Printf("load continuation %d: %v", jobID, err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to load employer continuation")
		return
	}
	writeResumeJSON(w, http.StatusOK, payload)
}

type browserActionInput struct {
	Action         string `json:"action"`
	ConfirmRestart string `json:"confirm_restart"`
}

// POST /api/jobs/{id}/browser runs one read-only or lifecycle operation.
// Opening a browser sends no saved material; transfers stay separately
// authorized. The fresh state is always returned so React can render it.
func (a *App) continuationBrowserAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input browserActionInput
	if !decodeResumeJSON(w, r, 2<<10, &input) {
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load opportunity")
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeError(w, http.StatusConflict, "Inspection is limited to active jobs")
		return
	}
	if !browser.OwnsPostingURL(job.URL, job.URL) {
		writeResumeError(w, http.StatusUnprocessableEntity, "Managed inspection supports saved Ashby, Greenhouse, and Lever postings")
		return
	}
	fail := func(message string) {
		writeResumeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": message, "browser": a.browsers.state(jobID)})
	}
	switch input.Action {
	case "start":
		err = a.browsers.start(r.Context(), jobID, job.URL)
		if err == nil {
			a.browsers.watch(jobID, job.URL, func(o browser.Observation) (int64, error) {
				result, e := browser.AsResult(job.URL, o)
				if e != nil {
					return 0, e
				}
				return a.saveInspection(jobID, result)
			})
		}
		if err != nil {
			log.Printf("browser start: %v", err)
			a.browsers.reportLaunchFailure(jobID, err)
			fail("The employer browser could not be opened. " + a.browsers.state(jobID).Message)
			return
		}
	case "observe":
		obs, err := a.browsers.inspect(r.Context(), jobID)
		if err == nil {
			res, e := browser.AsResult(job.URL, obs)
			if e != nil {
				err = e
			} else {
				var savedID int64
				savedID, err = a.saveInspection(jobID, res)
				if err == nil {
					a.browsers.setState(jobID, "detected", "Form captured manually.", savedID, false)
				}
			}
		}
		if err != nil {
			log.Printf("inspection for job %d: %v", jobID, err)
			a.browsers.interrupt(jobID, "The application browser or its tab was closed. Reopen the application; the selected resume remains in YourMove, but unsaved form data may be lost.")
			fail("Could not inspect this page. Return to the employer posting in Chromium, or use Reopen application. No data was uploaded.")
			return
		}
	case "restart":
		if input.ConfirmRestart != "yes" {
			writeResumeError(w, http.StatusBadRequest, "Confirm that reopening may lose unsaved employer form changes")
			return
		}
		// Never carry a previous file authorization into a restarted browser.
		_ = a.browsers.close(jobID)
		if err := a.browsers.start(r.Context(), jobID, job.URL); err != nil {
			a.browsers.reportLaunchFailure(jobID, err)
			fail("The employer browser could not be reopened. " + a.browsers.state(jobID).Message)
			return
		}
		a.browsers.watch(jobID, job.URL, func(o browser.Observation) (int64, error) {
			result, e := browser.AsResult(job.URL, o)
			if e != nil {
				return 0, e
			}
			return a.saveInspection(jobID, result)
		})
	case "retry-resume":
		if err := a.browsers.retryResumePreflight(jobID); err != nil {
			a.browsers.setState(jobID, "attention", "The resume could not be safely retried. Review the live employer form or start a new authorized handoff.", 0, false)
			fail("The resume could not be safely retried. Review the live employer form or start a new authorized handoff.")
			return
		}
		a.browsers.setState(jobID, "watching", "Rechecking the current employer form without sending another file.", 0, false)
	case "stop":
		if err := a.browsers.close(jobID); err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Could not close inspection browser")
			return
		}
	default:
		writeResumeError(w, http.StatusNotFound, "Unknown browser action")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"ok": true, "browser": a.browsers.state(jobID)})
}

type continueInput struct {
	Destination      string `json:"destination"`
	IncludeResume    bool   `json:"include_resume"`
	ResumeID         string `json:"resume_id"`
	ResumeHash       string `json:"resume_hash"`
	TransferContacts bool   `json:"transfer_contacts"`
	ProfileRevision  string `json:"profile_revision"`
	IncludeLetter    bool   `json:"include_letter"`
	DraftID          string `json:"draft_id"`
	BodyHash         string `json:"body_hash"`
	Mode             string `json:"mode"`
	IncludeAnswers   bool   `json:"include_answers"`
	ReviewHash       string `json:"review_hash"`
}

type continueItemResult struct {
	Item   string `json:"item"`
	OK     bool   `json:"ok"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func continueForm(input continueInput) url.Values {
	form := url.Values{}
	form.Set("destination", input.Destination)
	if input.IncludeResume {
		form.Set("include_resume", "yes")
	}
	form.Set("resume_id", input.ResumeID)
	form.Set("resume_hash", input.ResumeHash)
	if input.TransferContacts {
		form.Set("transfer_contacts", "yes")
	}
	form.Set("profile_revision", input.ProfileRevision)
	if input.IncludeLetter {
		form.Set("include_letter", "yes")
	}
	form.Set("draft_id", input.DraftID)
	form.Set("require_selected_letter", "yes")
	form.Set("body_hash", input.BodyHash)
	form.Set("mode", input.Mode)
	if input.IncludeAnswers {
		form.Set("include_answers", "yes")
	}
	form.Set("review_hash", input.ReviewHash)
	return form
}

// POST /api/jobs/{id}/continue composes the same independently guarded
// commands as the legacy form. Unselected material is never transferred; a
// failed item does not roll back or prevent another eligible item.
func (a *App) continuationContinueAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input continueInput
	if !decodeResumeJSON(w, r, 8<<10, &input) {
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load opportunity")
		return
	}
	if input.Destination != job.URL {
		writeResumeError(w, http.StatusConflict, "The employer destination changed. Review Prepare again.")
		return
	}
	// React's Fill action carries the displayed profile revision and résumé
	// identity even for subsequent material. Revalidate both before any effect.
	if input.ProfileRevision != "" {
		profile, profileErr := a.candidateProfile()
		if profileErr != nil || input.ProfileRevision != strconv.FormatInt(profile.Revision, 10) {
			writeResumeError(w, http.StatusConflict, "Candidate Profile changed. Review it before filling.")
			return
		}
		resume, resumeErr := a.resumeByID(job.SelectedResumeID)
		if resumeErr != nil || input.ResumeID != strconv.FormatInt(resume.ID, 10) || input.ResumeHash != resume.SHA256 || resume.ArchivedAt != "" {
			writeResumeError(w, http.StatusConflict, "Select an available résumé before filling.")
			return
		}
		if _, err := a.readResumeBytes(resume); err != nil {
			writeResumeError(w, http.StatusConflict, "The selected résumé is missing or changed. Select it again before filling.")
			return
		}
	}
	if !input.IncludeResume && !input.IncludeLetter && !input.IncludeAnswers && !input.TransferContacts {
		writeResumeError(w, http.StatusUnprocessableEntity, "No transfer material was selected. Review Prepare again.")
		return
	}
	form := continueForm(input)
	cmd := &http.Request{Method: http.MethodPost, PostForm: form}
	cmd = cmd.WithContext(r.Context())
	results := make([]continueItemResult, 0, 3)
	run := func(item string, included bool, fn func(*http.Request, int64) (int, string)) {
		if !included {
			return
		}
		status, detail := fn(cmd, jobID)
		results = append(results, continueItemResult{Item: item, OK: status == http.StatusSeeOther, Status: status, Detail: detail})
	}
	run("resume", input.IncludeResume, func(r *http.Request, id int64) (int, string) { return a.resumeHandoffForAction(r, id, true) })
	if input.TransferContacts && !input.IncludeResume {
		status, detail := http.StatusConflict, "The selected résumé or Candidate Profile changed. Review them before filling."
		resume, resumeErr := a.resumeByID(job.SelectedResumeID)
		profile, profileErr := a.candidateProfile()
		if resumeErr == nil && profileErr == nil && input.ResumeID == strconv.FormatInt(resume.ID, 10) && input.ResumeHash == resume.SHA256 && input.ProfileRevision == strconv.FormatInt(profile.Revision, 10) {
			if _, readErr := a.readResumeBytes(resume); readErr == nil {
				contacts := &browser.ContactValues{Revision: profile.Revision, FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email, Phone: profile.Phone}
				current := func() bool { p, e := a.candidateProfile(); return e == nil && p.Revision == profile.Revision }
				if armErr := a.browsers.armExistingContacts(jobID, job.URL, resume.ID, resume.SHA256, contacts, current); armErr == nil {
					status, detail = http.StatusSeeOther, "Contact filling started"
				} else {
					detail = armErr.Error()
				}
			}
		}
		results = append(results, continueItemResult{Item: "contacts", OK: status == http.StatusSeeOther, Status: status, Detail: detail})
	}
	run("letter", input.IncludeLetter, a.coverLetterCommand)
	run("answers", input.IncludeAnswers, func(r *http.Request, id int64) (int, string) {
		return a.answerTransferForAction(r, id, input.ProfileRevision != "")
	})
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"items": results, "browser": a.browsers.state(jobID),
	})
}

// POST /api/jobs/{id}/submission-review mints one review token bound to the
// current selection. This explicit operation replaces the legacy page-load
// side effect; ordinary reads never mint tokens.
func (a *App) continuationReviewAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load opportunity")
		return
	}
	token, err := newSubmissionToken()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to prepare submission confirmation")
		return
	}
	resumeHash := ""
	if job.SelectedResumeID > 0 {
		if resume, resumeErr := a.resumeByID(job.SelectedResumeID); resumeErr == nil {
			resumeHash = resume.SHA256
		} else if !errors.Is(resumeErr, sql.ErrNoRows) {
			writeResumeError(w, http.StatusInternalServerError, "Unable to prepare submission confirmation")
			return
		}
	}
	if err := a.recordSubmissionReview(jobID, token, job.SelectedResumeID, resumeHash); err != nil {
		if errors.Is(err, ErrSubmissionReviewStale) {
			writeResumeError(w, http.StatusConflict, "The selected resume changed while preparing. Review it again.")
			return
		}
		log.Printf("record submission review: %v", err)
		writeResumeError(w, http.StatusInternalServerError, "Unable to prepare submission confirmation")
		return
	}
	var resumeName string
	if job.SelectedResumeID > 0 {
		if resume, resumeErr := a.resumeByID(job.SelectedResumeID); resumeErr == nil {
			resumeName = resume.Name
		}
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"token": token, "jobId": jobID,
		"resumeId": job.SelectedResumeID, "resumeName": resumeName, "resumeHash": resumeHash,
	})
}

type submitInput struct {
	Confirm         string `json:"confirm"`
	Notes           string `json:"notes"`
	SubmissionToken string `json:"submission_token"`
}

// POST /api/jobs/{id}/submit records the applicant's manual submission
// declaration locally. It never submits to an employer. Browser opening,
// form population, or transfer success never mark the job Applied.
func (a *App) continuationSubmitAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	var input submitInput
	if !decodeResumeJSON(w, r, 4<<10, &input) {
		return
	}
	if input.Confirm != "yes" {
		writeResumeError(w, http.StatusBadRequest, "Confirm submission completed on employer website")
		return
	}
	if len(strings.TrimSpace(input.Notes)) > 1000 {
		writeResumeError(w, http.StatusBadRequest, "Notes too long")
		return
	}
	if err := a.recordSubmission(jobID, strings.TrimSpace(input.Notes), input.SubmissionToken); err != nil {
		if errors.Is(err, ErrSubmissionReviewStale) {
			writeResumeJSON(w, http.StatusConflict, map[string]string{
				"code":    "stale",
				"message": "Your confirmation was not recorded. This review expired, or the selected resume changed or is unavailable. Review the current resume, then confirm again.",
			})
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusNotFound, "Unknown job")
			return
		}
		log.Printf("record application: %v", err)
		writeResumeJSON(w, http.StatusInternalServerError, map[string]string{
			"code":    "failed",
			"message": "YourMove could not confirm whether this recording completed. Check submission history before retrying. If no record appears, return to Prepare and try again.",
		})
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"recorded": true, "jobId": jobID})
}

// GET /api/jobs/{id}/submissions returns the durable per-job history.
func (a *App) continuationSubmissionsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if _, err := a.jobByID(jobID); errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	} else if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load opportunity")
		return
	}
	submissions, err := a.jobSubmissions(jobID)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load applications")
		return
	}
	out := make([]continuationSubmissionJSON, 0, len(submissions))
	for _, s := range submissions {
		out = append(out, continuationSubmissionJSON{ID: s.ID, SubmittedAt: s.SubmittedAt, ResumeName: s.ResumeName, ResumeSHA256: s.ResumeSHA256, Notes: s.Notes})
	}
	var next *NextAction
	if jobs, loadErr := a.allJobs(); loadErr == nil {
		remaining := make([]Job, 0, len(jobs))
		for _, job := range jobs {
			if job.ID != jobID {
				remaining = append(remaining, job)
			}
		}
		next = recommendNext(remaining)
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"submissions": out, "count": len(out), "next": next})
}

// POST /api/jobs/{id}/next closes an authorized managed browser and recommends another opportunity.
func (a *App) continuationNextAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if _, err := a.jobByID(jobID); errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	} else if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load opportunity")
		return
	}
	jobs, err := a.allJobs()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to load the next opportunity")
		return
	}
	if err := a.browsers.close(jobID, r.URL.Query().Get("close_browser") == "yes"); err != nil {
		if errors.Is(err, errBrowserCloseNeedsConsent) {
			writeResumeError(w, http.StatusConflict, "The employer browser is open or opening. Closing it may lose unsaved form work.")
			return
		}
		writeResumeError(w, http.StatusInternalServerError, "Could not close the employer browser. Return to the application before moving on.")
		return
	}
	remaining := make([]Job, 0, len(jobs))
	for _, job := range jobs {
		if job.ID != jobID {
			remaining = append(remaining, job)
		}
	}
	target := "/jobs"
	if next := recommendNext(remaining); next != nil {
		target = "/jobs/" + strconv.FormatInt(next.JobID, 10) + "#prepare"
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"target": target, "next": recommendNext(remaining)})
}
