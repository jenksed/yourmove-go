package answerlibrary

import (
	"math/bits"
	"sort"
	"strings"
)

// SearchDocument is the ranked-search projection of one library entry.
type SearchDocument struct {
	EntryID          int64
	Revision         int64
	OriginalQuestion string
	SourceEmployer   string
	SourceJobTitle   string
	SourceHost       string
	UpdatedAt        string
	QuestionTerms    map[string]struct{}
	AnswerTerms      map[string]struct{}
	AltTerms         map[string]struct{}
	SourceTerms      map[string]struct{}
}

// RankedDocument pairs a SearchDocument with its query score.
type RankedDocument struct {
	Document        SearchDocument
	Score           int
	QuestionWeight  int
	PhraseMatches   int
	SharedTerms     int
	MostInformative int
}

// NewSearchDocument builds the ranked-search projection for one entry.
func NewSearchDocument(entry Entry) SearchDocument {
	return SearchDocument{
		EntryID: entry.ID, Revision: entry.Revision, OriginalQuestion: entry.OriginalQuestion,
		SourceEmployer: entry.SourceEmployer, SourceJobTitle: entry.SourceJobTitle,
		SourceHost: entry.SourceHost, UpdatedAt: entry.UpdatedAt,
		QuestionTerms: TermSet(IndexTerms(entry.OriginalQuestion), ""),
		AnswerTerms:   TermSet(IndexTerms(entry.Answer), ""),
		AltTerms:      TermSet(IndexTerms(strings.Join(entry.Alternatives, "\n")), ""),
		SourceTerms: TermSet("", entry.SourceEmployer+" "+entry.SourceJobTitle+
			" "+entry.SourceHost),
	}
}

// DocumentFrequency counts question-term occurrences across the corpus.
func DocumentFrequency(docs []SearchDocument) map[string]int {
	df := make(map[string]int)
	for _, doc := range docs {
		for term := range doc.QuestionTerms {
			df[term]++
		}
	}
	return df
}

// TermWeight scores a term by corpus rarity.
func TermWeight(librarySize, documentFrequency int) int {
	if librarySize < 1 {
		librarySize = 1
	}
	ratio := (librarySize + 1) / (documentFrequency + 1)
	if ratio < 1 {
		ratio = 1
	}
	return 2 + bits.Len(uint(ratio)) - 1
}

// WordPairs returns adjacent informative word pairs for phrase matching.
func WordPairs(value string) map[string]struct{} {
	words := Words(value)
	if len(words) < 2 {
		return nil
	}
	pairs := make(map[string]struct{})
	for i := 0; i+1 < len(words); i++ {
		if _, firstStop := stopwords[words[i]]; firstStop {
			continue
		}
		if _, secondStop := stopwords[words[i+1]]; secondStop {
			continue
		}
		pairs[words[i]+"\x00"+words[i+1]] = struct{}{}
	}
	return pairs
}

// SharedPhraseMatches counts informative pair overlap up to limit.
func SharedPhraseMatches(left, right map[string]struct{}, limit int) int {
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

// PhraseMatches counts informative adjacent-phrase overlap for one question.
func PhraseMatches(query, originalQuestion string) int {
	return SharedPhraseMatches(WordPairs(query), WordPairs(originalQuestion), 2)
}

// RankDocuments ranks docs for one query with the preparation cap.
func RankDocuments(query string, docs []SearchDocument, limit int) []RankedDocument {
	return RankDocumentsWithLimit(query, docs, limit, SearchPageLimit, false)
}

// RankDocumentsWithLimit ranks docs for one query with an explicit cap.
func RankDocumentsWithLimit(query string, docs []SearchDocument, limit, cap int, libraryWide bool) []RankedDocument {
	if limit < 1 || len(docs) == 0 {
		return nil
	}
	if limit > cap {
		limit = cap
	}
	queryTerms := InformativeTerms(query, 32)
	if len(queryTerms) == 0 {
		return nil
	}
	df := DocumentFrequency(docs)
	ranked := make([]RankedDocument, 0)
	for _, doc := range docs {
		questionWeight, answerWeight, sourceWeight, sharedCount, mostInformative := 0, 0, 0, 0, 0
		seen := make(map[string]bool, len(queryTerms))
		for _, term := range queryTerms {
			weight := TermWeight(len(docs), df[term])
			_, inQuestion := doc.QuestionTerms[term]
			_, inAlt := doc.AltTerms[term]
			_, inAnswer := doc.AnswerTerms[term]
			_, inSource := doc.SourceTerms[term]
			if inQuestion || inAlt {
				questionWeight += weight
			}
			if inAnswer {
				answerWeight += weight
			}
			if inSource {
				sourceWeight += weight
			}
			if inQuestion || inAlt || inAnswer || inSource {
				seen[term] = true
				if weight > mostInformative {
					mostInformative = weight
				}
			}
		}
		for range seen {
			sharedCount++
		}
		phraseMatches := PhraseMatches(query, doc.OriginalQuestion)
		if answerWeight > 6 {
			answerWeight = 6
		}
		if sourceWeight > 4 {
			sourceWeight = 4
		}
		score := 2*questionWeight + answerWeight + sourceWeight + 4*phraseMatches
		// A sharp source-context match (employer/title/host) is a legitimate
		// library-wide hit even without question/answer overlap. Ranked
		// preparation suggestions keep the stricter historical gate.
		sourceHit := libraryWide && sourceWeight >= 4
		if score < 5 && !sourceHit {
			continue
		}
		if sharedCount < 2 && mostInformative < 6 && phraseMatches == 0 && !sourceHit {
			continue
		}
		ranked = append(ranked, RankedDocument{
			Document: doc, Score: score, QuestionWeight: questionWeight,
			PhraseMatches: phraseMatches, SharedTerms: sharedCount, MostInformative: mostInformative,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		if ranked[i].QuestionWeight != ranked[j].QuestionWeight {
			return ranked[i].QuestionWeight > ranked[j].QuestionWeight
		}
		if ranked[i].PhraseMatches != ranked[j].PhraseMatches {
			return ranked[i].PhraseMatches > ranked[j].PhraseMatches
		}
		return ranked[i].Document.EntryID < ranked[j].Document.EntryID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// Excerpt truncates answer text to the library excerpt bound.
func Excerpt(value string) string {
	runes := []rune(value)
	if len(runes) <= ExcerptRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:ExcerptRunes])) + "…"
}
