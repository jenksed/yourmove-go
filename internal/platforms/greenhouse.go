package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
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
	ID       json.RawMessage `json:"id"`
	Title    string          `json:"title"`
	Content  json.RawMessage `json:"content"`
	Location json.RawMessage `json:"location"`
}

func fetchGreenhouse(ctx context.Context, client *http.Client, board, jobID string) (Metadata, error) {
	endpoint := "https://boards-api.greenhouse.io/v1/boards/" +
		url.PathEscape(board) + "/jobs/" + url.PathEscape(jobID)
	data, err := getPostingJSON(ctx, client, endpoint)
	if err != nil {
		return Metadata{}, err
	}
	return extractGreenhouseMetadata(data, board, jobID)
}

func extractGreenhouseMetadata(data []byte, board, requestedID string) (Metadata, error) {
	var posting greenhouseJob
	if err := json.Unmarshal(data, &posting); err != nil {
		return Metadata{}, fmt.Errorf("decode Greenhouse posting: %w", err)
	}
	if strings.TrimSpace(posting.Title) == "" {
		return Metadata{}, fmt.Errorf("Greenhouse posting has no title")
	}
	responseID, err := greenhouseResponseID(posting.ID)
	if err != nil {
		return Metadata{}, err
	}
	if responseID != requestedID {
		return Metadata{}, fmt.Errorf("Greenhouse posting identity mismatch: got %q, want %q", responseID, requestedID)
	}

	content := greenhouseOptionalString(posting.Content)
	location := greenhouseLocationName(posting.Location)
	metadata := completeMetadata(Metadata{
		Title:       strings.TrimSpace(posting.Title),
		Company:     boardNameSuggestion(board),
		Description: descriptionText(content),
	})

	if strings.TrimSpace(content) == "" && location == "" {
		metadata.Source = UnavailablePostingSource(PostingSourceAbsent)
		return metadata, nil
	}

	candidate := greenhouseSourceCandidate(content, location, board, requestedID)
	source, err := SealPostingSource(candidate, metadata.Description)
	if err != nil {
		reason := PostingSourceInvalid
		if errors.Is(err, ErrPostingSourceTooLarge) {
			reason = PostingSourceTooLarge
		}
		metadata.Source = UnavailablePostingSource(reason)
		return metadata, nil
	}
	metadata.Source = AvailablePostingSource(source)
	return metadata, nil
}

func greenhouseResponseID(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("Greenhouse posting has no ID")
	}
	var id uint64
	if err := json.Unmarshal(raw, &id); err != nil || id == 0 {
		return "", fmt.Errorf("Greenhouse posting has invalid ID")
	}
	return strconv.FormatUint(id, 10), nil
}

func greenhouseOptionalString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func greenhouseLocationName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var location struct {
		Name json.RawMessage `json:"name"`
	}
	if err := json.Unmarshal(raw, &location); err != nil {
		return ""
	}
	return greenhouseDirectFact(greenhouseOptionalString(location.Name))
}

func greenhouseDirectFact(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 || len(value) > 512 {
		return ""
	}
	return value
}

func greenhouseSourceCandidate(content, location, board, jobID string) PostingSource {
	return PostingSource{
		Version: PostingSourceFormatVersion,
		Identity: PostingIdentity{
			Provider:  "greenhouse",
			Board:     board,
			PostingID: jobID,
		},
		DescriptionHTML: greenhouseDecodedHTML(content),
		Location:        location,
	}
}

func greenhouseDecodedHTML(content string) string {
	// The Job Board API example includes entity-encoded markup; match the
	// existing canonical extractor's two-pass decoding for rich source too.
	for i := 0; i < 2; i++ {
		decoded := stdhtml.UnescapeString(content)
		if decoded == content {
			break
		}
		content = decoded
	}
	return content
}
