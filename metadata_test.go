package main

import (
	"strings"
	"testing"
)

func TestExtractJobTitle(t *testing.T) {
	page := `
	<html>
	<head>
		<title>Paylocity - HCM Account Executive</title>
		<script type="application/ld+json">
		{
			"@context": "https://schema.org",
			"@type": "JobPosting",
			"title": "HCM Account Executive"
		}
		</script>
	</head>
	</html>`

	title, err := extractJobTitle(strings.NewReader(page))
	if err != nil {
		t.Fatalf("extract job title: %v", err)
	}

	if want := "HCM Account Executive"; title != want {
		t.Errorf("title = %q, want %q", title, want)
	}
}
