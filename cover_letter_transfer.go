package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"example.com/yourmove-lite/internal/application/browser"
	"example.com/yourmove-lite/internal/application/browser/chromium"
)

// A letter approval is separate from resume/submission authority and belongs
// only to the current managed session. No user text enters status responses.
type coverLetterTransfer struct {
	draftID                                        int64
	dir, path, body, digest, mode, status, message string
	destination                                    string
	current                                        func() bool
}
type coverLetterSession interface {
	AttachCoverLetter(context.Context, string, string) (chromium.AttachmentResult, error)
	FillCoverLetter(context.Context, string) (chromium.AttachmentResult, error)
}

func (a *App) transferCoverLetter(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	status, detail := a.coverLetterCommand(r, id)
	writeTransferResult(w, r, status, detail)
}

// coverLetterCommand validates the exact submitted material before arming its existing operation.
func (a *App) coverLetterCommand(r *http.Request, id int64) (int, string) {
	job, err := a.jobByID(id)
	if err != nil {
		return http.StatusNotFound, "Opportunity or material unavailable"
	}
	if job.Stage == StageClosed || job.Stage == StageApplied || job.SubmissionCount > 0 || !browser.OwnsPostingURL(job.URL, job.URL) {
		return 409, "This opportunity is not eligible for transfer"
	}
	draftID, _ := strconv.ParseInt(r.PostForm.Get("draft_id"), 10, 64)
	draft, err := a.coverLetterByID(id, draftID)
	if err != nil {
		return http.StatusNotFound, "Opportunity or material unavailable"
	}
	if len(draft.Body) < 30 || len(draft.Body) > maxLetterBytes || !utf8.ValidString(draft.Body) || strings.ContainsRune(draft.Body, 0) {
		return 409, "The saved letter is invalid. Review and save its text before transfer."
	}
	if r.PostForm.Get("body_hash") != draft.ContentHash() || r.PostForm.Get("destination") != job.URL {
		return 409, "The saved letter or destination changed. Review it again before transfer."
	}
	mode := r.PostForm.Get("mode")
	if mode != "file" && mode != "text" {
		return 422, "Choose a cover-letter destination"
	}
	up := &coverLetterTransfer{draftID: draft.ID, destination: job.URL, body: draft.Body, mode: mode, status: "waiting", message: "Waiting for the intended cover-letter control in the current employer browser."}
	up.current = func() bool {
		latest, e := a.coverLetterByID(id, draftID)
		target, je := a.jobByID(id)
		return e == nil && je == nil && latest.ContentHash() == draft.ContentHash() && target.URL == job.URL && target.Stage != StageApplied && target.Stage != StageClosed && target.SubmissionCount == 0
	}
	if mode == "file" {
		profile, err := a.candidateProfile()
		if err != nil {
			return 500, "Unable to read candidate information"
		}
		data, err := coverLetterPDF(draft.Body)
		if err != nil {
			return 422, err.Error()
		}
		up.digest = fmt.Sprintf("%x", sha256.Sum256(data))
		up.dir, err = os.MkdirTemp("", "yourmove-approved-letter-")
		if err != nil {
			return 500, "Unable to prepare the letter"
		}
		up.path = filepath.Join(up.dir, attachmentFilename(profile, job, "Cover_Letter", ".pdf"))
		if err = os.WriteFile(up.path, data, 0600); err != nil {
			os.RemoveAll(up.dir)
			return 500, "Unable to prepare the letter"
		}
	}
	if err = a.browsers.armCoverLetter(id, up); err != nil {
		os.RemoveAll(up.dir)
		return 409, err.Error()
	}
	// The existing watcher owns mutation and cancellation. This never starts or
	// replaces a browser and never arms the resume operation again.
	return http.StatusSeeOther, coverDraftNoticeURL(id, draft.ID, "transferring")
}

func (h *sessionHost) armCoverLetter(jobID int64, up *coverLetterTransfer) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[jobID] == nil || h.cancels[jobID] == nil {
		return errors.New("Open this application's managed browser before transferring the letter")
	}
	if h.states[jobID].Destination != up.destination {
		return errors.New("The destination changed. Reopen and review the correct application before transfer")
	}
	if h.letters == nil {
		h.letters = map[int64]*coverLetterTransfer{}
	}
	if old := h.letters[jobID]; old != nil && old.status != "waiting" && old.status != "review" {
		return errors.New("The cover-letter transfer was already authorized. Review its result in Chromium; no duplicate transfer was started")
	}
	if old := h.letters[jobID]; old != nil {
		os.RemoveAll(old.dir)
	}
	h.letters[jobID] = up
	return nil
}
func (h *sessionHost) tryCoverLetter(ctx context.Context, jobID int64) {
	if ctx.Err() != nil {
		return
	}
	h.mu.Lock()
	up, s := h.letters[jobID], h.sessions[jobID]
	if up == nil || s == nil || up.status != "waiting" {
		h.mu.Unlock()
		return
	}
	up.status = "checking"
	h.mu.Unlock()
	stale := up.current != nil && !up.current()
	filler, ok := s.(coverLetterSession)
	var result chromium.AttachmentResult
	var err error
	if stale {
		err = errors.New("saved letter or destination changed after approval")
	} else if !ok {
		err = errors.New("this browser cannot transfer cover letters")
	} else if up.mode == "text" {
		result, err = filler.FillCoverLetter(ctx, up.body)
	} else {
		result, err = filler.AttachCoverLetter(ctx, up.path, up.digest)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.letters[jobID] != up {
		return
	}
	switch {
	case errors.Is(err, chromium.ErrNoCoverInput):
		up.status = "waiting"
	case stale:
		up.status = "review"
		up.message = "The saved letter or destination changed before transfer. Review and authorize the current draft. No letter was transferred."
	case errors.Is(err, chromium.ErrDocumentFormat):
		up.status = "review"
		up.message = "The employer's cover-letter control does not accept PDF. Use its supported text control, or attach an accepted document manually. No file was selected."
	case errors.Is(err, chromium.ErrExistingDocument):
		up.status = "review"
		up.message = "Existing employer attachment preserved. Review it in Chromium before changing documents."
	case errors.Is(err, chromium.ErrAmbiguousCoverInput):
		up.status = "review"
		up.message = "No unique cover-letter upload control. Complete the attachment manually in Chromium."
	case err != nil:
		var preflight *chromium.PreflightError
		if errors.As(err, &preflight) {
			up.status = "review"
			up.message = "The cover-letter control could not be checked. No file was selected; review before authorizing another attempt."
			break
		}
		up.status = "attention"
		up.message = "Letter transfer could not be verified. Review the cover-letter control in Chromium; it will not be retried automatically."
	case result.Selected && result.Verified:
		up.status = "complete"
		up.message = result.Detail
	default:
		up.status = "attention"
		if !result.Selected {
			up.status = "review"
		}
		up.message = result.Detail
	}
	if up.status != "waiting" {
		up.body = ""
	}
}
