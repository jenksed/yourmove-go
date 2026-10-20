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

func TestAshbyBoardCanExceedSinglePostingLimit(t *testing.T) {
	body := `{"jobs":[` + strings.Repeat(" ", maxPostingResponse) + `]}`
	client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	if _, err := getPostingJSON(context.Background(), client, "https://api.ashbyhq.com/posting-api/job-board/acme"); err == nil {
		t.Fatal("single-posting limit should reject a 4 MiB+ response")
	}
	if got, err := getPostingJSONLimited(context.Background(), client, "https://api.ashbyhq.com/posting-api/job-board/acme", 32<<20); err != nil || string(got) != body {
		t.Fatalf("board-cap fetch: bytes=%d err=%v", len(got), err)
	}
}
