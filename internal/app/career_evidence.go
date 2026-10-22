package app

import (
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	careerEvidenceTitleRunes          = 160
	careerEvidenceSituationRunes      = 6000
	careerEvidenceActionsRunes        = 8000
	careerEvidenceContextRunes        = 240
	careerEvidenceTechnologiesRunes   = 1200
	careerEvidenceOutcomeRunes        = 3000
	careerEvidenceSourceContextRunes  = 2000
	careerEvidencePeriodRunes         = 120
	careerEvidenceSourceNotesRunes    = 3000
	careerEvidenceSourceQuestionRunes = 4000
)

var (
	ErrCareerEvidenceStale          = errors.New("career evidence changed")
	ErrCareerEvidenceInvalid        = errors.New("invalid career evidence")
	ErrCareerEvidenceIncomplete     = errors.New("career evidence needs a title, situation, and actions before review")
	ErrCareerEvidenceOutcomeMissing = errors.New("a recorded outcome needs its actual description")
)

// CareerEvidence records applicant-curated experience independently from
// employer questions and application answers. Reviewed means applicant reviewed.
type CareerEvidence struct {
	ID                   int64
	Title                string
	Situation            string
	Actions              string
	EmployerContext      string
	RoleContext          string
	Technologies         string
	Outcome              string
	OutcomeState         string
	Period               string
	SourceNotes          string
	SourceLibraryEntryID int64
	SourceQuestion       string
	SourceWriting        string
	SourceEmployer       string
	SourceJobTitle       string
	SourceHost           string
	Status               string
	Revision             int64
	CreatedAt            string
	UpdatedAt            string
}

type careerEvidenceContent struct {
	Title, Situation, Actions                  string
	EmployerContext, RoleContext, Technologies string
	Outcome, OutcomeState, Period, SourceNotes string
}

func careerEvidenceValidText(value string, maxRunes int) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && utf8.RuneCountInString(value) <= maxRunes
}

func (content careerEvidenceContent) valid() bool {
	return careerEvidenceValidText(content.Title, careerEvidenceTitleRunes) &&
		careerEvidenceValidText(content.Situation, careerEvidenceSituationRunes) &&
		careerEvidenceValidText(content.Actions, careerEvidenceActionsRunes) &&
		careerEvidenceValidText(content.EmployerContext, careerEvidenceContextRunes) &&
		careerEvidenceValidText(content.RoleContext, careerEvidenceContextRunes) &&
		careerEvidenceValidText(content.Technologies, careerEvidenceTechnologiesRunes) &&
		careerEvidenceValidText(content.Outcome, careerEvidenceOutcomeRunes) &&
		(content.OutcomeState == "not_recorded" || content.OutcomeState == "unknown" || content.OutcomeState == "recorded") &&
		careerEvidenceValidText(content.Period, careerEvidencePeriodRunes) &&
		careerEvidenceValidText(content.SourceNotes, careerEvidenceSourceNotesRunes)
}

func (e CareerEvidence) content() careerEvidenceContent {
	return careerEvidenceContent{
		Title: e.Title, Situation: e.Situation, Actions: e.Actions,
		EmployerContext: e.EmployerContext, RoleContext: e.RoleContext,
		Technologies: e.Technologies, Outcome: e.Outcome, OutcomeState: e.OutcomeState, Period: e.Period,
		SourceNotes: e.SourceNotes,
	}
}

func (content careerEvidenceContent) equal(other careerEvidenceContent) bool {
	return content == other
}

type careerEvidenceQueryer interface {
	QueryRow(string, ...any) *sql.Row
}

const careerEvidenceSelect = `SELECT id,title,situation,actions,employer_context,role_context,technologies,outcome,outcome_state,period,source_notes,
	source_library_entry_id,source_question,source_writing,source_employer,source_job_title,source_host,status,revision,created_at,updated_at
	FROM career_evidence`

func careerEvidenceByID(queryer careerEvidenceQueryer, id int64) (CareerEvidence, error) {
	var evidence CareerEvidence
	err := queryer.QueryRow(careerEvidenceSelect+` WHERE id=?`, id).Scan(
		&evidence.ID, &evidence.Title, &evidence.Situation, &evidence.Actions,
		&evidence.EmployerContext, &evidence.RoleContext, &evidence.Technologies,
		&evidence.Outcome, &evidence.OutcomeState, &evidence.Period, &evidence.SourceNotes,
		&evidence.SourceLibraryEntryID, &evidence.SourceQuestion, &evidence.SourceWriting,
		&evidence.SourceEmployer, &evidence.SourceJobTitle, &evidence.SourceHost,
		&evidence.Status, &evidence.Revision, &evidence.CreatedAt, &evidence.UpdatedAt)
	return evidence, err
}

func (a *App) careerEvidenceEntries() ([]CareerEvidence, error) {
	rows, err := a.db.Query(careerEvidenceSelect + ` ORDER BY updated_at DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []CareerEvidence
	for rows.Next() {
		var evidence CareerEvidence
		if err := rows.Scan(
			&evidence.ID, &evidence.Title, &evidence.Situation, &evidence.Actions,
			&evidence.EmployerContext, &evidence.RoleContext, &evidence.Technologies,
			&evidence.Outcome, &evidence.OutcomeState, &evidence.Period, &evidence.SourceNotes,
			&evidence.SourceLibraryEntryID, &evidence.SourceQuestion, &evidence.SourceWriting,
			&evidence.SourceEmployer, &evidence.SourceJobTitle, &evidence.SourceHost,
			&evidence.Status, &evidence.Revision, &evidence.CreatedAt, &evidence.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, evidence)
	}
	return entries, rows.Err()
}

func (a *App) createCareerEvidenceDraft() (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := a.db.Exec(`INSERT INTO career_evidence(status,revision,created_at,updated_at) VALUES('draft',1,?,?)`, now, now)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// createCareerEvidenceFromLibrary keeps a source snapshot and deduplicates an
// accidental repeat while the first evidence record still exists.
func (a *App) createCareerEvidenceFromLibrary(libraryID int64) (int64, error) {
	if libraryID < 1 {
		return 0, ErrCareerEvidenceInvalid
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var question, writing, employer, jobTitle, host, sourceCreatedAt string
	err = tx.QueryRow(`SELECT original_question,answer,source_employer,source_job_title,source_host,created_at FROM answer_library WHERE id=?`, libraryID).
		Scan(&question, &writing, &employer, &jobTitle, &host, &sourceCreatedAt)
	if err != nil {
		return 0, err
	}
	var evidenceID int64
	err = tx.QueryRow(`SELECT id FROM career_evidence WHERE source_library_entry_id=? AND source_library_created_at=?`, libraryID, sourceCreatedAt).Scan(&evidenceID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return evidenceID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if sourceCreatedAt == "" || !careerEvidenceValidText(question, careerEvidenceSourceQuestionRunes) || !validAnswerLibraryText(writing) ||
		!careerEvidenceValidText(employer, careerEvidenceSourceContextRunes) || !careerEvidenceValidText(jobTitle, careerEvidenceSourceContextRunes) ||
		!careerEvidenceValidText(host, careerEvidenceSourceContextRunes) {
		return 0, ErrCareerEvidenceInvalid
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.Exec(`INSERT INTO career_evidence(source_library_entry_id,source_library_created_at,source_question,source_writing,source_employer,source_job_title,source_host,status,revision,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,'draft',1,?,?) ON CONFLICT DO NOTHING`, libraryID, sourceCreatedAt, question, writing, employer, jobTitle, host, now, now)
	if err != nil {
		return 0, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if inserted == 1 {
		evidenceID, err = result.LastInsertId()
	} else {
		err = tx.QueryRow(`SELECT id FROM career_evidence WHERE source_library_entry_id=? AND source_library_created_at=?`, libraryID, sourceCreatedAt).Scan(&evidenceID)
	}
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return evidenceID, nil
}

func (a *App) updateCareerEvidence(id, expectedRevision int64, content careerEvidenceContent, review bool) error {
	if id < 1 || expectedRevision < 1 || !content.valid() {
		return ErrCareerEvidenceInvalid
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := careerEvidenceByID(tx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	if current.Revision != expectedRevision {
		return ErrCareerEvidenceStale
	}
	if review && (strings.TrimSpace(content.Title) == "" || strings.TrimSpace(content.Situation) == "" || strings.TrimSpace(content.Actions) == "") {
		return ErrCareerEvidenceIncomplete
	}
	if review && content.OutcomeState == "recorded" && strings.TrimSpace(content.Outcome) == "" {
		return ErrCareerEvidenceOutcomeMissing
	}
	changed := !content.equal(current.content())
	status := current.Status
	if review {
		status = "reviewed"
	} else if changed {
		status = "draft"
	}
	if !changed && status == current.Status {
		return tx.Commit()
	}
	result, err := tx.Exec(`UPDATE career_evidence SET title=?,situation=?,actions=?,employer_context=?,role_context=?,technologies=?,outcome=?,outcome_state=?,period=?,source_notes=?,status=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`,
		content.Title, content.Situation, content.Actions, content.EmployerContext, content.RoleContext, content.Technologies,
		content.Outcome, content.OutcomeState, content.Period, content.SourceNotes, status, time.Now().UTC().Format(time.RFC3339Nano), id, expectedRevision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrCareerEvidenceStale
	}
	return tx.Commit()
}

func (a *App) deleteCareerEvidence(id, expectedRevision int64) error {
	if id < 1 || expectedRevision < 1 {
		return ErrCareerEvidenceInvalid
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM career_evidence WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		return tx.Commit()
	}
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM career_evidence WHERE id=?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return ErrCareerEvidenceStale
}
