package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/schema"
)

const answerLibraryMaxText = 12000

func answerLibraryTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z07:00")
}

var (
	ErrAnswerLibrarySource     = errors.New("saved source could not be verified")
	ErrAnswerLibraryIneligible = errors.New("this question is not eligible for the Answer Library")
	ErrAnswerLibraryStale      = errors.New("saved answer changed")
	ErrAnswerLibraryInvalid    = errors.New("invalid Answer Library edit")
)

// AnswerLibraryEntry is a curated copy of one saved answer, with enough source
// metadata to preserve its provenance after the original job is removed.
type AnswerLibraryEntry struct {
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

func validAnswerLibraryText(value string) bool {
	return len(value) <= answerLibraryMaxText && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

// answerLibraryEligible is deliberately narrower than the local-answer editor:
// only stable, free-text narrative prompts can be promoted. Sensitive flags and
// conservative label checks are barriers, not a claim that arbitrary prose is safe.
func answerLibraryEligible(f schema.Field, unique bool) bool {
	if !stableQuestionIdentity(f, unique) || (f.Kind != "text" && f.Kind != "textarea") {
		return false
	}
	for _, blocked := range answerLibrarySensitivePhrases {
		if answerLibraryContainsPhrase(f.Label, blocked) {
			return false
		}
	}
	label := strings.ToLower(strings.TrimSpace(f.Label))
	words := strings.Fields(label)
	if len(words) < 3 || len(label) < 15 {
		return false
	}
	for _, marker := range []string{
		"describe", "tell us", "share", "give an example", "provide an example", "example of",
		"how have", "how did", "what was", "what would", "what interests", "why do you want",
		"why are you", "walk us through", "discuss", "explain", "elaborate", "challenge",
		"incident", "project", "situation", "time when", "accomplishment", "achievement",
		"proud of", "motivat", "strength", "improvement", "experience with", "experience in",
		// Evidence-backed 2026-11-06 (real Ashby posting): genuine narrative
		// prompts missed by the list above. Each stays clear of the generic
		// "What is your experience?" negative case in TestAnswerLibraryEligibilityIsConservative.
		"improv", "excit", "what problem",
	} {
		if strings.Contains(label, marker) {
			return true
		}
	}
	return false
}

var answerLibrarySensitivePhrases = []string{
	"authorization", "authorized to work", "eligible to work", "work eligibility", "visa", "sponsor", "sponsors",
	"immigration", "citizenship", "citizen", "race", "ethnicity", "gender", "sexual orientation",
	"sponsorship", "work permit", "disability", "disabilities", "accommodation", "accommodations", "veteran", "veterans",
	"date of birth", "birth date", "birth year", "birthday", "how old", "age", "pronoun", "pronouns",
	"religion", "marital status", "medical", "health condition", "health conditions", "salary", "salaries", "compensation",
	"pay expectation", "pay expectations", "desired pay", "hourly rate", "hourly rates", "wage", "wages", "reference", "references", "referee", "referees", "email",
	"phone", "telephone", "street address", "street addresses", "mailing address", "mailing addresses", "password", "passwords", "credential", "credentials", "secret", "secrets",
	"api key", "api keys", "access token", "access tokens", "social security", "ssn", "bank account", "bank accounts", "agree to", "i agree",
	"consent", "attest", "attestation", "attestations", "certify", "certification", "certifications", "legally bound", "background check", "background checks",
}

func answerLibraryContainsPhrase(text, phrase string) bool {
	words, blocked := answerLibraryWords(text), answerLibraryWords(phrase)
	if len(blocked) == 0 || len(blocked) > len(words) {
		return false
	}
	for start := 0; start <= len(words)-len(blocked); start++ {
		match := true
		for offset := range blocked {
			if words[start+offset] != blocked[offset] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// promoteApplicationAnswer validates and snapshots the authoritative saved row
// in one transaction. A later answer revision has a distinct deduplication key
// and cannot overwrite an independently edited library entry.
func (a *App) promoteApplicationAnswer(jobID int64, key string, expectedRevision int64) (AnswerLibraryEntry, bool, error) {
	if jobID <= 0 || !questionReturnKey.MatchString(key) || expectedRevision < 1 {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	tx, err := a.db.Begin()
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	defer tx.Rollback()

	var job Job
	err = tx.QueryRow(`SELECT id,url,host,title,company,stage FROM jobs WHERE id=?`, jobID).
		Scan(&job.ID, &job.URL, &job.Host, &job.Title, &job.Company, &job.Stage)
	if errors.Is(err, sql.ErrNoRows) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	provider := browser.PostingPlatform(job.URL)
	if provider == "" || !browser.OwnsPostingURL(job.URL, job.URL) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryIneligible
	}
	var savedLabel, answer string
	var revision int64
	err = tx.QueryRow(`SELECT label,answer,revision FROM application_answers
	 WHERE job_id=? AND destination=? AND provider=? AND question_key=?`, job.ID, job.URL, provider, key).
		Scan(&savedLabel, &answer, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	if revision != expectedRevision {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryStale
	}
	if strings.TrimSpace(answer) == "" || !validAnswerLibraryText(answer) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryIneligible
	}

	field, inspectionID, inspectionFingerprint, err := verifiedAnswerLibraryField(tx, job, provider, key)
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	if field.Label != savedLabel || !answerLibraryEligible(field, true) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryIneligible
	}

	now := answerLibraryTimestamp()
	entry := AnswerLibraryEntry{
		OriginalQuestion: field.Label, Answer: answer, SourceEmployer: job.Company,
		SourceJobTitle: job.Title, SourceHost: job.Host, SourceJobID: job.ID,
		SourceDestination: job.URL, SourceProvider: provider, SourceQuestionKey: key,
		SourceRevision: revision, SourceInspectionID: inspectionID, SourceInspectionFingerprint: inspectionFingerprint,
		SourceFieldID: field.ID, SourceFieldKind: field.Kind, SourceKind: "live",
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	result, err := tx.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,
	 source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
	 source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,question_terms,answer_terms,search_index_version,created_at,updated_at,revision)
	 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?, ?,1)
	 ON CONFLICT(source_job_id,source_destination,source_provider,source_question_key,source_revision) DO NOTHING`,
		entry.OriginalQuestion, entry.Answer, entry.SourceEmployer, entry.SourceJobTitle, entry.SourceHost,
		entry.SourceJobID, entry.SourceDestination, entry.SourceProvider, entry.SourceQuestionKey,
		entry.SourceRevision, entry.SourceInspectionID, entry.SourceInspectionFingerprint, entry.SourceFieldID, entry.SourceFieldKind, entry.SourceKind,
		answerLibraryIndexTerms(entry.OriginalQuestion), answerLibraryIndexTerms(entry.Answer), answerLibrarySearchIndexVersion,
		entry.CreatedAt, entry.UpdatedAt)
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	created := inserted == 1
	if created {
		entry.ID, err = result.LastInsertId()
		if err != nil {
			return AnswerLibraryEntry{}, false, err
		}
	} else {
		entry, err = answerLibraryEntryBySource(tx, job.ID, job.URL, provider, key, revision)
		if err != nil {
			return AnswerLibraryEntry{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	return entry, created, nil
}

// verifiedAnswerLibraryField reconstructs an observed field from saved local
// inspection rows and checks the exact same question key used by the workbench.
// It reads at most the 100 most recent inspections for this provider/job.
func verifiedAnswerLibraryField(tx *sql.Tx, job Job, provider, key string) (schema.Field, int64, string, error) {
	rows, err := tx.Query(`SELECT i.id,i.application_url,i.fingerprint,f.field_id,f.label,f.kind,f.required,
	 f.sensitive,f.group_name,f.options_json
	 FROM application_inspections i JOIN application_fields f ON f.inspection_id=i.id
	 WHERE i.id IN (SELECT id FROM application_inspections WHERE job_id=? AND platform=? AND source='browser'
	  ORDER BY checked_at DESC,id DESC LIMIT 100)
	 ORDER BY i.checked_at DESC,i.id DESC,f.ordinal`, job.ID, provider)
	if err != nil {
		return schema.Field{}, 0, "", err
	}
	defer rows.Close()
	type observed struct {
		id                          int64
		applicationURL, fingerprint string
		field                       schema.Field
	}
	var group []observed
	var groupID int64
	inspect := func(items []observed) (schema.Field, int64, string, bool, error) {
		if len(items) == 0 || !browser.OwnsPostingURL(job.URL, items[0].applicationURL) {
			return schema.Field{}, 0, "", false, nil
		}
		counts := make(map[string]int, len(items))
		for _, item := range items {
			counts[item.field.ID]++
		}
		for _, item := range items {
			if questionKey(job.URL, provider, item.field) == key {
				return item.field, item.id, item.fingerprint, counts[item.field.ID] == 1, nil
			}
		}
		return schema.Field{}, 0, "", false, nil
	}
	for rows.Next() {
		var item observed
		var required, sensitive bool
		var options string
		if err := rows.Scan(&item.id, &item.applicationURL, &item.fingerprint,
			&item.field.ID, &item.field.Label, &item.field.Kind, &required, &sensitive,
			&item.field.Group, &options); err != nil {
			return schema.Field{}, 0, "", err
		}
		item.field.Required, item.field.Sensitive = required, sensitive
		if err := json.Unmarshal([]byte(options), &item.field.Options); err != nil {
			return schema.Field{}, 0, "", fmt.Errorf("read observed question options: %w", err)
		}
		if groupID != 0 && item.id != groupID {
			field, id, fingerprint, unique, err := inspect(group)
			if err != nil {
				return schema.Field{}, 0, "", err
			}
			if id != 0 {
				if !unique {
					return schema.Field{}, 0, "", ErrAnswerLibraryIneligible
				}
				return field, id, fingerprint, nil
			}
			group = group[:0]
		}
		groupID = item.id
		group = append(group, item)
	}
	if err := rows.Err(); err != nil {
		return schema.Field{}, 0, "", err
	}
	field, id, fingerprint, unique, err := inspect(group)
	if err != nil {
		return schema.Field{}, 0, "", err
	}
	if id == 0 {
		return schema.Field{}, 0, "", ErrAnswerLibrarySource
	}
	if !unique {
		return schema.Field{}, 0, "", ErrAnswerLibraryIneligible
	}
	return field, id, fingerprint, nil
}

func scanAnswerLibraryEntry(scanner interface {
	Scan(...any) error
}, entry *AnswerLibraryEntry) error {
	return scanner.Scan(
		&entry.ID, &entry.OriginalQuestion, &entry.Answer, &entry.SourceEmployer, &entry.SourceJobTitle,
		&entry.SourceHost, &entry.SourceJobID, &entry.SourceDestination, &entry.SourceProvider,
		&entry.SourceQuestionKey, &entry.SourceRevision, &entry.SourceInspectionID, &entry.SourceInspectionFingerprint, &entry.SourceFieldID,
		&entry.SourceFieldKind, &entry.SourceKind, &entry.EarlyDiscoverySource, &entry.EarlyFingerprint,
		&entry.EarlySourceQuestionID, &entry.EarlyAnswerRevision, &entry.CreatedAt, &entry.UpdatedAt, &entry.Revision)
}

const answerLibrarySelectColumns = `id,original_question,answer,source_employer,source_job_title,source_host,
	 source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
	 source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,early_discovery_source,
	 early_fingerprint,early_source_question_id,early_answer_revision,created_at,updated_at,revision`

func answerLibraryEntryBySource(queryer interface {
	QueryRow(string, ...any) *sql.Row
}, jobID int64, destination, provider, key string, sourceRevision int64) (AnswerLibraryEntry, error) {
	var entry AnswerLibraryEntry
	err := scanAnswerLibraryEntry(queryer.QueryRow(`SELECT `+answerLibrarySelectColumns+` FROM answer_library
	 WHERE source_job_id=? AND source_destination=? AND source_provider=? AND source_question_key=? AND source_revision=? AND source_kind='live'`,
		jobID, destination, provider, key, sourceRevision), &entry)
	return entry, err
}

func (a *App) answerLibraryEntries() ([]AnswerLibraryEntry, error) {
	rows, err := a.db.Query(`SELECT ` + answerLibrarySelectColumns + ` FROM answer_library ORDER BY updated_at DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []AnswerLibraryEntry
	for rows.Next() {
		var entry AnswerLibraryEntry
		if err := scanAnswerLibraryEntry(rows, &entry); err != nil {
			return nil, err
		}
		if entry.SourceKind == "" {
			entry.SourceKind = "live"
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (a *App) answerLibraryCount() (int64, error) {
	var total int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM answer_library`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (a *App) answerLibraryAltCounts(ids []int64) (map[int64]int, error) {
	counts := make(map[int64]int, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := a.db.Query(`SELECT entry_id,COUNT(*) FROM answer_library_alternatives WHERE entry_id IN (`+strings.Join(placeholders, ",")+`) GROUP BY entry_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	return counts, rows.Err()
}

// answerLibraryListPage serves the library-wide browse list only. Sort is
// "recent" (updated_at DESC, id DESC) or "alpha" (question A-Z). Paging is
// offset-based with a bounded page size; the ranked application suggestions
// keep their own top-3 path and are unaffected.
func (a *App) answerLibraryListPage(sort string, page, perPage int) ([]AnswerLibraryEntry, int64, error) {
	if perPage < 1 || perPage > answerLibraryLibrarySearchLimit {
		perPage = answerLibraryLibraryPageSize
	}
	if page < 1 {
		page = 1
	}
	total, err := a.answerLibraryCount()
	if err != nil {
		return nil, 0, err
	}
	order := `ORDER BY updated_at DESC,id DESC`
	if sort == "alpha" {
		order = `ORDER BY original_question COLLATE NOCASE ASC,id ASC`
	}
	rows, err := a.db.Query(`SELECT `+answerLibrarySelectColumns+` FROM answer_library `+order+` LIMIT ? OFFSET ?`, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var entries []AnswerLibraryEntry
	for rows.Next() {
		var entry AnswerLibraryEntry
		if err := scanAnswerLibraryEntry(rows, &entry); err != nil {
			return nil, 0, err
		}
		if entry.SourceKind == "" {
			entry.SourceKind = "live"
		}
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

func (a *App) answerLibraryBySourceID(queryer interface {
	QueryRow(string, ...any) *sql.Row
}, id int64) (AnswerLibraryEntry, error) {
	var entry AnswerLibraryEntry
	err := scanAnswerLibraryEntry(queryer.QueryRow(`SELECT `+answerLibrarySelectColumns+` FROM answer_library WHERE id=?`, id), &entry)
	if err == nil && entry.SourceKind == "" {
		entry.SourceKind = "live"
	}
	return entry, err
}

// promoteEarlyApplicationAnswer curates a saved early narrative as a phrasing
// example only. It never inserts application_answers, never marks
// reconciliation accepted, never approves exact memory, and never authorizes
// Fill. The library entry uses source_kind='early' with a documented absent
// browser inspection (inspection ID 0) and a distinct deduplication namespace
// so live and early identities cannot collide.
func (a *App) promoteEarlyApplicationAnswer(jobID int64, key string, expectedRevision int64) (AnswerLibraryEntry, bool, error) {
	if jobID <= 0 || !questionReturnKey.MatchString(key) || expectedRevision < 1 {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	job, err := a.jobByID(jobID)
	if err != nil {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	if job.Stage == StageApplied || job.Stage == StageClosed || job.SubmissionCount > 0 {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	early, err := a.prepareEarlyQuestions(job, nil)
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	var match *earlyApplicationQuestion
	for i := range early.Questions {
		if early.Questions[i].Key == key && early.Questions[i].Revision == expectedRevision {
			candidate := early.Questions[i]
			match = &candidate
			break
		}
	}
	if match == nil {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryStale
	}
	if !match.Editable || match.TriageCategory != "answer" || !earlyPotentialNarrative(match.Field) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryIneligible
	}
	if strings.TrimSpace(match.Value) == "" || !validAnswerLibraryText(match.Value) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryIneligible
	}
	if match.DiscoverySource == "" || match.DiscoveryFingerprint == "" || match.SourceQuestionID == "" {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	tx, err := a.db.Begin()
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	defer tx.Rollback()
	// Revalidate the saved early revision transactionally: the question row
	// must still hold the expected answer.
	var savedAnswer string
	var savedRevision int64
	err = tx.QueryRow(`SELECT answer,revision FROM early_application_answers
	 WHERE job_id=? AND destination=? AND discovery_source=? AND discovery_fingerprint=? AND source_question_id=?`,
		job.ID, job.URL, match.DiscoverySource, match.DiscoveryFingerprint, match.SourceQuestionID).Scan(&savedAnswer, &savedRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return AnswerLibraryEntry{}, false, ErrAnswerLibrarySource
	}
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	if savedRevision != expectedRevision || savedAnswer != match.Value {
		return AnswerLibraryEntry{}, false, ErrAnswerLibraryStale
	}
	now := answerLibraryTimestamp()
	// Early deduplication uses a distinct namespace so live and early source
	// revisions cannot collide: provider is prefixed and inspection is absent.
	entry := AnswerLibraryEntry{
		OriginalQuestion: match.Field.Label, Answer: savedAnswer, SourceEmployer: job.Company,
		SourceJobTitle: job.Title, SourceHost: job.Host, SourceJobID: job.ID,
		SourceDestination: job.URL, SourceProvider: "early:" + match.DiscoverySource, SourceQuestionKey: key,
		SourceRevision: savedRevision, SourceInspectionID: 0, SourceInspectionFingerprint: match.DiscoveryFingerprint,
		SourceFieldID: match.SourceQuestionID, SourceFieldKind: match.Field.Kind, SourceKind: "early",
		EarlyDiscoverySource: match.DiscoverySource, EarlyFingerprint: match.DiscoveryFingerprint,
		EarlySourceQuestionID: match.SourceQuestionID, EarlyAnswerRevision: savedRevision,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	result, err := tx.Exec(`INSERT INTO answer_library(original_question,answer,source_employer,source_job_title,source_host,
	 source_job_id,source_destination,source_provider,source_question_key,source_revision,source_inspection_id,
	 source_inspection_fingerprint,source_field_id,source_field_kind,source_kind,early_discovery_source,early_fingerprint,early_source_question_id,early_answer_revision,question_terms,answer_terms,search_index_version,created_at,updated_at,revision)
	 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?, ?,1)
	 ON CONFLICT(source_job_id,source_destination,source_provider,source_question_key,source_revision) DO NOTHING`,
		entry.OriginalQuestion, entry.Answer, entry.SourceEmployer, entry.SourceJobTitle, entry.SourceHost,
		entry.SourceJobID, entry.SourceDestination, entry.SourceProvider, entry.SourceQuestionKey,
		entry.SourceRevision, entry.SourceInspectionID, entry.SourceInspectionFingerprint, entry.SourceFieldID, entry.SourceFieldKind, entry.SourceKind,
		entry.EarlyDiscoverySource, entry.EarlyFingerprint, entry.EarlySourceQuestionID, entry.EarlyAnswerRevision,
		answerLibraryIndexTerms(entry.OriginalQuestion), answerLibraryIndexTerms(entry.Answer), answerLibrarySearchIndexVersion,
		entry.CreatedAt, entry.UpdatedAt)
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	created := inserted == 1
	if created {
		entry.ID, err = result.LastInsertId()
		if err != nil {
			return AnswerLibraryEntry{}, false, err
		}
	} else {
		err = scanAnswerLibraryEntry(tx.QueryRow(`SELECT `+answerLibrarySelectColumns+` FROM answer_library
		 WHERE source_job_id=? AND source_destination=? AND source_provider=? AND source_question_key=? AND source_revision=?`,
			job.ID, job.URL, entry.SourceProvider, key, savedRevision), &entry)
		if err != nil {
			return AnswerLibraryEntry{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return AnswerLibraryEntry{}, false, err
	}
	return entry, created, nil
}

func (a *App) updateAnswerLibrary(id, expectedRevision int64, answer string) error {
	return a.updateAnswerLibraryWithAlternatives(id, expectedRevision, answer, nil)
}

func (a *App) updateAnswerLibraryWithAlternatives(id, expectedRevision int64, answer string, alternatives []string) error {
	if id <= 0 || expectedRevision < 1 || !validAnswerLibraryText(answer) || strings.TrimSpace(answer) == "" {
		return ErrAnswerLibraryInvalid
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE answer_library SET answer=?,answer_terms=?,search_index_version=?,updated_at=?,revision=revision+1 WHERE id=? AND revision=?`,
		answer, answerLibraryIndexTerms(answer), answerLibrarySearchIndexVersion, answerLibraryTimestamp(), id, expectedRevision)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 1 {
		if alternatives != nil {
			if len(alternatives) > 30 {
				return ErrAnswerLibraryInvalid
			}
			if _, err := tx.Exec("DELETE FROM answer_library_alternatives WHERE entry_id=?", id); err != nil {
				return err
			}
			for _, phrase := range alternatives {
				phrase = strings.TrimSpace(phrase)
				if phrase == "" {
					continue
				}
				if len(phrase) > 1000 || !validAnswerLibraryText(phrase) {
					return ErrAnswerLibraryInvalid
				}
				if _, err := tx.Exec("INSERT OR IGNORE INTO answer_library_alternatives(entry_id,phrasing,normalized) VALUES(?,?,?)", id, phrase, normalizeLibraryQuestion(phrase)); err != nil {
					return err
				}
			}
		}
		return tx.Commit()
	}
	var exists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM answer_library WHERE id=?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return ErrAnswerLibraryStale
}

func (a *App) deleteAnswerLibrary(id, expectedRevision int64) error {
	if id <= 0 || expectedRevision < 1 {
		return ErrAnswerLibraryInvalid
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM answer_library WHERE id=? AND revision=?`, id, expectedRevision)
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
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM answer_library WHERE id=?)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return ErrAnswerLibraryStale
}
