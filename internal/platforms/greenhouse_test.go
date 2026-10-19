package platforms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (fn testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func testPostingClient(t *testing.T, expectedHost, expectedPath, response string) *http.Client {
	t.Helper()
	return &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Host != expectedHost || r.URL.Path != expectedPath || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected API request: %s %s Accept=%q", r.Method, r.URL, r.Header.Get("Accept"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(response)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}
}

func TestParseGreenhouseURL(t *testing.T) {
	tests := []struct {
		url     string
		board   string
		id      string
		wantErr bool
	}{
		{"https://job-boards.greenhouse.io/acme/jobs/1234567", "acme", "1234567", false},
		{"https://boards.greenhouse.io/acme/jobs/789?gh_src=email", "acme", "789", false},
		{"https://job-boards.greenhouse.io/acme", "", "", true},
		{"https://job-boards.greenhouse.io/acme/jobs/nope", "", "", true},
		{"https://job-boards.greenhouse.io.evil.test/acme/jobs/123", "", "", true},
		{"http://boards.greenhouse.io/acme/jobs/123", "", "", true},
		{"https://boards.greenhouse.io@evil.test/acme/jobs/123", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			board, id, err := parseGreenhouseURL(tt.url)
			if (err != nil) != tt.wantErr || (err == nil && (board != tt.board || id != tt.id)) {
				t.Fatalf("got (%q, %q, %v), expected (%q, %q, err=%v)", board, id, err, tt.board, tt.id, tt.wantErr)
			}
		})
	}
}

func TestFetchGreenhouse(t *testing.T) {
	client := testPostingClient(t, "boards-api.greenhouse.io", "/v1/boards/acme/jobs/1234567", `{
		"title":"Support Engineer",
		"content":"&amp;lt;p&amp;gt;Debug API issues&amp;lt;/p&amp;gt;&amp;lt;ul&amp;gt;&amp;lt;li&amp;gt;Read logs&amp;lt;/li&amp;gt;&amp;lt;/ul&amp;gt;"
	}`)
	got, err := fetchGreenhouse(context.Background(), client, "acme", "1234567")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Support Engineer" || got.Description != "Debug API issues\n• Read logs" {
		t.Fatalf("unexpected Greenhouse metadata: %+v", got)
	}
}

func TestDescriptionTextSkipsScripts(t *testing.T) {
	got := descriptionText("<p>First</p><script>alert('x')</script><p>Second &amp; third</p>")
	if got != "First\nSecond & third" {
		t.Fatalf("description text = %q", got)
	}
}
