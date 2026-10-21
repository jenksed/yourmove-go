package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Greenhouse redirects removed postings to the employer's board with error=true.
// Other jobs listed on that page do not mean the original posting is still open.
func isGreenhouseErrorBoard(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		(u.Hostname() != "job-boards.greenhouse.io" && u.Hostname() != "boards.greenhouse.io") {
		return false
	}
	parts := pathSegments(u.Path)
	return len(parts) == 1 && safeToken(parts[0]) && u.Query().Get("error") == "true"
}

func parseGreenhouseURL(rawURL string) (board, jobID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse Greenhouse URL: %w", err)
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" ||
		(u.Hostname() != "job-boards.greenhouse.io" && u.Hostname() != "boards.greenhouse.io") {
		return "", "", fmt.Errorf("unsupported Greenhouse URL")
	}
	parts := pathSegments(u.Path)
	if len(parts) != 3 || !safeToken(parts[0]) || parts[1] != "jobs" {
		return "", "", fmt.Errorf("invalid Greenhouse posting path")
	}
	id, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || id == 0 {
		return "", "", fmt.Errorf("invalid Greenhouse job ID")
	}
	return parts[0], parts[2], nil
}

type greenhouseJob struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func fetchGreenhouse(ctx context.Context, client *http.Client, board, jobID string) (Metadata, error) {
	endpoint := "https://boards-api.greenhouse.io/v1/boards/" +
		url.PathEscape(board) + "/jobs/" + url.PathEscape(jobID)
	data, err := getPostingJSON(ctx, client, endpoint)
	if err != nil {
		return Metadata{}, err
	}
	var posting greenhouseJob
	if err := json.Unmarshal(data, &posting); err != nil {
		return Metadata{}, fmt.Errorf("decode Greenhouse posting: %w", err)
	}
	if strings.TrimSpace(posting.Title) == "" {
		return Metadata{}, fmt.Errorf("Greenhouse posting has no title")
	}
	return Metadata{
		Title:       strings.TrimSpace(posting.Title),
		Company:     boardNameSuggestion(board),
		Description: descriptionText(posting.Content),
	}, nil
}
