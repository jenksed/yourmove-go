package platforms

import (
	"context"
	"testing"
)

func TestParseLeverURL(t *testing.T) {
	tests := []struct {
		url, site, id, apiHost string
		wantErr                bool
	}{
		{"https://jobs.lever.co/example/abc-123", "example", "abc-123", "api.lever.co", false},
		{"https://jobs.eu.lever.co/lever/f00a01?lever-source=site", "lever", "f00a01", "api.eu.lever.co", false},
		{"https://jobs.lever.co/example", "", "", "", true},
		{"https://jobs.lever.co.example.test/example/abc", "", "", "", true},
		{"https://jobs.lever.co/example/../admin", "", "", "", true},
		{"http://jobs.lever.co/example/abc", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			site, id, host, err := parseLeverURL(tt.url)
			if (err != nil) != tt.wantErr || (err == nil && (site != tt.site || id != tt.id || host != tt.apiHost)) {
				t.Fatalf("got (%q, %q, %q, %v), expected (%q, %q, %q, err=%v)", site, id, host, err, tt.site, tt.id, tt.apiHost, tt.wantErr)
			}
		})
	}
}

func TestFetchLever(t *testing.T) {
	client := testPostingClient(t, "api.eu.lever.co", "/v0/postings/lever/abc-123", `{
		"text": "Implementation Engineer",
		"descriptionPlain": "Work with customers.",
		"lists": [{"text": "Responsibilities", "content": "<li>Configure systems</li><li>Debug issues</li>"}],
		"additionalPlain": "Equal opportunity employer."
	}`)
	got, err := fetchLever(context.Background(), client, "lever", "abc-123", "api.eu.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	want := "Work with customers.\n\nResponsibilities\n• Configure systems\n• Debug issues\n\nEqual opportunity employer."
	if got.Title != "Implementation Engineer" || got.Description != want {
		t.Fatalf("unexpected Lever metadata: %+v, want description %q", got, want)
	}
}
