package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type careerEvidencePageData struct {
	Entries []CareerEvidence
	Current *CareerEvidence
	Notice  string
}

func (a *App) careerEvidencePage(w http.ResponseWriter, r *http.Request) {
	entries, err := a.careerEvidenceEntries()
	if err != nil {
		http.Error(w, "Unable to load Career Evidence", http.StatusInternalServerError)
		return
	}
	notice := map[string]string{
		"created":  "Draft created. Add the details you remember, then save your changes.",
		"saved":    "Career Evidence draft saved.",
		"reviewed": "Marked as reviewed by you. This is your confirmation, not independent verification.",
		"deleted":  "Career Evidence record deleted.",
		"source":   "Draft created with a snapshot of the original writing for reference.",
	}[r.URL.Query().Get("notice")]
	a.renderCareerEvidence(w, careerEvidencePageData{Entries: entries, Notice: notice})
}

func (a *App) careerEvidenceEntryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	evidence, err := careerEvidenceByID(a.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load this Career Evidence record", http.StatusInternalServerError)
		return
	}
	entries, err := a.careerEvidenceEntries()
	if err != nil {
		http.Error(w, "Unable to load Career Evidence", http.StatusInternalServerError)
		return
	}
	data := careerEvidencePageData{Entries: entries, Current: &evidence}
	data.Notice = map[string]string{
		"created":  "Draft created. Add the details you remember, then save your changes.",
		"saved":    "Career Evidence draft saved.",
		"reviewed": "Marked as reviewed by you. This is your confirmation, not independent verification.",
		"source":   "Draft created with a snapshot of the original writing for reference. Rewrite it as situation, actions, and outcome; nothing was extracted.",
	}[r.URL.Query().Get("notice")]
	a.renderCareerEvidence(w, data)
}

func (a *App) renderCareerEvidence(w http.ResponseWriter, data careerEvidencePageData) {
	a.renderCareerEvidenceStatus(w, data, http.StatusOK)
}

func (a *App) renderCareerEvidenceStatus(w http.ResponseWriter, data careerEvidencePageData, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = a.resources.careerEvidenceTemplate.Execute(w, data)
}

func (a *App) createCareerEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	id, err := a.createCareerEvidenceDraft()
	if err != nil {
		http.Error(w, "Unable to create a Career Evidence draft", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/career-evidence/"+strconv.FormatInt(id, 10)+"?notice=created", http.StatusSeeOther)
}

func (a *App) createCareerEvidenceFromLibraryHandler(w http.ResponseWriter, r *http.Request) {
	libraryID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || libraryID < 1 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	evidenceID, err := a.createCareerEvidenceFromLibrary(libraryID)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrCareerEvidenceInvalid):
			http.Error(w, "The source example cannot be copied into Career Evidence", http.StatusUnprocessableEntity)
		default:
			http.Error(w, "Unable to create a Career Evidence draft", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/career-evidence/"+strconv.FormatInt(evidenceID, 10)+"?notice=source", http.StatusSeeOther)
}

func parseCareerEvidenceRevision(r *http.Request) (int64, error) {
	values := r.PostForm["revision"]
	if len(values) != 1 {
		return 0, ErrCareerEvidenceInvalid
	}
	revision, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || revision < 1 {
		return 0, ErrCareerEvidenceInvalid
	}
	return revision, nil
}

func parseCareerEvidenceContent(r *http.Request) (careerEvidenceContent, error) {
	get := func(name string) (string, error) {
		values := r.PostForm[name]
		if len(values) != 1 {
			return "", ErrCareerEvidenceInvalid
		}
		value := strings.ReplaceAll(values[0], "\r\n", "\n")
		return strings.ReplaceAll(value, "\r", "\n"), nil
	}
	var content careerEvidenceContent
	fields := []struct {
		name string
		to   *string
	}{
		{"title", &content.Title}, {"situation", &content.Situation}, {"actions", &content.Actions},
		{"employer_context", &content.EmployerContext}, {"role_context", &content.RoleContext},
		{"technologies", &content.Technologies}, {"outcome", &content.Outcome}, {"outcome_state", &content.OutcomeState}, {"period", &content.Period},
		{"source_notes", &content.SourceNotes},
	}
	for _, field := range fields {
		value, err := get(field.name)
		if err != nil {
			return careerEvidenceContent{}, err
		}
		*field.to = value
	}
	if !content.valid() {
		return careerEvidenceContent{}, ErrCareerEvidenceInvalid
	}
	return content, nil
}

func (a *App) updateCareerEvidenceHandler(w http.ResponseWriter, r *http.Request, review bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 96<<10) {
		return
	}
	if review {
		confirm := r.PostForm["confirm_review"]
		if len(confirm) != 1 || confirm[0] != "yes" {
			http.Error(w, "Confirm that this record accurately describes your experience before marking it reviewed", http.StatusBadRequest)
			return
		}
	}
	revision, err := parseCareerEvidenceRevision(r)
	if err != nil {
		http.Error(w, "Review the current Career Evidence revision before saving", http.StatusBadRequest)
		return
	}
	content, err := parseCareerEvidenceContent(r)
	if err != nil {
		http.Error(w, "Career Evidence fields are invalid or too long", http.StatusUnprocessableEntity)
		return
	}
	if err := a.updateCareerEvidence(id, revision, content, review); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrCareerEvidenceStale):
			http.Error(w, "This Career Evidence record changed in another window. Reload it before saving.", http.StatusConflict)
		case errors.Is(err, ErrCareerEvidenceIncomplete):
			a.renderCareerEvidenceReviewError(w, id, content, "Add a title, situation, and actions before marking this record reviewed.")
			return
		case errors.Is(err, ErrCareerEvidenceOutcomeMissing):
			a.renderCareerEvidenceReviewError(w, id, content, "Add the actual outcome or change its status to Unknown or Not recorded.")
			return
		case errors.Is(err, ErrCareerEvidenceInvalid):
			http.Error(w, "Career Evidence fields are invalid or too long", http.StatusUnprocessableEntity)
		default:
			http.Error(w, "Unable to save Career Evidence", http.StatusInternalServerError)
		}
		return
	}
	notice := "saved"
	if review {
		notice = "reviewed"
	}
	http.Redirect(w, r, "/career-evidence/"+strconv.FormatInt(id, 10)+"?notice="+notice, http.StatusSeeOther)
}

func (a *App) renderCareerEvidenceReviewError(w http.ResponseWriter, id int64, content careerEvidenceContent, message string) {
	evidence, err := careerEvidenceByID(a.db, id)
	if err != nil {
		http.Error(w, "Unable to reload Career Evidence after the review check", http.StatusInternalServerError)
		return
	}
	if !evidence.content().equal(content) {
		evidence.Status = "draft"
	}
	evidence.Title, evidence.Situation, evidence.Actions = content.Title, content.Situation, content.Actions
	evidence.EmployerContext, evidence.RoleContext, evidence.Technologies = content.EmployerContext, content.RoleContext, content.Technologies
	evidence.Outcome, evidence.OutcomeState, evidence.Period, evidence.SourceNotes = content.Outcome, content.OutcomeState, content.Period, content.SourceNotes
	entries, err := a.careerEvidenceEntries()
	if err != nil {
		http.Error(w, "Unable to load Career Evidence", http.StatusInternalServerError)
		return
	}
	a.renderCareerEvidenceStatus(w, careerEvidencePageData{Entries: entries, Current: &evidence, Notice: message}, http.StatusUnprocessableEntity)
}

func (a *App) saveCareerEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	a.updateCareerEvidenceHandler(w, r, false)
}

func (a *App) reviewCareerEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	a.updateCareerEvidenceHandler(w, r, true)
}

func (a *App) deleteCareerEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	confirm := r.PostForm["confirm"]
	if len(confirm) != 1 || confirm[0] != "yes" {
		http.Error(w, "Check the confirmation to delete this Career Evidence record", http.StatusBadRequest)
		return
	}
	revision, err := parseCareerEvidenceRevision(r)
	if err != nil {
		http.Error(w, "Review the current Career Evidence revision before deleting", http.StatusBadRequest)
		return
	}
	if err := a.deleteCareerEvidence(id, revision); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrCareerEvidenceStale):
			http.Error(w, "This Career Evidence record changed in another window. Reload it before deleting.", http.StatusConflict)
		case errors.Is(err, ErrCareerEvidenceInvalid):
			http.Error(w, "Invalid Career Evidence revision", http.StatusBadRequest)
		default:
			http.Error(w, "Unable to delete Career Evidence", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/career-evidence?notice=deleted", http.StatusSeeOther)
}
