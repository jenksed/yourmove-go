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

	if supportedMetadataURL(jobURL) {
		return fetchJobMetadata(ctx, jobURL)
	}

	return Metadata{}, ErrUnsupported
}
