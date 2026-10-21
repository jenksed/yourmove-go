package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

	if len(parts) != 2 || !safeToken(parts[0]) || !ashbyUUID(parts[1]) {
		return "", "", fmt.Errorf("invalid Ashby posting URL: expected full posting UUID")
	}

	return parts[0], parts[1], nil
}

func ashbyUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

type ashbyBoard struct {
	Jobs []struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		DescriptionPlain string `json:"descriptionPlain"`
		DescriptionHTML  string `json:"descriptionHtml"`
		JobURL           string `json:"jobUrl"`
		Compensation     *struct {
			SalarySummary string `json:"scrapeableCompensationSalarySummary"`
		} `json:"compensation"`
	} `json:"jobs"`
}

func extractAshbyMetadata(data []byte, postingID string) (Metadata, error) {
	var board ashbyBoard

	if err := json.Unmarshal(data, &board); err != nil {
		return Metadata{}, fmt.Errorf("decode Ashby response: %w", err)
	}

	for _, job := range board.Jobs {
		match := job.ID == postingID
		if !match && job.JobURL != "" {
			if u, err := url.Parse(job.JobURL); err == nil && u.Hostname() == "jobs.ashbyhq.com" {
				parts := pathSegments(u.Path)
				match = len(parts) == 2 && parts[1] == postingID
			}
		}
		if match {
			salary := ""
			if job.Compensation != nil {
				salary = strings.TrimSpace(job.Compensation.SalarySummary)
			}
			description := strings.TrimSpace(job.DescriptionPlain)
			if description == "" {
				description = descriptionText(job.DescriptionHTML)
			}
			return Metadata{
				Title:       job.Title,
				Description: description,
				SalaryText:  salary,
			}, nil
		}
	}

	return Metadata{}, fmt.Errorf("%w: Ashby posting %s not found", ErrClosed, postingID)
}

func fetchAshby(ctx context.Context, board, postingID string) (Metadata, error) {
	apiURL := "https://api.ashbyhq.com/posting-api/job-board/" +
		url.PathEscape(board) + "?includeCompensation=true"

	// Unlike Lever/Greenhouse, this API returns every posting on the board.
	// 32 MiB remains bounded but accommodates larger employers.
	body, err := getPostingJSONLimited(ctx, postingClient(), apiURL, 32<<20)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch Ashby board: %w", err)
	}
	meta, err := extractAshbyMetadata(body, postingID)
	if err != nil {
		return Metadata{}, err
	}
	meta.Company = boardNameSuggestion(board)
	return meta, nil
}
