package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "<h1>YourMove — Job Tracker</h1>")
	fmt.Fprint(w, "<h2>Let's GOOOOOOO</h2>")
}

func jobForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
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

		fmt.Fprintf(w, `
			<h1>Job URL Validated</h1>
			<p>URL: %s</p>
			<p>Host: %s</p>
			<a href="/jobs">Add another job</a>
		`,
			html.EscapeString(job.URL),
			html.EscapeString(job.Host),
		)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func main() {
	http.HandleFunc("/", home)
	http.HandleFunc("/jobs", jobForm)

	fmt.Println("Open http://127.0.0.1:3000")

	err := http.ListenAndServe("127.0.0.1:3000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
