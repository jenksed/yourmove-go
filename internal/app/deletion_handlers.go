package app

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) deleteJobHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid deletion confirmation", 400)
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Check the deletion confirmation", 400)
		return
	}
	var allowHistory bool
	switch r.PostForm.Get("confirm_phrase") {
	case "DELETE":
	case "DELETE APPLICATION HISTORY":
		if r.PostForm.Get("confirm_history") != "yes" {
			http.Error(w, "Confirm permanent removal of submission history", 400)
			return
		}
		allowHistory = true
	default:
		http.Error(w, "Type the confirmation phrase exactly", 400)
		return
	}
	_, err = a.deleteJobs([]int64{id}, allowHistory)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.NotFound(w, r)
	case errors.Is(err, ErrJobHasHistory):
		http.Error(w, "This job has submissions. Use the history-specific confirmation on its workspace.", 409)
	case err != nil:
		log.Printf("delete job: %v", err)
		http.Error(w, "Could not delete job", 500)
	default:
		http.Redirect(w, r, "/jobs", http.StatusSeeOther)
	}
}

// Bulk removal is deliberately restricted to jobs without submission records.
// Users can delete a submitted job through its individual high-friction control.
func (a *App) deleteSelectedJobs(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid deletion form", 400)
		return
	}
	if r.PostForm.Get("confirm") != "yes" || r.PostForm.Get("confirm_phrase") != "DELETE" {
		http.Error(w, "Check confirmation and type DELETE", 400)
		return
	}
	rawIDs := r.PostForm["job_ids"]
	if len(rawIDs) == 0 || len(rawIDs) > maxDeleteBatch {
		http.Error(w, "Select up to 50 jobs to delete", 400)
		return
	}
	ids := make([]int64, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "Invalid job ID", 400)
			return
		}
		ids = append(ids, id)
	}
	_, err := a.deleteJobs(ids, false)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "A selected job no longer exists. Refresh the list.", 409)
	case errors.Is(err, ErrJobHasHistory):
		http.Error(w, "Selection includes a job with confirmed submissions. Uncheck that job; remove it individually if intended.", 409)
	case err != nil:
		log.Printf("bulk deletion: %v", err)
		http.Error(w, "Unable to delete selected jobs; none were removed", 400)
	default:
		http.Redirect(w, r, "/jobs", http.StatusSeeOther)
	}
}
