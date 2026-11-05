package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/discovery"
)

// Discovery is explicitly triggered by preparation, never by a question GET.
// It uses an independent read-only preview, not the applicant session host.
func (a *App) writingDiscoverAPI(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	a.earlyDiscoveryMu.Lock()
	defer a.earlyDiscoveryMu.Unlock()
	job, err := a.jobByID(id)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeJSON(w, http.StatusOK, map[string]string{"status": "inactive"})
		return
	}
	state := a.browsers.state(id)
	if state.Active && state.Destination == job.URL && state.LatestID > 0 {
		if live, e := a.inspectionByID(state.LatestID); e == nil && live.JobID == id && live.Source == "browser" && browser.OwnsPostingURL(job.URL, live.ApplicationURL) {
			writeResumeJSON(w, http.StatusOK, map[string]string{"status": "available"})
			return
		}
	}
	source := ""
	if discovery.SupportsPublicQuestions(job.URL) {
		source = "public_api"
	} else if browser.PostingPlatform(job.URL) == "ashby" && browser.OwnsPostingURL(job.URL, job.URL) {
		source = "preview"
	}
	if source == "" {
		writeResumeJSON(w, http.StatusOK, map[string]string{"status": "unsupported"})
		return
	}
	_, err = a.latestValidInspectionForPosting(id, source, job.URL)
	if err == nil {
		writeResumeJSON(w, http.StatusOK, map[string]string{"status": "available"})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read saved questions")
		return
	}
	if source == "preview" {
		fetch := a.previewApplication
		if fetch == nil {
			fetch = chromium.Preview
		}
		result, fetchErr := fetch(r.Context(), job.URL)
		if fetchErr != nil {
			log.Printf("question preview for job %d: %v", id, fetchErr)
			writeResumeJSON(w, http.StatusOK, map[string]string{"status": "failed"})
			return
		}
		if result.Platform != "ashby" || result.Source != "preview" || result.Completeness != "rendered_preview_partial" || !browser.OwnsPostingURL(job.URL, result.ApplicationURL) || len(result.Fields) > 200 || len(result.Steps) > 30 {
			writeResumeJSON(w, http.StatusOK, map[string]string{"status": "failed"})
			return
		}
		current, e := a.jobByID(id)
		if e != nil || current.URL != job.URL || current.Stage == StageApplied || current.Stage == StageClosed || current.SubmissionCount > 0 {
			writeResumeError(w, http.StatusConflict, "The posting changed during discovery")
			return
		}
		if _, e = a.saveInspection(id, result); e != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save questions")
			return
		}
	} else {
		fetch := a.inspectApplication
		if fetch == nil {
			registry := discovery.New()
			fetch = registry.Inspect
		}
		result, fetchErr := fetch(r.Context(), job.URL)
		if fetchErr != nil {
			log.Printf("public question discovery for job %d: %v", id, fetchErr)
			writeResumeJSON(w, http.StatusOK, map[string]string{"status": "failed"})
			return
		}
		current, e := a.jobByID(id)
		if e != nil || current.URL != job.URL || current.Stage == StageApplied || current.Stage == StageClosed || current.SubmissionCount > 0 {
			writeResumeError(w, http.StatusConflict, "The posting changed during discovery")
			return
		}
		if result.Source != source || result.Platform != "greenhouse" || !browser.OwnsPostingURL(job.URL, result.ApplicationURL) {
			writeResumeJSON(w, http.StatusOK, map[string]string{"status": "failed"})
			return
		}
		if _, e = a.saveInspection(id, result); e != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save questions")
			return
		}
	}
	writeResumeJSON(w, http.StatusOK, map[string]string{"status": "available"})
}
