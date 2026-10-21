package main

import (
	"example.com/yourmove-lite/internal/application/schema"
	"testing"
)

func TestTrialProfileFields(t *testing.T) {
	p := CandidateProfile{Approved: true, LinkedIn: "https://www.linkedin.com/in/fixture", City: "Cadillac", Region: "Michigan", Country: "USA"}
	for _, label := range []string{"LinkedIn", "LinkedIn URL", "LinkedIn Profile", "LinkedIn Profile URL", "LinkedIn link (optional)", "Your LinkedIn URL *"} {
		f := schema.Field{ID: "browser:opaque-employer-field", Label: label, Kind: "url"}
		if got := resolveAdditionalProfileFact(f, p); got != p.LinkedIn {
			t.Errorf("%s: %q", label, got)
		}
	}
	for _, kind := range []string{"text", "combobox"} {
		f := schema.Field{ID: "browser:opaque-location", Label: "Current location", Kind: kind}
		if got := resolveAdditionalProfileFact(f, p); got != "Cadillac, Michigan, USA" {
			t.Errorf("location: %q", got)
		}
	}
	for _, label := range []string{"Manager LinkedIn URL", "Sign in with LinkedIn", "Preferred relocation location"} {
		if got := resolveAdditionalProfileFact(schema.Field{ID: "browser:opaque", Label: label, Kind: "text"}, p); got != "" {
			t.Errorf("unrelated %s: %q", label, got)
		}
	}
}
