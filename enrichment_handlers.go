package main

import (
	"errors"
	"html/template"
	"net/http"
)

var enrichmentTemplate = template.Must(template.ParseFS(templateFiles, "templates/enrichment.html", "templates/partials.html"))

func (a *App) enrichmentPage(w http.ResponseWriter, r *http.Request) {
	view := struct {
		EnrichmentSnapshot
		Pending, Retryable int
		Percent            int
	}{EnrichmentSnapshot: a.currentEnrichment()}
	if view.Total > 0 {
		view.Percent = 100 * view.Done / view.Total
	}
	jobs, err := a.allJobs()
	if err != nil {
		http.Error(w, "Unable to load jobs", 500)
		return
	}
	for _, j := range jobs {
		if j.Stage == StageApplied || j.Stage == StageClosed || j.SubmissionCount > 0 {
			continue
		}
		if j.SourceStatus == "not_checked" || j.SourceStatus == "" {
			view.Pending++
		}
		if j.SourceStatus == "failed" {
			view.Retryable++
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := enrichmentTemplate.Execute(w, view); err != nil {
		http.Error(w, "Unable to render metadata progress", 500)
	}
}

func (a *App) enrichmentAction(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid action", 400)
		return
	}
	var err error
	switch r.PathValue("action") {
	case "start":
		_, err = a.startEnrichment(r.PostForm.Get("mode"))
	case "pause":
		err = a.pauseEnrichment()
	case "resume":
		err = a.resumeEnrichment()
	default:
		http.NotFound(w, r)
		return
	}
	if errors.Is(err, ErrEnrichmentBusy) || errors.Is(err, ErrEnrichmentState) {
		http.Redirect(w, r, "/enrichment", http.StatusSeeOther)
		return
	}
	if err != nil {
		http.Error(w, "Unable to control metadata check: "+err.Error(), 400)
		return
	}
	http.Redirect(w, r, "/enrichment", http.StatusSeeOther)
}
