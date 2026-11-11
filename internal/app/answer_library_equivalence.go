package app

import (
	"database/sql"

	"github.com/jenksed/yourmove-go/internal/answerlibrary"
	"github.com/jenksed/yourmove-go/internal/application/browser"
	"strings"
)

// libraryAutoSource marks preparation values automatically supplied from a
// single equivalent Answer Library entry. It is presentation-only: no durable
// application answer is created until the applicant explicitly saves, and it
// never authorizes employer-side transfer.
const libraryAutoSource = "Answer Library — review before saving"

// normalizeLibraryQuestion canonicalizes a question for exact equivalence.
// Delegates to internal/answerlibrary; punctuation and word order remain
// significant with no stemming or synonyms.
func normalizeLibraryQuestion(s string) string {
	return answerlibrary.Normalize(s)
}

// libraryAutoForLabel returns the single equivalent library entry for exact
// automatic preparation, or ok=false when there is none or when equivalents
// conflict and require applicant selection. Related but unapproved wording
// never auto-populates; it stays a ranked suggestion.
func (a *App) libraryAutoForLabel(label string) (entry AnswerLibraryEntry, ok bool) {
	if strings.TrimSpace(label) == "" {
		return AnswerLibraryEntry{}, false
	}
	ids, err := equivalentLibraryIDs(a.db, label)
	if err != nil || len(ids) != 1 {
		return AnswerLibraryEntry{}, false
	}
	entry, err = a.answerLibraryBySourceID(a.db, ids[0])
	if err != nil || strings.TrimSpace(entry.Answer) == "" {
		return AnswerLibraryEntry{}, false
	}
	return entry, true
}

func (a *App) libraryAlternatives(id int64) ([]string, error) {
	return answerlibrary.Alternatives(a.db, id)
}

func equivalentLibraryIDs(tx interface {
	Query(string, ...any) (*sql.Rows, error)
}, question string) ([]int64, error) {
	return answerlibrary.EquivalentIDs(tx, question)
}

// Called within the application's save transaction. Existing equivalents are
// never overwritten or duplicated, even when the applicant adapts the answer.
func saveWritingLibrary(tx *sql.Tx, job Job, input writingSaveInput, label, kind string, eligible bool) error {
	if !eligible || strings.TrimSpace(input.Answer) == "" || !validAnswerLibraryText(input.Answer) {
		return ErrAnswerLibraryIneligible
	}
	ids, err := equivalentLibraryIDs(tx, label)
	if err != nil {
		return err
	}
	if len(ids) > 1 {
		return ErrAnswerLibraryInvalid
	}
	if len(ids) == 1 {
		return nil
	}
	// A reviewed source is already reusable, even for related wording.
	if input.OriginEntryID > 0 {
		return nil
	}
	var existing int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM answer_library WHERE source_job_id=? AND source_destination=? AND source_question_key=?`, job.ID, job.URL, input.QuestionKey).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	now := answerLibraryTimestamp()
	provider := browser.PostingPlatform(job.URL)
	sourceKind := "live"
	if input.Early {
		provider = "early:" + input.DiscoverySource
		sourceKind = "early"
	}
	var inspectionID int64
	fingerprint, fieldID := input.DiscoveryPrint, input.SourceQuestionID
	if !input.Early {
		field, id, fp, err := verifiedAnswerLibraryField(tx, job, provider, input.QuestionKey)
		if err != nil {
			return err
		}
		if field.Label != label || !answerLibraryEligible(field, true) {
			return ErrAnswerLibraryIneligible
		}
		inspectionID, fingerprint, fieldID = id, fp, field.ID
	}
	_, err = tx.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,early_discovery_source,early_fingerprint,early_source_question_id,early_answer_revision,question_terms,answer_terms,search_index_version,created_at,updated_at,revision) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`, label, input.Answer, job.Company, job.Title, job.Host, job.ID, job.URL, provider, input.QuestionKey, input.Revision+1, inspectionID, fingerprint, fieldID, kind, sourceKind, input.DiscoverySource, input.DiscoveryPrint, input.SourceQuestionID, input.Revision+1, answerLibraryIndexTerms(label), answerLibraryIndexTerms(input.Answer), answerLibrarySearchIndexVersion, now, now)
	return err
}
