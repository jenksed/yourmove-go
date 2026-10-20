package platforms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxPostingResponse = 4 << 20 // 4 MiB

func postingClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		// API URLs are constructed against fixed hosts. Do not follow redirects
		// to another destination based on a remote server's response.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func getPostingJSON(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	return getPostingJSONLimited(ctx, client, endpoint, maxPostingResponse)
}

// Ashby's public endpoint returns a whole board; it needs a different
// bounded limit from single-posting APIs such as Lever and Greenhouse.
func getPostingJSONLimited(ctx context.Context, client *http.Client, endpoint string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create posting request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch posting: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("posting API returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read posting response: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("posting response exceeds %d MiB", maxBytes>>20)
	}
	return body, nil
}

// safeToken permits only path-segment characters used in recognized ATS URLs.
func safeToken(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func pathSegments(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

// An ATS board slug is an identifying hint, not a verified employer name.
// Keep the capitalization simple so the user can correct branded names.
func boardNameSuggestion(slug string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(slug, "-", " ")), " ")
}
