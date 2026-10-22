package app

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
)

type answerLibraryPage struct {
	Entries []AnswerLibraryEntry
	Notice  string
}

func (a *App) answerLibrary(w http.ResponseWriter, r *http.Request) {
	entries, err := a.answerLibraryEntries()
	if err != nil {
		http.Error(w, "Unable to load the Answer Library", http.StatusInternalServerError)
		return
	}
	notice := map[string]string{
		"saved":    "Saved as a separate Answer Library example. The employer-specific answer remains unchanged.",
		"existing": "This saved answer revision is already in the Answer Library. Its existing library copy was left unchanged.",
		"edited":   "The Answer Library copy was updated. The original application answer was left unchanged.",
		"deleted":  "The Answer Library example was deleted. Application history was left unchanged.",
	}[r.URL.Query().Get("notice")]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := a.resources.answerLibraryTemplate.Execute(w, answerLibraryPage{Entries: entries, Notice: notice}); err != nil {
		http.Error(w, "Unable to render the Answer Library", http.StatusInternalServerError)
	}
}

func (a *App) answerLibraryEntryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	entry, err := a.answerLibraryBySourceID(a.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to load this Answer Library example", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := a.resources.answerLibraryTemplate.Execute(w, answerLibraryPage{Entries: []AnswerLibraryEntry{entry}}); err != nil {
		http.Error(w, "Unable to render this Answer Library example", http.StatusInternalServerError)
	}
}

func (a *App) promoteApplicationAnswerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	keyValues, revisionValues := r.PostForm["question_key"], r.PostForm["revision"]
	if len(keyValues) != 1 || len(revisionValues) != 1 || !questionReturnKey.MatchString(keyValues[0]) {
		http.Error(w, "Review the saved answer before adding it to the Answer Library", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Review the saved answer revision before adding it to the Answer Library", http.StatusBadRequest)
		return
	}
	_, created, err := a.promoteApplicationAnswer(id, keyValues[0], revision)
	if err != nil {
		switch {
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This saved answer changed. Review its current revision before adding it to the Answer Library.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryIneligible):
			http.Error(w, "This question is unsupported for the Answer Library. Only verified, eligible narrative answers can be saved.", http.StatusUnprocessableEntity)
		case errors.Is(err, ErrAnswerLibrarySource):
			http.Error(w, "The saved source answer could not be verified. Review the current observed question and saved answer.", http.StatusConflict)
		default:
			http.Error(w, "Unable to save this example to the Answer Library", http.StatusInternalServerError)
		}
		return
	}
	notice := "saved"
	if !created {
		notice = "existing"
	}
	http.Redirect(w, r, "/answer-library?notice="+notice, http.StatusSeeOther)
}

func (a *App) editAnswerLibraryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 24<<10) {
		return
	}
	revisionValues, answerValues := r.PostForm["revision"], r.PostForm["answer"]
	if len(revisionValues) != 1 || len(answerValues) != 1 {
		http.Error(w, "Invalid Answer Library edit", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Invalid Answer Library revision", http.StatusBadRequest)
		return
	}
	if err := a.updateAnswerLibrary(id, revision, answerValues[0]); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This library copy changed in another window. Reload the Answer Library before editing.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryInvalid):
			http.Error(w, "Answer text must be valid UTF-8, nonempty, and at most 12,000 bytes", http.StatusUnprocessableEntity)
		default:
			http.Error(w, "Unable to save the Answer Library edit", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/answer-library?notice=edited", http.StatusSeeOther)
}

func (a *App) deleteAnswerLibraryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	if r.PostForm.Get("confirm") != "yes" {
		http.Error(w, "Check the confirmation to delete this Answer Library example", http.StatusBadRequest)
		return
	}
	if err := a.deleteAnswerLibrary(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Unable to delete this Answer Library example", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/answer-library?notice=deleted", http.StatusSeeOther)
}
