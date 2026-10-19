package main

import (
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"example.com/yourmove-lite/internal/platforms"
)

var jobsTemplate = template.Must(
	template.ParseFiles("templates/jobs.html"),
)

var jobDetailTemplate = template.Must(
	template.ParseFiles("templates/job.html"),
)

func home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (a *App) jobForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Load jobs from SQLite before writing HTML.
		jobs, err := a.allJobs()
		if err != nil {
			log.Printf("load jobs: %v", err)
			http.Error(w, "Unable to load jobs", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := jobsTemplate.Execute(w, jobs); err != nil {
			log.Printf("render jobs: %v", err)
		}

		return

	}

	if r.Method == http.MethodPost {
		jobURL := strings.TrimSpace(r.FormValue("job_url"))
		jobTitle := strings.TrimSpace(r.FormValue("job_title"))

		parsed, err := url.Parse(jobURL)

		if err != nil ||
			(parsed.Scheme != "https" && parsed.Scheme != "http") ||
			parsed.Host == "" {

			http.Error(w, "Invalid job URL", http.StatusBadRequest)
			return
		}

		jobDescription := ""

		metadata, err := platforms.FetchJob(r.Context(), jobURL)

		switch {
		case errors.Is(err, platforms.ErrUnsupported):
			// Unsupported platforms can still be saved.
		case err != nil:
			log.Printf("extract job metadata: %v", err)
		default:
			jobDescription = metadata.Description

			if jobTitle == "" {
				jobTitle = metadata.Title
			}
		}

		job := Job{
			URL:         jobURL,
			Host:        parsed.Hostname(),
			Title:       jobTitle,
			Description: jobDescription,
		}

		if err := a.saveJob(job); err != nil {
			log.Printf("save job: %v", err)
			http.Error(w, "Unable to save job", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/jobs", http.StatusSeeOther)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (a *App) jobDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	job, err := a.jobByID(id)

	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}

	if err != nil {
		log.Printf("load job detail: %v", err)
		http.Error(w, "Unable to load job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := jobDetailTemplate.Execute(w, job); err != nil {
		log.Printf("render job detail: %v", err)
	}
}
