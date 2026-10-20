package main

import (
	"html/template"
	"log"
	"net/http"
)

// These read-only presentation models do not change the job lifecycle.
// In particular, Applied is counted only when a submission exists.
type opportunityPage struct {
	Jobs    []Job
	Total   int
	Ready   int
	Applied int
	Next    *NextAction
	Matches map[int64]MatchRecommendation
	Desktop bool
}

func summarizeJobs(jobs []Job) opportunityPage {
	page := opportunityPage{Jobs: jobs, Total: len(jobs), Next: recommendNext(jobs)}
	for _, job := range jobs {
		if job.SubmissionCount > 0 {
			page.Applied++
		} else if job.Stage == StageReady {
			page.Ready++
		}
	}
	return page
}

var applicationsTemplate = template.Must(template.ParseFiles("templates/applications.html", "templates/partials.html"))

func (a *App) applicationsPage(w http.ResponseWriter, r *http.Request) {
	jobs, err := a.allJobs()
	if err != nil {
		log.Printf("load application list: %v", err)
		http.Error(w, "Unable to load applications", http.StatusInternalServerError)
		return
	}
	submitted := make([]Job, 0)
	for i := len(jobs) - 1; i >= 0; i-- {
		if jobs[i].SubmissionCount > 0 {
			submitted = append(submitted, jobs[i])
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := applicationsTemplate.Execute(w, submitted); err != nil {
		log.Printf("render applications: %v", err)
	}
}
