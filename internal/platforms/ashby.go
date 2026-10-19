package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func parseAshbyURL(rawURL string) (board, postingID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse Ashby URL: %w", err)
	}

	if u.Scheme != "https" ||
		!strings.EqualFold(u.Hostname(), "jobs.ashbyhq.com") ||
		u.Port() != "" ||
		u.User != nil {
		return "", "", fmt.Errorf("unsupported Ashby URL")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid Ashby posting path")
	}

	return parts[0], parts[1], nil
}

type ashbyBoard struct {
	Jobs []struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		DescriptionPlain string `json:"descriptionPlain"`
	} `json:"jobs"`
}

func extractAshbyMetadata(data []byte, postingID string) (Metadata, error) {
	var board ashbyBoard

	if err := json.Unmarshal(data, &board); err != nil {
		return Metadata{}, fmt.Errorf("decode Ashby response: %w", err)
	}

	for _, job := range board.Jobs {
		if job.ID == postingID {
			return Metadata{
				Title:       job.Title,
				Description: job.DescriptionPlain,
			}, nil
		}
	}

	return Metadata{}, fmt.Errorf("Ashby posting %s not found", postingID)
}

func fetchAshby(ctx context.Context, board, postingID string) (Metadata, error) {
	apiURL := "https://api.ashbyhq.com/posting-api/job-board/" +
		url.PathEscape(board)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return Metadata{}, fmt.Errorf("create Ashby request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch Ashby board: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("Ashby API returned HTTP %d", resp.StatusCode)
	}

	const maxBytes = 4 << 20 // 4 MiB

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return Metadata{}, fmt.Errorf("read Ashby response: %w", err)
	}

	if len(body) > maxBytes {
		return Metadata{}, fmt.Errorf("Ashby response exceeds 4 MiB")
	}

	return extractAshbyMetadata(body, postingID)
}
