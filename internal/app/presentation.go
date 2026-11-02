package app

import (
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// humanDate changes presentation only. Callers retain the original timestamp in
// a time element so the exact recorded value remains available.
func humanDate(raw string) string {
	if raw == "" {
		return "Not recorded"
	}
	if value, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return value.Local().Format("Jan 2, 2006")
	}
	if value, err := time.Parse("2006-01-02", raw); err == nil {
		return value.Format("Jan 2, 2006")
	}
	return raw
}

// Shared material navigation accepts a job identity, never a return URL. Context
// is read from the body for POSTs and revalidated after every page transition.
func (a *App) sharedMaterialReturnJob(r *http.Request) *Job {
	values := r.URL.Query()["return_job"]
	if r.Method == http.MethodPost {
		values = r.PostForm["return_job"]
	}
	if len(values) != 1 {
		return nil
	}
	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != values[0] {
		return nil
	}
	job, err := a.jobByID(id)
	if err != nil {
		return nil
	}
	return &job
}

func sharedMaterialLocation(path string, job *Job, notice string) string {
	query := url.Values{}
	if job != nil {
		query.Set("return_job", strconv.FormatInt(job.ID, 10))
	}
	if notice != "" {
		query.Set("notice", notice)
	}
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}
