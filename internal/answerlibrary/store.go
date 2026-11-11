package answerlibrary

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Queryer is the existing codebase idiom for corpus reads satisfied by
// *sql.DB and *sql.Tx.
type Queryer interface {
	Query(string, ...any) (*sql.Rows, error)
}

// RowQueryer is the existing codebase idiom for single-entry reads
// satisfied by *sql.DB and *sql.Tx.
type RowQueryer interface {
	QueryRow(string, ...any) *sql.Row
}

// EquivalentIDs resolves exact-equivalence entry IDs for one question label.
// Only case, surrounding whitespace, and repeated whitespace are superficial.
func EquivalentIDs(q Queryer, question string) ([]int64, error) {
	rows, err := q.Query(`SELECT id,original_question FROM answer_library UNION ALL SELECT entry_id,phrasing FROM answer_library_alternatives`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	seen := map[int64]bool{}
	normalized := Normalize(question)
	for rows.Next() {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			return nil, err
		}
		if Normalize(label) == normalized && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, rows.Err()
}

// ScanEntry scans one entry row in SelectColumns order.
func ScanEntry(scanner interface {
	Scan(...any) error
}, entry *Entry) error {
	return scanner.Scan(
		&entry.ID, &entry.OriginalQuestion, &entry.Answer, &entry.SourceEmployer, &entry.SourceJobTitle,
		&entry.SourceHost, &entry.SourceJobID, &entry.SourceDestination, &entry.SourceProvider,
		&entry.SourceQuestionKey, &entry.SourceRevision, &entry.SourceInspectionID, &entry.SourceInspectionFingerprint, &entry.SourceFieldID,
		&entry.SourceFieldKind, &entry.SourceKind, &entry.EarlyDiscoverySource, &entry.EarlyFingerprint,
		&entry.EarlySourceQuestionID, &entry.EarlyAnswerRevision, &entry.CreatedAt, &entry.UpdatedAt, &entry.Revision)
}

// EntryByID loads one curated entry by ID.
func EntryByID(q RowQueryer, id int64) (Entry, error) {
	var entry Entry
	err := ScanEntry(q.QueryRow(`SELECT `+SelectColumns+` FROM answer_library WHERE id=?`, id), &entry)
	if err == nil && entry.SourceKind == "" {
		entry.SourceKind = "live"
	}
	return entry, err
}

// Alternatives loads user-defined alternative phrasings for one entry.
func Alternatives(db *sql.DB, id int64) ([]string, error) {
	rows, err := db.Query(`SELECT phrasing FROM answer_library_alternatives WHERE entry_id=? ORDER BY rowid`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LoadDocuments loads the ranked-search corpus from the curated table only.
func LoadDocuments(db *sql.DB) ([]SearchDocument, error) {
	rows, err := db.Query(`SELECT id,revision,original_question,source_employer,source_job_title,source_host,updated_at,question_terms,answer_terms
	 FROM answer_library WHERE search_index_version=? ORDER BY id DESC LIMIT ?`, SearchIndexVersion, CorpusLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []SearchDocument
	for rows.Next() {
		var doc SearchDocument
		var questionTerms, answerTerms string
		if err := rows.Scan(&doc.EntryID, &doc.Revision, &doc.OriginalQuestion, &doc.SourceEmployer, &doc.SourceJobTitle,
			&doc.SourceHost, &doc.UpdatedAt, &questionTerms, &answerTerms); err != nil {
			return nil, err
		}
		doc.QuestionTerms = TermSet(questionTerms, doc.OriginalQuestion)
		doc.AnswerTerms = TermSet(answerTerms, "")
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return docs, nil
	}
	altRows, err := db.Query(`SELECT entry_id,phrasing FROM answer_library_alternatives`)
	if err != nil {
		return nil, err
	}
	defer altRows.Close()
	byEntry := make(map[int64][]string)
	for altRows.Next() {
		var id int64
		var phrase string
		if err := altRows.Scan(&id, &phrase); err != nil {
			return nil, err
		}
		byEntry[id] = append(byEntry[id], phrase)
	}
	if err := altRows.Err(); err != nil {
		return nil, err
	}
	for i := range docs {
		docs[i].AltTerms = TermSet(IndexTerms(strings.Join(byEntry[docs[i].EntryID], "\n")), "")
		docs[i].SourceTerms = TermSet("", docs[i].SourceEmployer+" "+docs[i].SourceJobTitle+
			" "+docs[i].SourceHost)
	}
	return docs, nil
}

// Excerpts projects ranked documents to presentation suggestions.
func Excerpts(db *sql.DB, ranked []RankedDocument) ([]Suggestion, error) {
	if len(ranked) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ranked))
	args := make([]any, len(ranked))
	for i, item := range ranked {
		placeholders[i] = "?"
		args[i] = item.Document.EntryID
	}
	rows, err := db.Query(`SELECT id,revision,substr(answer,1,`+fmt.Sprint(ExcerptRunes)+`) FROM answer_library WHERE id IN (`+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, err
	}
	excerpts := make(map[int64]string, len(ranked))
	revisions := make(map[int64]int64, len(ranked))
	for rows.Next() {
		var id int64
		var revision int64
		var excerpt string
		if err := rows.Scan(&id, &revision, &excerpt); err != nil {
			rows.Close()
			return nil, err
		}
		excerpts[id] = Excerpt(excerpt)
		revisions[id] = revision
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	results := make([]Suggestion, 0, len(ranked))
	for _, item := range ranked {
		doc := item.Document
		results = append(results, Suggestion{EntryID: doc.EntryID, Revision: revisions[doc.EntryID], OriginalQuestion: doc.OriginalQuestion,
			SourceEmployer: doc.SourceEmployer, SourceJobTitle: doc.SourceJobTitle, SourceHost: doc.SourceHost, Excerpt: excerpts[doc.EntryID]})
	}
	return results, nil
}

// Search ranks the curated corpus for one application-preparation query.
func Search(db *sql.DB, query string, limit int) ([]Suggestion, error) {
	docs, err := LoadDocuments(db)
	if err != nil {
		return nil, err
	}
	ranked := RankDocuments(query, docs, limit)
	return Excerpts(db, ranked)
}

// SearchPaged serves library-wide browse/search only. It ranks with the
// higher library cap and merges exact-equivalence matches first so
// user-defined alternative phrasings always surface even when the ranker
// threshold would skip them. Ranked application suggestions keep using
// Search with the 3-result cap.
func SearchPaged(db *sql.DB, query string, limit int) ([]Suggestion, error) {
	if limit < 1 {
		limit = LibrarySearchLimit
	}
	if limit > LibrarySearchLimit {
		limit = LibrarySearchLimit
	}
	docs, err := LoadDocuments(db)
	if err != nil {
		return nil, err
	}
	ranked := RankDocumentsWithLimit(query, docs, limit, LibrarySearchLimit, true)
	excerpts, err := Excerpts(db, ranked)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(excerpts))
	for _, item := range excerpts {
		seen[item.EntryID] = true
	}
	ids, err := EquivalentIDs(db, query)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		placeholders := make([]string, 0, len(ids))
		args := make([]any, 0, len(ids))
		for _, id := range ids {
			if seen[id] {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		if len(placeholders) > 0 {
			rows, err := db.Query(`SELECT id,revision,original_question,source_employer,source_job_title,source_host,substr(answer,1,`+strconv.Itoa(ExcerptRunes)+`) FROM answer_library WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
			if err != nil {
				return nil, err
			}
			var exact []Suggestion
			for rows.Next() {
				var item Suggestion
				var excerpt string
				if err := rows.Scan(&item.EntryID, &item.Revision, &item.OriginalQuestion, &item.SourceEmployer, &item.SourceJobTitle, &item.SourceHost, &excerpt); err != nil {
					rows.Close()
					return nil, err
				}
				item.MatchKind = "Equivalent question — review and use"
				item.Excerpt = Excerpt(excerpt)
				exact = append(exact, item)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
			excerpts = append(exact, excerpts...)
			if len(excerpts) > limit {
				excerpts = excerpts[:limit]
			}
		}
	}
	return excerpts, nil
}

// IndexUnindexedEntries backfills only explicitly curated entries created by
// the pre-search schema. It never reads application_answers.
func IndexUnindexedEntries(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id,original_question,answer FROM answer_library WHERE search_index_version<>?`, SearchIndexVersion)
	if err != nil {
		return err
	}
	type oldEntry struct {
		id               int64
		question, answer string
	}
	var old []oldEntry
	for rows.Next() {
		var entry oldEntry
		if err := rows.Scan(&entry.id, &entry.question, &entry.answer); err != nil {
			rows.Close()
			return err
		}
		old = append(old, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, entry := range old {
		if _, err := tx.Exec(`UPDATE answer_library SET question_terms=?,answer_terms=?,search_index_version=? WHERE id=?`,
			IndexTerms(entry.question), IndexTerms(entry.answer), SearchIndexVersion, entry.id); err != nil {
			return fmt.Errorf("index curated Answer Library entry: %w", err)
		}
	}
	return tx.Commit()
}
