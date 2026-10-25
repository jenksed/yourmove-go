package platforms

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestGreenhouseErrorBoardIsClosed(t *testing.T) {
	for _, raw := range []string{
		"https://job-boards.greenhouse.io/tailscale?error=true",
		"https://boards.greenhouse.io/tailscale/?error=true&gh_src=email",
	} {
		meta, err := FetchJob(context.Background(), raw)
		if !errors.Is(err, ErrClosed) || meta != (Metadata{}) {
			t.Fatalf("%s: metadata=%+v err=%v", raw, meta, err)
		}
	}
	for _, raw := range []string{
		"https://job-boards.greenhouse.io/tailscale",
		"https://job-boards.greenhouse.io/tailscale?error=false",
		"https://job-boards.greenhouse.io/tailscale/jobs/123?error=true",
		"https://job-boards.greenhouse.io.evil.test/tailscale?error=true",
	} {
		if isGreenhouseErrorBoard(raw) {
			t.Fatalf("incorrectly closed %s", raw)
		}
	}
}

func TestMissingGreenhousePostingIsClosed(t *testing.T) {
	for _, status := range []int{404, 410, 403, 429, 500, 503} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header), Request: r}, nil
			})}
			_, err := fetchGreenhouse(context.Background(), client, "tailscale", "123")
			if err == nil || errors.Is(err, ErrClosed) != (status == 404 || status == 410) {
				t.Fatalf("HTTP %d: %v", status, err)
			}
		})
	}
}

func TestMissingAshbyPostingIsClosed(t *testing.T) {
	_, err := extractAshbyMetadata([]byte(`{"jobs":[{"id":"other","title":"Another open job"}]}`), "fixture", "11111111-1111-4111-8111-111111111111")
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("missing posting: %v", err)
	}
}
