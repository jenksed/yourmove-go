package main

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/discovery"
)

var candidateTemplate = template.Must(template.ParseFS(templateFiles, "templates/candidate.html", "templates/partials.html"))
var inspectionTemplate = template.Must(template.ParseFS(templateFiles, "templates/inspection.html", "templates/partials.html"))

func (a *App) candidatePage(w http.ResponseWriter, r *http.Request) {
	p, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to load candidate profile", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := candidateTemplate.Execute(w, p); err != nil {
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
	view := inspectionPageModel{Job: job, Profile: profile, BrowserSupported: browser.OwnsPostingURL(job.URL, job.URL)}
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
	if err := inspectionTemplate.Execute(w, view); err != nil {
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
		http.Error(w, "No public application schema is available for this platform yet. The employer site can still be opened manually.", 422)
		return
	}
	if err != nil {
		log.Printf("public application discovery for job %d: %v", id, err)
		http.Error(w, "Public inspection failed; saved data has not been changed. Retry later or open the employer site.", 502)
		return
	}
	if _, err := a.saveInspection(id, result); err != nil {
		http.Error(w, "Unable to store inspection", 500)
		return
	}
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10)+"/inspection", http.StatusSeeOther)
}
