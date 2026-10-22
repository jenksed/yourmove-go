package app

import (
	"fmt"
	"sort"
	"strings"
)

const (
	careerEvidenceSearchLimit  = 1000
	careerEvidenceRelatedLimit = 2
	careerEvidenceExcerptRunes = 260
)

type careerEvidenceSuggestion struct {
	Evidence       CareerEvidence
	ActionsExcerpt string
}

type careerEvidenceSearchDocument struct {
	evidenceID int64
	titleTerms map[string]struct{}
	context    map[string]struct{}
	actions    map[string]struct{}
	tools      map[string]struct{}
	allTerms   map[string]struct{}
	phraseSets []map[string]struct{}
}

type rankedCareerEvidenceDocument struct {
	document      careerEvidenceSearchDocument
	score         int
	titleWeight   int
	phraseMatches int
}

func careerEvidenceTerms(values ...string) map[string]struct{} {
	terms := make(map[string]struct{})
	for _, value := range values {
		for _, term := range informativeAnswerLibraryTerms(value, 512) {
			terms[term] = struct{}{}
		}
	}
	return terms
}

func newCareerEvidenceSearchDocument(evidence CareerEvidence) careerEvidenceSearchDocument {
	title := careerEvidenceTerms(evidence.Title)
	context := careerEvidenceTerms(evidence.Situation, evidence.EmployerContext, evidence.RoleContext)
	actions := careerEvidenceTerms(evidence.Actions)
	tools := careerEvidenceTerms(evidence.Technologies)
	all := make(map[string]struct{}, len(context)+len(actions)+len(tools))
	for _, terms := range []map[string]struct{}{title, context, actions, tools} {
		for term := range terms {
			all[term] = struct{}{}
		}
	}
	phraseSets := make([]map[string]struct{}, 0, 6)
	for _, value := range []string{evidence.Title, evidence.Situation, evidence.EmployerContext, evidence.RoleContext, evidence.Actions, evidence.Technologies} {
		phraseSets = append(phraseSets, answerLibraryWordPairs(value))
	}
	return careerEvidenceSearchDocument{
		evidenceID: evidence.ID,
		titleTerms: title, context: context, actions: actions,
		tools: tools, allTerms: all, phraseSets: phraseSets,
	}
}

func careerEvidenceDocumentFrequency(docs []careerEvidenceSearchDocument) map[string]int {
	frequency := make(map[string]int)
	for _, doc := range docs {
		for term := range doc.allTerms {
			frequency[term]++
		}
	}
	return frequency
}

func careerEvidencePhraseMatches(queryPairs map[string]struct{}, phraseSets []map[string]struct{}) int {
	matches := 0
	for _, pairs := range phraseSets {
		matches += answerLibrarySharedPhraseMatches(queryPairs, pairs, 2)
		if matches >= 4 {
			return 4
		}
	}
	return matches
}

// rankCareerEvidenceDocuments favors title overlap, then situation/actions,
// then technologies. A result needs at least two shared terms or an exact
// informative word pair; one incidental word never surfaces an entry.
func rankCareerEvidenceDocuments(query string, docs []careerEvidenceSearchDocument, limit int) []rankedCareerEvidenceDocument {
	return rankCareerEvidenceDocumentsWithFrequency(query, docs, limit, careerEvidenceDocumentFrequency(docs))
}

func rankCareerEvidenceDocumentsWithFrequency(query string, docs []careerEvidenceSearchDocument, limit int, frequency map[string]int) []rankedCareerEvidenceDocument {
	if limit < 1 || len(docs) == 0 {
		return nil
	}
	if limit > careerEvidenceRelatedLimit {
		limit = careerEvidenceRelatedLimit
	}
	queryTerms := informativeAnswerLibraryTerms(query, 32)
	if len(queryTerms) == 0 {
		return nil
	}
	queryPairs := answerLibraryWordPairs(query)
	ranked := make([]rankedCareerEvidenceDocument, 0)
	for _, doc := range docs {
		score, titleWeight, shared := 0, 0, 0
		for _, term := range queryTerms {
			weight := answerLibraryTermWeight(len(docs), frequency[term])
			_, inTitle := doc.titleTerms[term]
			_, inContext := doc.context[term]
			_, inActions := doc.actions[term]
			_, inTools := doc.tools[term]
			if inTitle {
				titleWeight += weight
				score += 5 * weight
			}
			if inContext {
				score += 3 * weight
			}
			if inActions {
				score += 3 * weight
			}
			if inTools {
				score += 2 * weight
			}
			if inTitle || inContext || inActions || inTools {
				shared++
			}
		}
		phrases := careerEvidencePhraseMatches(queryPairs, doc.phraseSets)
		score += 6 * phrases
		if score < 8 || (shared < 2 && phrases == 0) {
			continue
		}
		ranked = append(ranked, rankedCareerEvidenceDocument{
			document: doc, score: score, titleWeight: titleWeight,
			phraseMatches: phrases,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].titleWeight != ranked[j].titleWeight {
			return ranked[i].titleWeight > ranked[j].titleWeight
		}
		if ranked[i].phraseMatches != ranked[j].phraseMatches {
			return ranked[i].phraseMatches > ranked[j].phraseMatches
		}
		return ranked[i].document.evidenceID < ranked[j].document.evidenceID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func careerEvidenceActionExcerpt(value string) string {
	compact := strings.Join(strings.Fields(value), " ")
	runes := []rune(compact)
	if len(runes) <= careerEvidenceExcerptRunes {
		return compact
	}
	return strings.TrimSpace(string(runes[:careerEvidenceExcerptRunes])) + "…"
}

func (a *App) careerEvidenceSearchDocuments() ([]careerEvidenceSearchDocument, error) {
	rows, err := a.db.Query(`SELECT id,title,situation,actions,employer_context,role_context,technologies
		FROM career_evidence WHERE status='reviewed' ORDER BY updated_at DESC,id DESC LIMIT ?`, careerEvidenceSearchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []careerEvidenceSearchDocument
	for rows.Next() {
		var evidence CareerEvidence
		if err := rows.Scan(&evidence.ID, &evidence.Title, &evidence.Situation, &evidence.Actions,
			&evidence.EmployerContext, &evidence.RoleContext, &evidence.Technologies); err != nil {
			return nil, err
		}
		docs = append(docs, newCareerEvidenceSearchDocument(evidence))
	}
	return docs, rows.Err()
}

func (a *App) careerEvidenceDetails(ids []int64) (map[int64]CareerEvidence, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := a.db.Query(careerEvidenceSelect+` WHERE status='reviewed' AND id IN (`+strings.Join(placeholders, ",")+") ORDER BY id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	evidenceByID := make(map[int64]CareerEvidence, len(ids))
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
		evidenceByID[evidence.ID] = evidence
	}
	return evidenceByID, rows.Err()
}

func (a *App) careerEvidenceSuggestionsForQuestions(questions []preparedQuestion) (map[string][]careerEvidenceSuggestion, error) {
	eligible := make([]preparedQuestion, 0, len(questions))
	for _, question := range questions {
		if question.Editable && answerLibraryEligible(question.Field, true) {
			eligible = append(eligible, question)
		}
	}
	if len(eligible) == 0 {
		return nil, nil
	}
	docs, err := a.careerEvidenceSearchDocuments()
	if err != nil || len(docs) == 0 {
		return nil, err
	}
	frequency := careerEvidenceDocumentFrequency(docs)
	type scoredQuestion struct {
		key   string
		items []rankedCareerEvidenceDocument
	}
	var perQuestion []scoredQuestion
	seen := make(map[int64]bool)
	var ids []int64
	for _, question := range eligible {
		ranked := rankCareerEvidenceDocumentsWithFrequency(question.Field.Label, docs, careerEvidenceRelatedLimit, frequency)
		if len(ranked) == 0 {
			continue
		}
		perQuestion = append(perQuestion, scoredQuestion{key: question.Key, items: ranked})
		for _, item := range ranked {
			id := item.document.evidenceID
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	details, err := a.careerEvidenceDetails(ids)
	if err != nil {
		return nil, fmt.Errorf("load selected Career Evidence records: %w", err)
	}
	suggestions := make(map[string][]careerEvidenceSuggestion, len(perQuestion))
	for _, question := range perQuestion {
		for _, item := range question.items {
			evidence, exists := details[item.document.evidenceID]
			if !exists {
				continue
			}
			suggestions[question.key] = append(suggestions[question.key], careerEvidenceSuggestion{
				Evidence: evidence, ActionsExcerpt: careerEvidenceActionExcerpt(evidence.Actions),
			})
		}
	}
	return suggestions, nil
}
