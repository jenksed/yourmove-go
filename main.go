package main

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"net/url"
	"strings"
)

type App struct {
	db *sql.DB
}

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

		fmt.Fprint(w, `
            <h1>Add a Job</h1>
            <form method="POST" action="/jobs">
                <label for="job_url">Job URL</label>
                <input
                    id="job_url"
                    name="job_url"
                    type="url"
                    required
                >
                <button type="submit">Add Job</button>
            </form>
        `)

		fmt.Fprint(w, "<h2>Saved Jobs</h2>")
		fmt.Fprint(w, "<ul>")

		for _, job := range jobs {
			fmt.Fprintf(
				w,
				"<li>%s (%s)</li>",
				html.EscapeString(job.URL),
				html.EscapeString(job.Host),
			)
		}

		fmt.Fprint(w, "</ul>")

		return
	}

	if r.Method == http.MethodPost {
		jobURL := strings.TrimSpace(r.FormValue("job_url"))

		parsed, err := url.Parse(jobURL)

		if err != nil ||
			(parsed.Scheme != "https" && parsed.Scheme != "http") ||
			parsed.Host == "" {

			http.Error(w, "Invalid job URL", http.StatusBadRequest)
			return
		}

		job := Job{
			URL:  jobURL,
			Host: parsed.Hostname(),
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

func main() {
	db, err := sql.Open("sqlite", "jobs.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY,
			url TEXT NOT NULL,
			host TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}

	app := &App{db: db}

	http.HandleFunc("/", home)
	http.HandleFunc("/jobs", app.jobForm)

	fmt.Println("Open http://127.0.0.1:3000")

	if err := http.ListenAndServe("127.0.0.1:3000", nil); err != nil {
		log.Fatal(err)
	}
}
