package platforms

import (
	"context"
	"errors"
)

var ErrUnsupported = errors.New("unsupported job platform")

type Metadata struct {
	Title       string
	Description string
}

func FetchJob(ctx context.Context, jobURL string) (Metadata, error) {
	board, postingID, err := parseAshbyURL(jobURL)
	if err == nil {
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
