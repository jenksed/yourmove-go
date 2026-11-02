package app

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
	"github.com/jenksed/yourmove-go/internal/application/browser/playwright"
	"github.com/jenksed/yourmove-go/internal/application/discovery"
)

func (a *App) candidatePage(w http.ResponseWriter, r *http.Request) {
	p, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to load candidate profile", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := a.resources.candidateTemplate.Execute(w, p); err != nil {
		log.Printf("render profile: %v", err)
	}
}
func (a *App) updateCandidate(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 12<<10) {
		return
	}
	p := CandidateProfile{FirstName: r.PostForm.Get("first_name"), LastName: r.PostForm.Get("last_name"), Email: r.PostForm.Get("email"), Phone: r.PostForm.Get("phone"), City: r.PostForm.Get("city"), Region: r.PostForm.Get("region"), Country: r.PostForm.Get("country"), LinkedIn: r.PostForm.Get("linkedin"), GitHub: r.PostForm.Get("github"), Portfolio: r.PostForm.Get("portfolio"), Pronouns: r.PostForm.Get("pronouns"), Approved: r.PostForm.Get("approve") == "yes"}
	if err := a.saveCandidateProfile(p); err != nil {
		http.Error(w, "Profile was not saved: "+err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/candidate", http.StatusSeeOther)
}

type inspectionPageModel struct {
	Job              Job
	Item             *Inspection
	Profile          CandidateProfile
	Found            int
	Mapped           int
	Unsupported      bool
	StatusNote       string
	BrowserOpen      bool
	BrowserSupported bool
	BrowserViewingID int64
	BrowserLatestID  int64
	BrowserItem      *Inspection
	BrowserHistory   []InspectionSummary
	BrowserState     browserRunState
	SelectedResume   *Resume
	Handoff          HandoffEligibility
	Engines          []playwright.EngineInfo
}

func (a *App) inspectionPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Unable to read job", 500)
		}
		return
	}
	profile, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to read profile", 500)
		return
	}
	view := inspectionPageModel{Job: job, Profile: profile, BrowserSupported: browser.OwnsPostingURL(job.URL, job.URL), Engines: playwright.AvailableEngines()}
	view.BrowserState = a.browsers.state(id)
	view.BrowserOpen = view.BrowserState.Active
	if job.SelectedResumeID > 0 {
		if resume, e := a.resumeByID(job.SelectedResumeID); e == nil {
			view.SelectedResume = &resume
		}
	}
	view.Handoff = assessResumeHandoff(job, view.SelectedResume)
	view.BrowserHistory, _ = a.inspectionSummaries(id, "browser", 12)
	if len(view.BrowserHistory) > 0 {
		view.BrowserLatestID = view.BrowserHistory[0].ID
	}
	if browserItem, e := a.latestInspectionBySource(id, "browser"); e == nil {
		view.BrowserItem = &browserItem
	}
	if requestID, e := strconv.ParseInt(r.URL.Query().Get("browser"), 10, 64); e == nil && requestID > 0 {
		if requested, e := a.inspectionByID(requestID); e == nil && requested.JobID == id && requested.Source == "browser" {
			view.BrowserItem = &requested
		}
	}
	if view.BrowserItem != nil {
		view.BrowserViewingID = view.BrowserItem.ID
	}
	item, err := a.latestInspectionBySource(id, "public_api")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Unable to read inspection", 500)
		return
	}
	if err == nil {
		for i := range item.Fields {
			item.Fields[i].Mapping = mapProfileField(item.Fields[i].Field, profile)
			if item.Fields[i].Mapping == "approved profile" {
				view.Mapped++
			}
		}
		view.Found = len(item.Fields)
		view.Item = &item
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := a.resources.inspectionTemplate.Execute(w, view); err != nil {
		log.Printf("render inspection: %v", err)
	}
}

// inspectJob never opens an employer-facing application nor transmits profile facts.
// The URL comes exclusively from the saved job and is checked again by the provider.
func (a *App) inspectJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	returnToPrepare := r.PostForm.Get("return_to") == "prepare"
	prepareLocation := func(notice string) string {
		location := "/jobs/" + strconv.FormatInt(id, 10)
		if notice != "" {
			location += "?early_discovery=" + url.QueryEscape(notice)
		}
		return location + "#prepare"
	}
	job, err := a.jobByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load job", 500)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		http.Error(w, "Inspection is limited to active applications", 409)
		return
	}
	fetch := a.inspectApplication
	if fetch == nil {
		registry := discovery.New()
		fetch = registry.Inspect
	}
	result, err := fetch(r.Context(), job.URL)
	if errors.Is(err, discovery.ErrUnsupported) {
		if returnToPrepare {
			http.Redirect(w, r, prepareLocation("unsupported"), http.StatusSeeOther)
			return
		}
		http.Error(w, "No public application schema is available for this platform yet. The employer site can still be opened manually.", 422)
		return
	}
	if err != nil {
		log.Printf("public application discovery for job %d: %v", id, err)
		if returnToPrepare {
			http.Redirect(w, r, prepareLocation("failed"), http.StatusSeeOther)
			return
		}
		http.Error(w, "Public inspection failed; saved data has not been changed. Retry later or open the employer site.", 502)
		return
	}
	if _, err := a.saveInspection(id, result); err != nil {
		http.Error(w, "Unable to store inspection", 500)
		return
	}
	if returnToPrepare {
		http.Redirect(w, r, prepareLocation(""), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10)+"/inspection", http.StatusSeeOther)
}

// previewEarlyQuestions opens one isolated public Ashby application page,
// observes its current visible controls once, and stores only structural
// evidence. It never registers a managed watcher or authorizes browser writes.
func (a *App) previewEarlyQuestions(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Unable to load job", http.StatusInternalServerError)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		http.Error(w, "Preview is limited to active applications", http.StatusConflict)
		return
	}
	if browser.PostingPlatform(job.URL) != "ashby" || !browser.OwnsPostingURL(job.URL, job.URL) {
		http.Error(w, "Read-only question preview is available only for saved Ashby postings", http.StatusUnprocessableEntity)
		return
	}
	fetch := a.previewApplication
	if fetch == nil {
		fetch = chromium.Preview
	}
	result, err := fetch(r.Context(), job.URL)
	if err != nil {
		log.Printf("read-only Ashby preview for job %d: %v", id, err)
		if r.PostForm.Get("return_to") == "prepare" {
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d?early_preview=failed#prepare", id), http.StatusSeeOther)
			return
		}
		http.Error(w, "Read-only preview did not complete. Saved questions and answers were not changed. Continue through the rendered-form inspection path.", http.StatusBadGateway)
		return
	}
	if result.Platform != "ashby" || result.Source != "preview" || result.Completeness != "rendered_preview_partial" || !browser.OwnsPostingURL(job.URL, result.ApplicationURL) {
		http.Error(w, "Preview did not return a partial structural snapshot for the saved Ashby posting. Nothing was saved.", http.StatusConflict)
		return
	}
	if len(result.Fields) > 200 || len(result.Steps) > 30 {
		http.Error(w, "Preview exceeded the observation limit. Nothing was saved.", http.StatusUnprocessableEntity)
		return
	}
	if _, err := a.saveInspection(id, result); err != nil {
		http.Error(w, "Unable to save preview questions", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d#prepare", id), http.StatusSeeOther)
}
