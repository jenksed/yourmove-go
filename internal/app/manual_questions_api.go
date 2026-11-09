package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

// Manual-question JSON transport. Manually created questions are
// applicant-authored preparation for postings where managed capture is
// unavailable or incomplete. They use application-owned keys, never describe
// an observed employer control, and never enter save, reconciliation, or
// transfer scope for observed controls:
//
//   - POST /api/jobs/{id}/manual-questions                       create one question
//   - POST /api/jobs/{id}/manual-questions/{key}                 save question and/or answer
//   - DELETE /api/jobs/{id}/manual-questions/{key}               remove one question
//   - POST /api/jobs/{id}/manual-questions/{key}/starting-point reuse a library example as a draft
//
// Reads never mint authority and never write durable state. Ordinary reads
// flow through GET /api/jobs/{id}/questions, which appends manual rows after
// the unified observed inventory without merging them into it.

type manualQuestionJSON struct {
	Key        string `json:"key"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	Revision   int64  `json:"revision"`
	Reusable   bool   `json:"-"`
	OriginText string `json:"-"`
}

type manualQuestionCreateInput struct {
	Question string `json:"question"`
}

type manualQuestionSaveInput struct {
	Question       string `json:"question"`
	Answer         string `json:"answer"`
	Revision       int64  `json:"revision"`
	SaveToLibrary  bool   `json:"save_to_library"`
	OriginEntryID  int64  `json:"origin_entry_id"`
	OriginRevision int64  `json:"origin_revision"`
	OriginClear    bool   `json:"origin_clear"`
}

type manualQuestionDeleteInput struct {
	Revision int64 `json:"revision"`
}

type manualQuestionStartingPointInput struct {
	LibraryEntryID  int64  `json:"library_entry_id"`
	LibraryRevision int64  `json:"library_revision"`
	Revision        int64  `json:"revision"`
	Destination     string `json:"destination"`
}

// toManualWritingQuestion projects one manually created question onto the
// Questions transport. It is always an editable applicant writing task and
// never a Fill target: Fill scope is computed from observed preparations,
// which never contain manual rows.
func toManualWritingQuestion(q manualApplicationQuestion, suggestions []answerLibrarySuggestion) writingQuestionJSON {
	field := manualQuestionField(q.Key, q.Question)
	eligible := answerLibraryEligible(field, true)
	out := writingQuestionJSON{
		Key: q.Key, Label: q.Question, Kind: "manual", Required: false,
		Value: q.Answer, Source: "Manually added question",
		Revision: q.Revision, Editable: true, LibraryEligible: eligible,
		HelpEligible: false, IsEarly: false,
		TriageCategory: "answer", ManualReason: "",
		IsManual:           true,
		LibrarySuggestions: toWritingLibrarySuggestions(suggestions),
	}
	switch {
	case q.Answer == "":
		out.LocalStatus = "Needs your answer. Complete it on the employer website yourself; automatic Fill is unavailable."
	default:
		out.LocalStatus = "Answer saved locally. Complete it on the employer website yourself; automatic Fill is unavailable."
	}
	return out
}

func (a *App) manualWritingRow(q manualApplicationQuestion) (writingQuestionJSON, error) {
	out := toManualWritingQuestion(q, nil)
	field := manualQuestionField(q.Key, q.Question)
	if out.Value == "" && answerLibraryEligible(field, true) {
		if entry, ok := a.libraryAutoForLabel(q.Question); ok {
			out.Value = entry.Answer
			out.Source = libraryAutoSource
			out.LocalStatus = "Automatically prepared from Answer Library; review and save locally. Complete it on the employer website yourself; automatic Fill is unavailable."
			out.LibraryAutoEntryID = entry.ID
			out.LibraryAutoRevision = entry.Revision
			out.LibraryAutoQuestion = entry.OriginalQuestion
			out.LibraryAutoPrepared = true
		}
		synthetic := preparedQuestion{JobID: q.JobID, Destination: q.Destination, Field: field, Key: q.Key, Value: out.Value, Revision: q.Revision, Editable: true}
		if suggestions, err := a.answerLibrarySuggestionsForQuestions([]preparedQuestion{synthetic}); err != nil {
			return out, err
		} else {
			out.LibrarySuggestions = toWritingLibrarySuggestions(suggestions[q.Key])
		}
	}
	if q.ReuseJSON != "" {
		var origin answerReuseOrigin
		if err := json.Unmarshal([]byte(q.ReuseJSON), &origin); err == nil && origin.LibraryEntryID > 0 {
			out.OriginEntryID = origin.LibraryEntryID
			out.OriginRevision = origin.LibraryRevision
			out.OriginQuestion = origin.SourceQuestion
			out.OriginEmployer = origin.SourceEmployer
			out.OriginRecorded = true
		}
	}
	return out, nil
}

func (a *App) manualJob(w http.ResponseWriter, r *http.Request) (Job, bool) {
	jobID, err := parseJobID(r)
	if err != nil {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return Job{}, false
	}
	job, err := a.jobByID(jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "Unknown job")
		return Job{}, false
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read the opportunity")
		return Job{}, false
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		writeResumeError(w, http.StatusConflict, "This application is no longer active")
		return Job{}, false
	}
	return job, true
}

// POST /api/jobs/{id}/manual-questions creates one applicant-authored question.
func (a *App) manualQuestionsCreateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	job, ok := a.manualJob(w, r)
	if !ok {
		return
	}
	var input manualQuestionCreateInput
	if !decodeResumeJSON(w, r, 4<<10, &input) {
		return
	}
	if !validManualQuestionText(input.Question) {
		writeResumeError(w, http.StatusUnprocessableEntity, "Enter the employer's question exactly as displayed.")
		return
	}
	created, err := a.createManualQuestion(job, input.Question)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to add the question")
		return
	}
	row, err := a.manualWritingRow(created)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to add the question")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "question": row})
}

// POST /api/jobs/{id}/manual-questions/{key} saves the applicant-authored
// question and/or prepared answer with an optimistic revision check.
func (a *App) manualQuestionSaveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	job, ok := a.manualJob(w, r)
	if !ok {
		return
	}
	key := r.PathValue("key")
	if !validManualQuestionKey(key) {
		writeResumeError(w, http.StatusNotFound, "Unknown manual question")
		return
	}
	var input manualQuestionSaveInput
	if !decodeResumeJSON(w, r, 24<<10, &input) {
		return
	}
	current, err := a.manualQuestionByKey(job, key)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusConflict, "This manual question changed. Review the current questions before editing.")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read the manual question")
		return
	}
	if input.Revision != current.Revision {
		writeResumeError(w, http.StatusConflict, "This manual question changed in another window. Review the current answer before editing.")
		return
	}
	if strings.TrimSpace(input.Question) != "" && !validManualQuestionText(input.Question) {
		writeResumeError(w, http.StatusUnprocessableEntity, "Enter the employer's question exactly as displayed.")
		return
	}
	if !validManualAnswerText(input.Answer) {
		writeResumeError(w, http.StatusUnprocessableEntity, "Answer is too long or invalid.")
		return
	}
	var originJSON string
	if input.OriginEntryID > 0 {
		if input.OriginRevision < 1 {
			writeResumeError(w, http.StatusBadRequest, "Review the selected source before saving with provenance.")
			return
		}
		entry, err := a.answerLibraryBySourceID(a.db, input.OriginEntryID)
		if errors.Is(err, sql.ErrNoRows) {
			writeResumeError(w, http.StatusConflict, "The selected source changed or was deleted. Review the updated example or save as independent writing.")
			return
		}
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to verify the selected source")
			return
		}
		if entry.Revision != input.OriginRevision {
			writeResumeError(w, http.StatusConflict, "The selected source changed. Review the updated example or save as independent writing.")
			return
		}
		originJSON, err = buildAnswerReuseOrigin(entry, current.Revision+1)
		if err != nil {
			writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
			return
		}
	} else if input.OriginClear {
		originJSON = ""
	} else {
		originJSON = current.ReuseJSON
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	defer tx.Rollback()
	now := manualQuestionTimestamp()
	nextQuestion := current.Question
	if strings.TrimSpace(input.Question) != "" {
		nextQuestion = strings.TrimSpace(input.Question)
	}
	result, err := tx.Exec(`UPDATE manual_application_questions SET question=?,answer=?,revision=revision+1,reuse_source_json=?,updated_at=? WHERE job_id=? AND destination=? AND question_key=? AND revision=?`, nextQuestion, input.Answer, originJSON, now, job.ID, job.URL, key, input.Revision)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		writeResumeError(w, http.StatusConflict, "This manual question changed in another window. Review the current answer before editing.")
		return
	}
	if input.SaveToLibrary {
		if err := a.saveManualWritingLibrary(tx, job, key, nextQuestion, input.Answer, input.Revision+1, input.OriginEntryID); err != nil {
			status := http.StatusInternalServerError
			message := "Unable to save the answer"
			switch {
			case errors.Is(err, ErrAnswerLibraryIneligible):
				status, message = 409, "Not saved to the library. This question is ineligible or matches conflicting library entries; it stays saved for this application only."
			case errors.Is(err, ErrAnswerLibraryInvalid):
				status, message = 409, "Not saved to the library. This question matches conflicting library entries; it stays saved for this application only."
			}
			writeResumeError(w, status, message)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	updated, err := a.manualQuestionByKey(job, key)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	row, err := a.manualWritingRow(updated)
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to save the answer")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"saved": true, "question": row})
}

// saveManualWritingLibrary mirrors the live save-to-library policy for an
// applicant-authored question: exact equivalents are kept, conflicting
// equivalents are refused, and a reviewed starting source stays reusable
// without duplicating the library.
func (a *App) saveManualWritingLibrary(tx *sql.Tx, job Job, key, question, answer string, revision int64, originEntryID int64) error {
	if strings.TrimSpace(answer) == "" || !validAnswerLibraryText(answer) {
		return ErrAnswerLibraryIneligible
	}
	if !answerLibraryEligible(manualQuestionField(key, question), true) {
		return ErrAnswerLibraryIneligible
	}
	ids, err := equivalentLibraryIDs(tx, question)
	if err != nil {
		return err
	}
	if len(ids) > 1 {
		return ErrAnswerLibraryInvalid
	}
	if len(ids) == 1 {
		return nil
	}
	if originEntryID > 0 {
		return nil
	}
	var existing int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM answer_library WHERE source_job_id=? AND source_destination=? AND source_question_key=?`, job.ID, job.URL, key).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	now := answerLibraryTimestamp()
	_, err = tx.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,early_discovery_source,early_fingerprint,early_source_question_id,early_answer_revision,question_terms,answer_terms,search_index_version,created_at,updated_at,revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`, question, answer, job.Company, job.Title, job.Host, job.ID, job.URL, browser.PostingPlatform(job.URL), key, revision, 0, "", key, "manual", "manual", "", "", "", 0, answerLibraryIndexTerms(question), answerLibraryIndexTerms(answer), answerLibrarySearchIndexVersion, now, now)
	return err
}

// DELETE /api/jobs/{id}/manual-questions/{key} removes one applicant-authored
// question. Library snapshots and unrelated application answers are untouched.
func (a *App) manualQuestionDeleteAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	job, ok := a.manualJob(w, r)
	if !ok {
		return
	}
	key := r.PathValue("key")
	if !validManualQuestionKey(key) {
		writeResumeError(w, http.StatusNotFound, "Unknown manual question")
		return
	}
	var input manualQuestionDeleteInput
	if !decodeResumeJSON(w, r, 4<<10, &input) {
		return
	}
	if err := a.deleteManualQuestion(job, key, input.Revision); errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusConflict, "This manual question is already gone. Review the current questions.")
		return
	} else if errors.Is(err, errManualQuestionConflict) {
		writeResumeError(w, http.StatusConflict, "This manual question changed in another window. Review it before removing it.")
		return
	} else if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to remove the question")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{"deleted": true, "key": key})
}

// POST /api/jobs/{id}/manual-questions/{key}/starting-point returns the full
// text of one verified Answer Library example as an unsaved draft for a
// manual question. The editor stays unsaved until the applicant explicitly
// saves; nothing here authorizes transfer.
func (a *App) manualQuestionStartingPointAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	job, ok := a.manualJob(w, r)
	if !ok {
		return
	}
	key := r.PathValue("key")
	if !validManualQuestionKey(key) {
		writeResumeError(w, http.StatusNotFound, "Unknown manual question")
		return
	}
	var input manualQuestionStartingPointInput
	if !decodeResumeJSON(w, r, 12<<10, &input) {
		return
	}
	if input.Destination != job.URL {
		writeResumeError(w, http.StatusConflict, "The application destination changed. Review the current question before using previous writing.")
		return
	}
	current, err := a.manualQuestionByKey(job, key)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusConflict, "This manual question changed. Review it before using previous writing.")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read the manual question")
		return
	}
	if input.Revision != current.Revision {
		writeResumeError(w, http.StatusConflict, "This manual question changed. Review it before using previous writing.")
		return
	}
	if input.LibraryEntryID <= 0 || input.LibraryRevision < 1 {
		writeResumeError(w, http.StatusBadRequest, "Review the selected source before using it.")
		return
	}
	if !answerLibraryEligible(manualQuestionField(key, current.Question), true) {
		writeResumeError(w, http.StatusConflict, "This manual question is not eligible for library starting points.")
		return
	}
	entry, err := a.answerLibraryBySourceID(a.db, input.LibraryEntryID)
	if errors.Is(err, sql.ErrNoRows) {
		writeResumeError(w, http.StatusNotFound, "The selected Answer Library example changed or was deleted.")
		return
	}
	if err != nil {
		writeResumeError(w, http.StatusInternalServerError, "Unable to read the selected Answer Library example")
		return
	}
	if entry.Revision != input.LibraryRevision {
		writeResumeError(w, http.StatusConflict, "This Answer Library example changed. Review the updated example before using it.")
		return
	}
	writeResumeJSON(w, http.StatusOK, map[string]any{
		"library_entry_id": entry.ID, "library_revision": entry.Revision,
		"question_key": key, "revision": current.Revision, "answer": entry.Answer,
	})
}

// manualQuestionProjection appends applicant-authored rows after the unified
// observed inventory. Manual rows keep their own keys and are never merged
// with, reconciled against, or overwritten by later capture.
func (a *App) manualQuestionProjection(job Job) ([]writingQuestionJSON, error) {
	rows, err := a.manualQuestions(job)
	if err != nil {
		return nil, err
	}
	out := make([]writingQuestionJSON, 0, len(rows))
	for _, q := range rows {
		row, err := a.manualWritingRow(q)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}
