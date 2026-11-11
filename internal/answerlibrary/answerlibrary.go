// Package answerlibrary owns Answer Library retrieval: search, ranking,
// equivalence, normalization, and index-term helpers.
//
// internal/app retains HTTP routes, JSON contracts, validation, handlers,
// presentation, cross-feature promotion transactions, and shared locks.
// Promotion writers stay in internal/app; this package never creates a second
// writer or fragments transactions. It accepts concrete *sql.DB / *sql.Tx
// dependencies only.
package answerlibrary

const (
	SearchIndexVersion = 1
	CorpusLimit        = 5000
	RelatedLimit       = 3
	SearchPageLimit    = 3
	// LibrarySearchLimit bounds library-wide browse/search only.
	// Ranked application-preparation suggestions stay capped at
	// SearchPageLimit/RelatedLimit.
	LibrarySearchLimit = 50
	LibraryPageSize    = 20
	ExcerptRunes       = 320
)

// Entry is a curated copy of one saved answer, with enough source metadata
// to preserve its provenance after the original job is removed.
type Entry struct {
	Alternatives                []string
	ID                          int64
	OriginalQuestion            string
	Answer                      string
	SourceEmployer              string
	SourceJobTitle              string
	SourceHost                  string
	SourceJobID                 int64
	SourceDestination           string
	SourceProvider              string
	SourceQuestionKey           string
	SourceRevision              int64
	SourceInspectionID          int64
	SourceInspectionFingerprint string
	SourceFieldID               string
	SourceFieldKind             string
	SourceKind                  string
	EarlyDiscoverySource        string
	EarlyFingerprint            string
	EarlySourceQuestionID       string
	EarlyAnswerRevision         int64
	CreatedAt                   string
	UpdatedAt                   string
	Revision                    int64
}

// Suggestion is one ranked library hit projected for presentation.
type Suggestion struct {
	MatchKind        string
	EntryID          int64
	Revision         int64
	OriginalQuestion string
	SourceEmployer   string
	SourceJobTitle   string
	SourceHost       string
	Excerpt          string
}

// SelectColumns lists the answer_library columns owned by entry retrieval.
const SelectColumns = `id,original_question,answer,source_employer,source_job_title,source_host,` +
	` source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,` +
	` source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,early_discovery_source,` +
	` early_fingerprint,early_source_question_id,early_answer_revision,created_at,updated_at,revision`
