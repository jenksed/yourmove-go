package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type answerLibraryPage struct {
	Entries         []AnswerLibraryEntry
	SearchResults   []answerLibrarySuggestion
	SearchQuery     string
	SearchPerformed bool
	Notice          string
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

func (a *App) renderAnswerLibrary(w http.ResponseWriter, data answerLibraryPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	if err := a.resources.answerLibraryTemplate.Execute(w, data); err != nil {
		http.Error(w, "Unable to render the Answer Library", http.StatusInternalServerError)
	}
}

func (a *App) searchAnswerLibraryHandler(w http.ResponseWriter, r *http.Request) {
	if !parseLocalForm(w, r, 2<<10) {
		return
	}
	values := r.PostForm["query"]
	if len(values) != 1 || len(values[0]) > 1000 || !utf8.ValidString(values[0]) || strings.ContainsRune(values[0], 0) {
		http.Error(w, "Search text is invalid or too long", http.StatusBadRequest)
		return
	}
	query := strings.TrimSpace(values[0])
	data := answerLibraryPage{SearchQuery: query, SearchPerformed: query != ""}
	if query == "" {
		entries, err := a.answerLibraryEntries()
		if err != nil {
			http.Error(w, "Unable to load the Answer Library", http.StatusInternalServerError)
			return
		}
		data.Entries = entries
	} else {
		results, err := a.searchAnswerLibrary(query, answerLibrarySearchPageLimit)
		if err != nil {
			http.Error(w, "Unable to search the Answer Library", http.StatusInternalServerError)
			return
		}
		data.SearchResults = results
	}
	a.renderAnswerLibrary(w, data)
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

func (a *App) answerLibraryEntryContentHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(struct {
		OriginalQuestion string `json:"original_question"`
		Answer           string `json:"answer"`
		SourceEmployer   string `json:"source_employer"`
		SourceJobTitle   string `json:"source_job_title"`
		SourceHost       string `json:"source_host"`
	}{entry.OriginalQuestion, entry.Answer, entry.SourceEmployer, entry.SourceJobTitle, entry.SourceHost})
}

func (a *App) answerLibraryStartingPointHandler(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseJobID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !parseLocalForm(w, r, 4<<10) {
		return
	}
	entryValues, keyValues, revisionValues := r.PostForm["library_entry_id"], r.PostForm["question_key"], r.PostForm["revision"]
	if len(entryValues) != 1 || len(keyValues) != 1 || len(revisionValues) != 1 || !questionReturnKey.MatchString(keyValues[0]) {
		http.Error(w, "Review the current question and selected example", http.StatusBadRequest)
		return
	}
	entryID, entryErr := strconv.ParseInt(entryValues[0], 10, 64)
	revision, revisionErr := strconv.ParseInt(revisionValues[0], 10, 64)
	if entryErr != nil || entryID <= 0 || revisionErr != nil || revision < 0 {
		http.Error(w, "Review the current question and selected example", http.StatusBadRequest)
		return
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		http.Error(w, "Completed applications cannot be edited", http.StatusConflict)
		return
	}
	profile, err := a.candidateProfile()
	if err != nil {
		http.Error(w, "Unable to read current application questions", http.StatusInternalServerError)
		return
	}
	prepared, err := a.prepareQuestions(job, profile)
	if err != nil {
		http.Error(w, "Unable to read current application questions", http.StatusInternalServerError)
		return
	}
	var current *preparedQuestion
	for i := range prepared.Questions {
		if prepared.Questions[i].Key == keyValues[0] {
			current = &prepared.Questions[i]
			break
		}
	}
	if current == nil || !current.Editable || !answerLibraryEligible(current.Field, true) || current.Revision != revision {
		http.Error(w, "The current application question changed. Review it before using previous writing.", http.StatusConflict)
		return
	}
	entry, err := a.answerLibraryBySourceID(a.db, entryID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Unable to read the selected Answer Library example", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(struct {
		EntryID     int64  `json:"entry_id"`
		QuestionKey string `json:"question_key"`
		Revision    int64  `json:"revision"`
		Answer      string `json:"answer"`
	}{entry.ID, current.Key, current.Revision, entry.Answer})
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
