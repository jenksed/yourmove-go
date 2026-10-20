package platforms

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

var ErrUnsupported = errors.New("unsupported job platform")

type Metadata struct {
	Title       string
	Company     string
	Description string
	// SalaryText is supplied by the employer posting API, not inferred from the description.
	SalaryText string
}

func FetchJob(ctx context.Context, jobURL string) (Metadata, error) {
	// An incomplete Ashby UUID is an invalid source URL, not an
	// unsupported platform and not proof a posting has closed.
	if u, err := url.Parse(jobURL); err == nil && u.Hostname() == "jobs.ashbyhq.com" {
		board, postingID, err := parseAshbyURL(jobURL)
		if err != nil {
			return Metadata{}, fmt.Errorf("Ashby URL: %w", err)
		}
		return fetchAshby(ctx, board, postingID)
	}

	if board, jobID, err := parseGreenhouseURL(jobURL); err == nil {
		return fetchGreenhouse(ctx, postingClient(), board, jobID)
	}
	if site, postingID, apiHost, err := parseLeverURL(jobURL); err == nil {
		return fetchLever(ctx, postingClient(), site, postingID, apiHost)
	}
	if supportedMetadataURL(jobURL) {
		return fetchJobMetadata(ctx, jobURL)
	}

	return Metadata{}, ErrUnsupported
}
