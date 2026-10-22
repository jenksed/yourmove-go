package app

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

func (a *App) submitJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Confirm submission completed on employer website", 400)
		return
	}
	notes := strings.TrimSpace(r.PostForm.Get("notes"))
	if len(notes) > 1000 {
		http.Error(w, "Notes too long", 400)
		return
	}
	token := r.PostForm.Get("submission_token")
	if err := a.recordSubmission(id, notes, token); err != nil {
		if errors.Is(err, ErrSubmissionReviewStale) {
			http.Error(w, "The reviewed resume changed or is unavailable, or the selected resume changed. Review the application and resume again before confirming.", http.StatusConflict)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("record application: %v", err)
		http.Error(w, "Unable to record application", 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d#history", id), http.StatusSeeOther)
}
