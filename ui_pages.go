package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
)

// These read-only presentation models do not change the job lifecycle.
// In particular, Applied is counted only when a submission exists.
type opportunityPage struct {
	Jobs          []Job
	Total         int
	Remaining     int
	Applied       int
	RecordedToday int
	Next          *NextAction
	Matches       map[int64]MatchRecommendation
	Desktop       bool
}

func summarizeJobs(jobs []Job) opportunityPage {
	page := opportunityPage{Jobs: jobs, Total: len(jobs), Next: recommendNext(jobs)}
	for _, job := range jobs {
		if job.SubmissionCount > 0 {
			page.Applied++
		} else if job.Stage != StageClosed && job.Stage != StageApplied {
			page.Remaining++
		}
	}
	return page
}

// The bar measures recorded applications within the current active/applied
// queue. Closed opportunities do not make application progress look complete.
func (p opportunityPage) QueueTotal() int { return p.Remaining + p.Applied }

func (p opportunityPage) RecordedPercent() int {
	if p.QueueTotal() == 0 {
		return 0
	}
	return p.Applied * 100 / p.QueueTotal()
}

// Count jobs, not repeated history entries. Day boundaries use the app's local
// timezone and AddDate preserves the boundary across daylight-saving changes.
func (a *App) applicationsRecordedToday(now time.Time) (int, error) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var count int
	err := a.db.QueryRow(`SELECT COUNT(DISTINCT job_id) FROM submissions WHERE submitted_at>=? AND submitted_at<?`,
		start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)).Scan(&count)
	return count, err
}

var applicationsTemplate = template.Must(template.ParseFS(templateFiles, "templates/applications.html", "templates/partials.html"))

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
