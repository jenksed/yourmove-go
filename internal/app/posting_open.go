package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
)

// openJobPosting is deliberately a desktop-only POST. A stored job ID, not a
// client-provided URL, determines what the operating system is asked to open.
// Browser-mode templates use normal external anchors instead.
func (a *App) openJobPosting(w http.ResponseWriter, r *http.Request) {
	if a.openPosting == nil {
		http.NotFound(w, r)
		return
	}
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	job, err := a.jobByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Printf("load posting %d: %v", id, err)
		http.Error(w, "Unable to load saved posting", http.StatusInternalServerError)
		return
	}
	// Reject stale/malformed schemes, even for data stored before URL validation.
	_, _, err = normalizePostingURL(job.URL)
	if err != nil {
		http.Error(w, "Saved job URL is invalid; edit your job data before opening", http.StatusUnprocessableEntity)
		return
	}
	// Preserve the exact saved URL, including query and fragment, once validated.
	if err := a.openPosting(job.URL); err != nil {
		log.Printf("open posting in default browser: %v", err)
		http.Error(w, "Could not launch default browser; copy the saved posting URL to open it manually", http.StatusBadGateway)
		return
	}
	// The existing desktop 303 shim navigates the WebView back to the workspace.
	http.Redirect(w, r, "/jobs/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
