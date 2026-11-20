package app

import (
	"github.com/jenksed/yourmove-go/internal/answerlibrary"
)

// Canonical Answer Library retrieval lives in internal/answerlibrary.
// This file keeps the historical in-package names as thin delegating
// aliases so existing handlers, tests, and cross-feature callers keep
// compiling with zero behavior change.

type (
	answerLibrarySuggestion     = answerlibrary.Suggestion
	answerLibrarySearchDocument = answerlibrary.SearchDocument
	rankedAnswerLibraryDocument = answerlibrary.RankedDocument
)

const (
	answerLibrarySearchIndexVersion = answerlibrary.SearchIndexVersion
	answerLibraryCorpusLimit        = answerlibrary.CorpusLimit
	answerLibraryRelatedLimit       = answerlibrary.RelatedLimit
	answerLibrarySearchPageLimit    = answerlibrary.SearchPageLimit
	answerLibraryLibrarySearchLimit = answerlibrary.LibrarySearchLimit
	answerLibraryLibraryPageSize    = answerlibrary.LibraryPageSize
	answerLibraryExcerptRunes       = answerlibrary.ExcerptRunes
)

func answerLibraryWords(value string) []string {
	return answerlibrary.Words(value)
}

func informativeAnswerLibraryTerms(value string, maxTerms int) []string {
	return answerlibrary.InformativeTerms(value, maxTerms)
}

func answerLibraryIndexTerms(value string) string {
	return answerlibrary.IndexTerms(value)
}

func answerLibraryTermSet(index string, fallback string) map[string]struct{} {
	return answerlibrary.TermSet(index, fallback)
}

func newAnswerLibrarySearchDocument(entry AnswerLibraryEntry) answerLibrarySearchDocument {
	return answerlibrary.NewSearchDocument(answerlibrary.Entry(entry))
}

func answerLibraryDocumentFrequency(docs []answerLibrarySearchDocument) map[string]int {
	return answerlibrary.DocumentFrequency(docs)
}

func answerLibraryTermWeight(librarySize, documentFrequency int) int {
	return answerlibrary.TermWeight(librarySize, documentFrequency)
}

func answerLibraryWordPairs(value string) map[string]struct{} {
	return answerlibrary.WordPairs(value)
}

func answerLibrarySharedPhraseMatches(left, right map[string]struct{}, limit int) int {
	return answerlibrary.SharedPhraseMatches(left, right, limit)
}

func answerLibraryPhraseMatches(query, originalQuestion string) int {
	return answerlibrary.PhraseMatches(query, originalQuestion)
}

func rankAnswerLibraryDocuments(query string, docs []answerLibrarySearchDocument, limit int) []rankedAnswerLibraryDocument {
	return answerlibrary.RankDocuments(query, docs, limit)
}

func rankAnswerLibraryDocumentsWithLimit(query string, docs []answerLibrarySearchDocument, limit, cap int, libraryWide bool) []rankedAnswerLibraryDocument {
	return answerlibrary.RankDocumentsWithLimit(query, docs, limit, cap, libraryWide)
}

func answerLibraryExcerpt(value string) string {
	return answerlibrary.Excerpt(value)
}
