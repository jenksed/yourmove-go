package main

import (
	"mime"
	"net/http"
	"strconv"
)

// downloadCoverLetter exports the saved draft, never unsaved editor contents.
func (a *App) downloadCoverLetter(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	draftID, err := strconv.ParseInt(r.PathValue("draft"), 10, 64)
	if err != nil || draftID <= 0 {
		http.NotFound(w, r)
		return
	}
	job, err := a.jobByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	draft, err := a.coverLetterByID(id, draftID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	profile, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to read candidate information", 500)
		return
	}
	data, err := coverLetterPDF(draft.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	filename := attachmentFilename(profile, job, "Cover_Letter", ".pdf")
	if a.exportDesktopDocument(w, r, filename, data, "/jobs/"+strconv.FormatInt(id, 10)+"?draft="+strconv.FormatInt(draftID, 10)+"#prepare") {
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}
