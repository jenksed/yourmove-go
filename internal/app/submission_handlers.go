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
			http.Redirect(w, r, fmt.Sprintf("/jobs/%d?submission=stale#prepare", id), http.StatusSeeOther)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		log.Printf("record application: %v", err)
		http.Redirect(w, r, fmt.Sprintf("/jobs/%d?submission=failed#prepare", id), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/jobs/%d#history", id), http.StatusSeeOther)
}

func submissionRecoveryNotice(code string) (string, bool) {
	switch code {
	case "stale":
		return "Your confirmation was not recorded. This review expired, or the selected resume changed or is unavailable. Review the current resume, then confirm again.", false
	case "failed":
		return "YourMove could not confirm whether this recording completed. Check submission history before retrying. If no record appears, return to Prepare and try again.", true
	default:
		return "", false
	}
}
