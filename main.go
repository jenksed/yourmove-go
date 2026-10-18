package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
		jobURL := r.FormValue("job_url")

		parsed, err := url.Parse(jobURL)

		if err != nil ||
			(parsed.Scheme != "https" && parsed.Scheme != "http") ||
			parsed.Host == "" {

			http.Error(w, "Invalid job URL", http.StatusBadRequest)
			return
		}

		fmt.Fprintf(w, "Received: %s", jobURL)
		return
	}

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
