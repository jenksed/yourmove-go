package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
)

var jobsTemplate = template.Must(template.New("jobs").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>YourMove — Saved Jobs</title>
</head>
<body>
    <h1>Add a Job</h1>

    <form method="POST" action="/jobs">
        <label for="job_url">Job URL</label>
        <input id="job_url" name="job_url" type="url" required>
		<label for="job_title">Job Title (optional)</label>
		<input id="job_title" name="job_title" type="text">
        <button type="submit">Add Job</button>
    </form>

    <h2>Saved Jobs</h2>
    <ul>
        {{range .}}
            <li>
                <a href="{{.URL}}" target="_blank" rel="noopener noreferrer">
					{{if .Title}}
						{{.Title}}
					{{else}}
						{{.URL}}
					{{end}}
				</a>
				({{.Host}})
            </li>
        {{else}}
            <li>No saved jobs yet.</li>
        {{end}}
    </ul>
</body>
</html>
`))

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "<h1>YourMove — Job Tracker</h1>")
	fmt.Fprint(w, "<h2>Let's GOOOOOOO</h2>")
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

		job := Job{
			URL:   jobURL,
			Host:  parsed.Hostname(),
			Title: jobTitle,
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
