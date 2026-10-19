package platforms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetPostingJSONRejectsBadResponses(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
	}{
		{"not found", http.StatusNotFound, `{}`},
		{"too large", http.StatusOK, strings.Repeat("x", maxPostingResponse+1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tt.status,
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			})}
			if _, err := getPostingJSON(context.Background(), client, "https://api.lever.co/v0/postings/acme/123"); err == nil {
				t.Fatal("expected posting API error")
			}
		})
	}
}
