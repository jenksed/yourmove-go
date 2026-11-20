package app

import (
	"database/sql"

	"github.com/jenksed/yourmove-go/internal/answerlibrary"
)

// Retrieval operations delegate to internal/answerlibrary. Historical
// in-package names are aliased in answer_library_search_shim.go.

func (a *App) answerLibrarySearchDocuments() ([]answerLibrarySearchDocument, error) {
	return answerlibrary.LoadDocuments(a.db)
}

func (a *App) answerLibraryExcerpts(ranked []rankedAnswerLibraryDocument) ([]answerLibrarySuggestion, error) {
	return answerlibrary.Excerpts(a.db, ranked)
}

func (a *App) searchAnswerLibrary(query string, limit int) ([]answerLibrarySuggestion, error) {
	return answerlibrary.Search(a.db, query, limit)
}

// searchAnswerLibraryPaged serves library-wide browse/search only. It ranks
// with the higher library cap and merges exact-equivalence matches first so
// user-defined alternative phrasings always surface even when the ranker
// threshold would skip them. Ranked application suggestions keep using
// searchAnswerLibrary with the 3-result cap.
func (a *App) searchAnswerLibraryPaged(query string, limit int) ([]answerLibrarySuggestion, error) {
	return answerlibrary.SearchPaged(a.db, query, limit)
}

func (a *App) answerLibrarySuggestionsForQuestions(questions []preparedQuestion) (map[string][]answerLibrarySuggestion, error) {
	var eligible []preparedQuestion
	for _, question := range questions {
		if question.Editable && answerLibraryEligible(question.Field, true) {
			eligible = append(eligible, question)
		}
	}
	if len(eligible) == 0 {
		return nil, nil
	}
	docs, err := a.answerLibrarySearchDocuments()
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	type scoredForQuestion struct {
		key   string
		items []rankedAnswerLibraryDocument
	}
	var perQuestion []scoredForQuestion
	allIDs := make(map[int64]bool)
	var union []rankedAnswerLibraryDocument
	for _, question := range eligible {
		ranked := rankAnswerLibraryDocuments(question.Field.Label, docs, answerLibraryRelatedLimit)
		if len(ranked) == 0 {
			continue
		}
		perQuestion = append(perQuestion, scoredForQuestion{key: question.Key, items: ranked})
		for _, item := range ranked {
			if !allIDs[item.Document.EntryID] {
				allIDs[item.Document.EntryID] = true
				union = append(union, item)
			}
		}
	}
	excerpts, err := a.answerLibraryExcerpts(union)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]answerLibrarySuggestion, len(excerpts))
	for _, excerpt := range excerpts {
		byID[excerpt.EntryID] = excerpt
	}
	result := make(map[string][]answerLibrarySuggestion, len(perQuestion))
	for _, group := range perQuestion {
		for _, item := range group.items {
			result[group.key] = append(result[group.key], byID[item.Document.EntryID])
		}
	}
	for _, q := range eligible {
		ids, err := equivalentLibraryIDs(a.db, q.Field.Label)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			continue
		}
		matches := []answerLibrarySuggestion{}
		seen := map[int64]bool{}
		kind := "Equivalent question — review and use"
		if len(ids) > 1 {
			kind = "Conflicting equivalents — choose after review"
		}
		for _, id := range ids {
			e, err := a.answerLibraryBySourceID(a.db, id)
			if err != nil {
				return nil, err
			}
			seen[id] = true
			matches = append(matches, answerLibrarySuggestion{EntryID: e.ID, Revision: e.Revision, OriginalQuestion: e.OriginalQuestion, SourceEmployer: e.SourceEmployer, SourceJobTitle: e.SourceJobTitle, SourceHost: e.SourceHost, Excerpt: answerLibraryExcerpt(e.Answer), MatchKind: kind})
		}
		for _, s := range result[q.Key] {
			if !seen[s.EntryID] {
				matches = append(matches, s)
			}
		}
		result[q.Key] = matches
	}
	return result, nil
}

// indexUnindexedAnswerLibraryEntries backfills only explicitly curated entries
// created by the pre-search schema. It never reads application_answers.
func indexUnindexedAnswerLibraryEntries(db *sql.DB) error {
	return answerlibrary.IndexUnindexedEntries(db)
}
