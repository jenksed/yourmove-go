package answerlibrary

import "testing"

func TestNormalizeIsCaseAndSpaceInsensitiveOnly(t *testing.T) {
	if got := Normalize("  TELL us about a challenging customer interaction.  "); got != "tell us about a challenging customer interaction." {
		t.Fatalf("normalize changed: %q", got)
	}
	if Normalize("Tell us about a difficult customer interaction.") == Normalize("Tell us about a challenging customer interaction.") {
		t.Fatal("unapproved synonyms matched")
	}
}

func TestRankPrefersQuestionOverlap(t *testing.T) {
	docs := []SearchDocument{
		NewSearchDocument(Entry{ID: 10, OriginalQuestion: "Describe a technical problem you resolved for a customer", Answer: "I investigated a challenging Linux production incident."}),
		NewSearchDocument(Entry{ID: 11, OriginalQuestion: "How do you keep stakeholders informed about project budgets", Answer: "I share concise updates."}),
	}
	ranked := RankDocuments("Tell us about a challenging Linux incident you investigated", docs, RelatedLimit)
	if len(ranked) != 1 || ranked[0].Document.EntryID != 10 {
		t.Fatalf("related technical writing was not surfaced: %+v", ranked)
	}
}

func TestIndexTermsDeterministic(t *testing.T) {
	a := IndexTerms("Describe a Linux incident you resolved")
	b := IndexTerms("resolved incident Linux Describe you a")
	if a != b {
		t.Fatalf("index terms not deterministic: %q vs %q", a, b)
	}
}
