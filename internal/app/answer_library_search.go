package app

import (
	"database/sql"
	"fmt"
	"math/bits"
	"sort"
	"strings"
	"unicode"
)

const (
	answerLibrarySearchIndexVersion = 1
	answerLibraryCorpusLimit        = 5000
	answerLibraryRelatedLimit       = 3
	answerLibrarySearchPageLimit    = 3
	answerLibraryExcerptRunes       = 320
)

type answerLibrarySuggestion struct {
	EntryID          int64
	Revision         int64
	OriginalQuestion string
	SourceEmployer   string
	SourceJobTitle   string
	SourceHost       string
	Excerpt          string
}

type answerLibrarySearchDocument struct {
	EntryID          int64
	Revision         int64
	OriginalQuestion string
	SourceEmployer   string
	SourceJobTitle   string
	SourceHost       string
	UpdatedAt        string
	questionTerms    map[string]struct{}
	answerTerms      map[string]struct{}
}

type rankedAnswerLibraryDocument struct {
	document        answerLibrarySearchDocument
	score           int
	questionWeight  int
	phraseMatches   int
	sharedTerms     int
	mostInformative int
}

var answerLibraryStopwords = map[string]struct{}{
	"a": {}, "about": {}, "above": {}, "after": {}, "again": {}, "against": {}, "all": {}, "am": {},
	"an": {}, "and": {}, "any": {}, "are": {}, "as": {}, "at": {}, "be": {}, "because": {}, "been": {},
	"before": {}, "being": {}, "below": {}, "between": {}, "both": {}, "but": {}, "by": {}, "can": {},
	"could": {}, "did": {}, "do": {}, "does": {}, "doing": {}, "down": {}, "during": {}, "each": {},
	"few": {}, "for": {}, "from": {}, "further": {}, "had": {}, "has": {}, "have": {}, "having": {},
	"he": {}, "her": {}, "here": {}, "hers": {}, "herself": {}, "him": {}, "himself": {}, "his": {},
	"how": {}, "i": {}, "if": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {}, "itself": {},
	"just": {}, "me": {}, "more": {}, "most": {}, "my": {}, "myself": {}, "no": {}, "nor": {}, "not": {},
	"of": {}, "off": {}, "on": {}, "once": {}, "only": {}, "or": {}, "other": {}, "our": {}, "ours": {},
	"ourselves": {}, "out": {}, "over": {}, "own": {}, "same": {}, "she": {}, "should": {}, "so": {},
	"some": {}, "such": {}, "than": {}, "that": {}, "the": {}, "their": {}, "theirs": {}, "them": {},
	"themselves": {}, "then": {}, "there": {}, "these": {}, "they": {}, "this": {}, "those": {}, "through": {},
	"to": {}, "too": {}, "under": {}, "until": {}, "up": {}, "very": {}, "was": {}, "we": {}, "were": {},
	"what": {}, "when": {}, "where": {}, "which": {}, "while": {}, "who": {}, "whom": {}, "why": {},
	"will": {}, "with": {}, "would": {}, "you": {}, "your": {}, "yours": {}, "yourself": {}, "yourselves": {},
	// These words occur in many application prompts and do not establish intent.
	"answer": {}, "company": {}, "describe": {}, "discuss": {}, "experience": {}, "example": {}, "explain": {},
	"job": {}, "organization": {}, "previous": {}, "provide": {}, "question": {}, "role": {}, "share": {},
	"team": {}, "tell": {}, "work": {},
}

func answerLibraryWords(value string) []string {
	var words []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || (unicode.IsMark(r) && current.Len() > 0) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return words
}

func informativeAnswerLibraryTerms(value string, maxTerms int) []string {
	if maxTerms < 1 {
		return nil
	}
	seen := make(map[string]struct{})
	var terms []string
	for _, word := range answerLibraryWords(value) {
		if len([]rune(word)) < 3 {
			continue
		}
		if _, stop := answerLibraryStopwords[word]; stop {
			continue
		}
		if _, exists := seen[word]; exists {
			continue
		}
		seen[word] = struct{}{}
		terms = append(terms, word)
		if len(terms) == maxTerms {
			break
		}
	}
	return terms
}

func answerLibraryIndexTerms(value string) string {
	terms := informativeAnswerLibraryTerms(value, 512)
	sort.Strings(terms)
	if len(terms) == 0 {
		return ""
	}
	return "|" + strings.Join(terms, "|") + "|"
}

func answerLibraryTermSet(index string, fallback string) map[string]struct{} {
	set := make(map[string]struct{})
	if index == "" {
		for _, word := range informativeAnswerLibraryTerms(fallback, 512) {
			set[word] = struct{}{}
		}
		return set
	}
	for _, word := range strings.Split(strings.Trim(index, "|"), "|") {
		if word != "" {
			set[word] = struct{}{}
		}
	}
	return set
}

func newAnswerLibrarySearchDocument(entry AnswerLibraryEntry) answerLibrarySearchDocument {
	return answerLibrarySearchDocument{
		EntryID: entry.ID, Revision: entry.Revision, OriginalQuestion: entry.OriginalQuestion,
		SourceEmployer: entry.SourceEmployer, SourceJobTitle: entry.SourceJobTitle,
		SourceHost: entry.SourceHost, UpdatedAt: entry.UpdatedAt,
		questionTerms: answerLibraryTermSet(answerLibraryIndexTerms(entry.OriginalQuestion), ""),
		answerTerms:   answerLibraryTermSet(answerLibraryIndexTerms(entry.Answer), ""),
	}
}

func answerLibraryDocumentFrequency(docs []answerLibrarySearchDocument) map[string]int {
	df := make(map[string]int)
	for _, doc := range docs {
		for term := range doc.questionTerms {
			df[term]++
		}
	}
	return df
}

func answerLibraryTermWeight(librarySize, documentFrequency int) int {
	if librarySize < 1 {
		librarySize = 1
	}
	ratio := (librarySize + 1) / (documentFrequency + 1)
	if ratio < 1 {
		ratio = 1
	}
	return 2 + bits.Len(uint(ratio)) - 1
}

func answerLibraryWordPairs(value string) map[string]struct{} {
	words := answerLibraryWords(value)
	if len(words) < 2 {
		return nil
	}
	pairs := make(map[string]struct{})
	for i := 0; i+1 < len(words); i++ {
		if _, firstStop := answerLibraryStopwords[words[i]]; firstStop {
			continue
		}
		if _, secondStop := answerLibraryStopwords[words[i+1]]; secondStop {
			continue
		}
		pairs[words[i]+"\x00"+words[i+1]] = struct{}{}
	}
	return pairs
}

func answerLibrarySharedPhraseMatches(left, right map[string]struct{}, limit int) int {
	matches := 0
	for pair := range left {
		if _, exists := right[pair]; !exists {
			continue
		}
		matches++
		if limit > 0 && matches >= limit {
			return limit
		}
	}
	return matches
}

func answerLibraryPhraseMatches(query, originalQuestion string) int {
	return answerLibrarySharedPhraseMatches(answerLibraryWordPairs(query), answerLibraryWordPairs(originalQuestion), 2)
}

func rankAnswerLibraryDocuments(query string, docs []answerLibrarySearchDocument, limit int) []rankedAnswerLibraryDocument {
	if limit < 1 || len(docs) == 0 {
		return nil
	}
	if limit > answerLibrarySearchPageLimit {
		limit = answerLibrarySearchPageLimit
	}
	queryTerms := informativeAnswerLibraryTerms(query, 32)
	if len(queryTerms) == 0 {
		return nil
	}
	df := answerLibraryDocumentFrequency(docs)
	ranked := make([]rankedAnswerLibraryDocument, 0)
	for _, doc := range docs {
		questionWeight, answerWeight, sharedCount, mostInformative := 0, 0, 0, 0
		seen := make(map[string]bool, len(queryTerms))
		for _, term := range queryTerms {
			weight := answerLibraryTermWeight(len(docs), df[term])
			_, inQuestion := doc.questionTerms[term]
			_, inAnswer := doc.answerTerms[term]
			if inQuestion {
				questionWeight += weight
			}
			if inAnswer {
				answerWeight += weight
			}
			if inQuestion || inAnswer {
				seen[term] = true
				if weight > mostInformative {
					mostInformative = weight
				}
			}
		}
		for range seen {
			sharedCount++
		}
		phraseMatches := answerLibraryPhraseMatches(query, doc.OriginalQuestion)
		if answerWeight > 6 {
			answerWeight = 6
		}
		score := 2*questionWeight + answerWeight + 4*phraseMatches
		if score < 5 || (sharedCount < 2 && mostInformative < 6 && phraseMatches == 0) {
			continue
		}
		ranked = append(ranked, rankedAnswerLibraryDocument{
			document: doc, score: score, questionWeight: questionWeight,
			phraseMatches: phraseMatches, sharedTerms: sharedCount, mostInformative: mostInformative,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].questionWeight != ranked[j].questionWeight {
			return ranked[i].questionWeight > ranked[j].questionWeight
		}
		if ranked[i].phraseMatches != ranked[j].phraseMatches {
			return ranked[i].phraseMatches > ranked[j].phraseMatches
		}
		return ranked[i].document.EntryID < ranked[j].document.EntryID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func answerLibraryExcerpt(value string) string {
	runes := []rune(value)
	if len(runes) <= answerLibraryExcerptRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:answerLibraryExcerptRunes])) + "…"
}

func (a *App) answerLibrarySearchDocuments() ([]answerLibrarySearchDocument, error) {
	rows, err := a.db.Query(`SELECT id,revision,original_question,source_employer,source_job_title,source_host,updated_at,question_terms,answer_terms
	 FROM answer_library WHERE search_index_version=? ORDER BY id DESC LIMIT ?`, answerLibrarySearchIndexVersion, answerLibraryCorpusLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []answerLibrarySearchDocument
	for rows.Next() {
		var doc answerLibrarySearchDocument
		var questionTerms, answerTerms string
		if err := rows.Scan(&doc.EntryID, &doc.Revision, &doc.OriginalQuestion, &doc.SourceEmployer, &doc.SourceJobTitle,
			&doc.SourceHost, &doc.UpdatedAt, &questionTerms, &answerTerms); err != nil {
			return nil, err
		}
		doc.questionTerms = answerLibraryTermSet(questionTerms, doc.OriginalQuestion)
		doc.answerTerms = answerLibraryTermSet(answerTerms, "")
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (a *App) answerLibraryExcerpts(ranked []rankedAnswerLibraryDocument) ([]answerLibrarySuggestion, error) {
	if len(ranked) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ranked))
	args := make([]any, len(ranked))
	for i, item := range ranked {
		placeholders[i] = "?"
		args[i] = item.document.EntryID
	}
	rows, err := a.db.Query(`SELECT id,revision,substr(answer,1,`+fmt.Sprint(answerLibraryExcerptRunes)+`) FROM answer_library WHERE id IN (`+strings.Join(placeholders, ",")+")", args...)
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
		excerpts[id] = answerLibraryExcerpt(excerpt)
		revisions[id] = revision
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	results := make([]answerLibrarySuggestion, 0, len(ranked))
	for _, item := range ranked {
		doc := item.document
		results = append(results, answerLibrarySuggestion{EntryID: doc.EntryID, Revision: revisions[doc.EntryID], OriginalQuestion: doc.OriginalQuestion,
			SourceEmployer: doc.SourceEmployer, SourceJobTitle: doc.SourceJobTitle, SourceHost: doc.SourceHost, Excerpt: excerpts[doc.EntryID]})
	}
	return results, nil
}

func (a *App) searchAnswerLibrary(query string, limit int) ([]answerLibrarySuggestion, error) {
	docs, err := a.answerLibrarySearchDocuments()
	if err != nil {
		return nil, err
	}
	ranked := rankAnswerLibraryDocuments(query, docs, limit)
	return a.answerLibraryExcerpts(ranked)
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
			if !allIDs[item.document.EntryID] {
				allIDs[item.document.EntryID] = true
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
			result[group.key] = append(result[group.key], byID[item.document.EntryID])
		}
	}
	return result, nil
}

// indexUnindexedAnswerLibraryEntries backfills only explicitly curated entries
// created by the pre-search schema. It never reads application_answers.
func indexUnindexedAnswerLibraryEntries(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id,original_question,answer FROM answer_library WHERE search_index_version<>?`, answerLibrarySearchIndexVersion)
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
			answerLibraryIndexTerms(entry.question), answerLibraryIndexTerms(entry.answer), answerLibrarySearchIndexVersion, entry.id); err != nil {
			return fmt.Errorf("index curated Answer Library entry: %w", err)
		}
	}
	return tx.Commit()
}
