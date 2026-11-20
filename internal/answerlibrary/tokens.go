package answerlibrary

import (
	"sort"
	"strings"
	"unicode"
)

var stopwords = map[string]struct{}{
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

// Words tokenizes free text into lowercase letter/digit runs.
func Words(value string) []string {
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

// InformativeTerms returns up to maxTerms significant tokens in first-seen order.
func InformativeTerms(value string, maxTerms int) []string {
	if maxTerms < 1 {
		return nil
	}
	seen := make(map[string]struct{})
	var terms []string
	for _, word := range Words(value) {
		if len([]rune(word)) < 3 {
			continue
		}
		if _, stop := stopwords[word]; stop {
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

// IndexTerms renders a deterministic searchable term set for one value.
func IndexTerms(value string) string {
	terms := InformativeTerms(value, 512)
	sort.Strings(terms)
	if len(terms) == 0 {
		return ""
	}
	return "|" + strings.Join(terms, "|") + "|"
}

// TermSet parses a stored index string, falling back to tokenizing fallback.
func TermSet(index string, fallback string) map[string]struct{} {
	set := make(map[string]struct{})
	if index == "" {
		for _, word := range InformativeTerms(fallback, 512) {
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

// ContainsPhrase reports whether phrase appears as a contiguous word run in text.
func ContainsPhrase(text, phrase string) bool {
	words, blocked := Words(text), Words(phrase)
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
