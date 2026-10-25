package app

import (
	"database/sql"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// createManualCoverLetterHandler saves text written by the applicant without
// consulting AI providers or claiming that resume/job evidence was used.
func (a *App) createManualCoverLetterHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// A URL-encoded form can expand every input byte to three bytes.
	if !parseLocalForm(w, r, 3*maxLetterBytes+4096) {
		return
	}
	body := r.PostForm.Get("body")
	fail := func(status int, message string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = manualLetterRecovery.Execute(w, struct{ ID, Message, Body string }{strconv.FormatInt(id, 10), message, strings.ReplaceAll(strings.ToValidUTF8(r.PostForm.Get("body"), "�"), "\x00", "�")})
	}

	if len(strings.TrimSpace(body)) < 30 || len(body) > maxLetterBytes || !utf8.ValidString(body) || strings.ContainsRune(body, 0) {
		fail(http.StatusUnprocessableEntity, "Write 30–12000 bytes of valid text without NUL characters. Your submitted text is below.")
		return
	}
	if _, err := a.jobByID(id); errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "Unable to load saved job", http.StatusInternalServerError)
		return
	}

	result, err := a.db.Exec(`INSERT INTO cover_letters
		(job_id,body,provider,model,master_resume_id,master_sha256,role_resume_id,role_sha256,job_description_sha256,created_at)
		VALUES(?,?,'manual','user-authored',NULL,'',NULL,'','',?)`, id, body, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		fail(http.StatusInternalServerError, "The letter could not be saved. Copy your work below or try saving again.")
		return
	}
	draftID, err := result.LastInsertId()
	if err != nil || draftID <= 0 {
		http.Error(w, "Unable to identify the saved letter", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, coverDraftNoticeURL(id, draftID, "saved"), http.StatusSeeOther)
}

var manualLetterRecovery = template.Must(template.New("manual-letter-recovery").Parse(`<!doctype html><html lang="en"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Letter not saved</title><link rel="stylesheet" href="/static/styles.css"><main><h1>Letter not saved</h1><p role="alert">{{.Message}}</p><form method="post" action="/jobs/{{.ID}}/cover-letter/manual"><label for="body">Your letter</label><textarea id="body" name="body" rows="15" required>{{.Body}}</textarea><button type="submit">Save letter</button></form><a href="/jobs/{{.ID}}#cover-letter">Return to job</a></main></html>`))
