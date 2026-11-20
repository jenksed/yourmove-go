package answerlibrary

import "strings"

// Normalize canonicalizes a question for exact-equivalence comparison.
// Only case, surrounding whitespace, and repeated whitespace are superficial.
// Punctuation and word order remain significant; no stemming or synonyms.
func Normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
