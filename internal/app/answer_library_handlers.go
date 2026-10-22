package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/schema"
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
		Revision         int64  `json:"revision"`
		OriginalQuestion string `json:"original_question"`
		Answer           string `json:"answer"`
		SourceEmployer   string `json:"source_employer"`
		SourceJobTitle   string `json:"source_job_title"`
		SourceHost       string `json:"source_host"`
	}{entry.Revision, entry.OriginalQuestion, entry.Answer, entry.SourceEmployer, entry.SourceJobTitle, entry.SourceHost})
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
	entryValues, keyValues, revisionValues, libraryRevisionValues := r.PostForm["library_entry_id"], r.PostForm["question_key"], r.PostForm["revision"], r.PostForm["library_revision"]
	if len(entryValues) != 1 || len(keyValues) != 1 || len(revisionValues) != 1 || len(libraryRevisionValues) != 1 || !questionReturnKey.MatchString(keyValues[0]) {
		http.Error(w, "Review the current question and selected example", http.StatusBadRequest)
		return
	}
	entryID, entryErr := strconv.ParseInt(entryValues[0], 10, 64)
	revision, revisionErr := strconv.ParseInt(revisionValues[0], 10, 64)
	libraryRevision, libraryRevisionErr := strconv.ParseInt(libraryRevisionValues[0], 10, 64)
	if entryErr != nil || entryID <= 0 || revisionErr != nil || revision < 0 || libraryRevisionErr != nil || libraryRevision < 1 {
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
	destinationValues := r.PostForm["destination"]
	if len(destinationValues) != 1 || destinationValues[0] != job.URL {
		http.Error(w, "The application destination changed. Review the current question before using previous writing.", http.StatusConflict)
		return
	}
	var currentField schema.Field
	validCurrent := false
	if r.PostForm.Get("early") == "yes" {
		source, fp, sourceID := r.PostForm.Get("discovery_source"), r.PostForm.Get("discovery_fingerprint"), r.PostForm.Get("source_question_id")
		if source == "" || fp == "" || sourceID == "" || len(r.PostForm["early"]) != 1 || len(r.PostForm["discovery_source"]) != 1 || len(r.PostForm["discovery_fingerprint"]) != 1 || len(r.PostForm["source_question_id"]) != 1 {
			http.Error(w, "The discovered question changed. Review it before using previous writing.", http.StatusConflict)
			return
		}
		prepared, prepErr := a.prepareEarlyQuestions(job, r)
		if prepErr != nil || prepared.Item == nil || prepared.Item.Source != source || prepared.Item.Fingerprint != fp {
			http.Error(w, "The discovered question changed. Review it before using previous writing.", http.StatusConflict)
			return
		}
		for _, q := range prepared.Questions {
			if q.Key == keyValues[0] && q.SourceQuestionID == sourceID && q.Destination == job.URL && q.Revision == revision && q.Editable && q.TriageCategory == "answer" && earlyPotentialNarrative(q.Field) {
				currentField, validCurrent = q.Field, true
				break
			}
		}
	} else {
		profile, profileErr := a.candidateProfile()
		if profileErr != nil {
			http.Error(w, "Unable to read current application questions", http.StatusInternalServerError)
			return
		}
		prepared, prepErr := a.prepareQuestions(job, profile)
		if prepErr != nil {
			http.Error(w, "Unable to read current application questions", http.StatusInternalServerError)
			return
		}
		for _, q := range prepared.Questions {
			if q.Key == keyValues[0] && q.Destination == job.URL && q.Editable && q.Revision == revision && answerLibraryEligible(q.Field, true) {
				currentField, validCurrent = q.Field, true
				break
			}
		}
	}
	if !validCurrent || !answerLibraryEligible(currentField, true) {
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
	if entry.Revision != libraryRevision {
		http.Error(w, "This Answer Library example changed. Review the updated example before using it.", http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(struct {
		EntryID         int64  `json:"entry_id"`
		LibraryRevision int64  `json:"library_revision"`
		QuestionKey     string `json:"question_key"`
		Revision        int64  `json:"revision"`
		Answer          string `json:"answer"`
	}{entry.ID, entry.Revision, keyValues[0], revision, entry.Answer})
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
	revisionValues := r.PostForm["revision"]
	if len(revisionValues) != 1 {
		http.Error(w, "Review the current Answer Library revision before deleting this example", http.StatusBadRequest)
		return
	}
	revision, err := strconv.ParseInt(revisionValues[0], 10, 64)
	if err != nil || revision < 1 {
		http.Error(w, "Invalid Answer Library revision", http.StatusBadRequest)
		return
	}
	if err := a.deleteAnswerLibrary(id, revision); err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(err, ErrAnswerLibraryStale):
			http.Error(w, "This library copy changed in another window. Reload the Answer Library before deleting.", http.StatusConflict)
		case errors.Is(err, ErrAnswerLibraryInvalid):
			http.Error(w, "Invalid Answer Library revision", http.StatusBadRequest)
		default:
			http.Error(w, "Unable to delete this Answer Library example", http.StatusInternalServerError)
		}
		return
	}
	http.Redirect(w, r, "/answer-library?notice=deleted", http.StatusSeeOther)
}
