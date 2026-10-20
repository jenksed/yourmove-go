package platforms

import (
	"strings"
	"testing"
)

func TestExtractJobMetadata(t *testing.T) {
	page := `
	<html>
	<head>
		<title>Paylocity - HCM Account Executive</title>
		<script type="application/ld+json">
		{
			"@context": "https://schema.org",
			"@type": "JobPosting",
			"title": "HCM Account Executive",
			"description": "<p>Description</p><p>Paylocity is...</p>"
		}
		</script>
	</head>
	</html>`

	metadata, err := extractJobMetadata(strings.NewReader(page))
	if err != nil {
		t.Fatalf("extract job title: %v", err)
	}

	if want := "HCM Account Executive"; metadata.Title != want {
		t.Errorf("title = %q, want %q", metadata.Title, want)
	}

	if want := "Description\nPaylocity is..."; metadata.Description != want {
		t.Errorf("description = %q, want %q", metadata.Description, want)
	}
}
