package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

func parseAshbyURL(rawURL string) (board, postingID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse Ashby URL: %w", err)
	}

	if u.Scheme != "https" ||
		!strings.EqualFold(u.Hostname(), "jobs.ashbyhq.com") ||
		u.Port() != "" ||
		u.User != nil || u.RawPath != "" || u.Opaque != "" {
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
	Jobs []ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	ID               string              `json:"id"`
	Title            string              `json:"title"`
	DescriptionPlain ashbyOptionalString `json:"descriptionPlain"`
	DescriptionHTML  ashbyOptionalString `json:"descriptionHtml"`
	JobURL           string              `json:"jobUrl"`
	Location         ashbyOptionalString `json:"location"`
	WorkplaceType    ashbyOptionalString `json:"workplaceType"`
	EmploymentType   ashbyOptionalString `json:"employmentType"`
	Compensation     json.RawMessage     `json:"compensation"`
}

// Ashby has several optional strings on each posting. Treat malformed optional
// values as absent so an otherwise usable canonical posting still imports.
type ashbyOptionalString string

func (value *ashbyOptionalString) UnmarshalJSON(data []byte) error {
	if !utf8.Valid(data) {
		*value = ""
		return nil
	}
	var decoded string
	if err := json.Unmarshal(data, &decoded); err != nil {
		*value = ""
		return nil
	}
	*value = ashbyOptionalString(decoded)
	return nil
}

func extractAshbyMetadata(data []byte, boardName, postingID string) (Metadata, error) {
	var board ashbyBoard

	if err := json.Unmarshal(data, &board); err != nil {
		return Metadata{}, fmt.Errorf("decode Ashby response: %w", err)
	}

	requested := PostingIdentity{Provider: "ashby", Board: boardName, PostingID: postingID}
	selected := -1
	for index := range board.Jobs {
		matched, err := ashbyJobMatchesIdentity(board.Jobs[index], requested)
		if err != nil {
			return Metadata{}, err
		}
		if matched {
			if selected >= 0 {
				return Metadata{}, fmt.Errorf("Ashby posting identity is ambiguous on board %q", boardName)
			}
			selected = index
		}
	}
	if selected < 0 {
		return Metadata{}, fmt.Errorf("%w: Ashby posting %s not found on board %q", ErrClosed, postingID, boardName)
	}

	job := board.Jobs[selected]
	if strings.TrimSpace(job.Title) == "" {
		return Metadata{}, fmt.Errorf("Ashby selected posting %s on board %q has no title", postingID, boardName)
	}
	salary := ashbySalarySummary(job.Compensation)
	plain := string(job.DescriptionPlain)
	html := string(job.DescriptionHTML)
	description := strings.TrimSpace(plain)
	if description == "" {
		description = descriptionText(html)
	}
	location := ashbyDirectFact(string(job.Location))
	workplaceType := ashbyDirectFact(string(job.WorkplaceType))
	employmentType := ashbyDirectFact(string(job.EmploymentType))
	metadata := completeMetadata(Metadata{
		Title:       strings.TrimSpace(job.Title),
		Description: description,
		SalaryText:  salary,
	})

	if strings.TrimSpace(html) == "" && strings.TrimSpace(plain) == "" &&
		location == "" && workplaceType == "" && employmentType == "" && salary == "" {
		metadata.Source = UnavailablePostingSource(PostingSourceAbsent)
		return metadata, nil
	}
	source, err := SealPostingSource(PostingSource{
		Version:          PostingSourceFormatVersion,
		Identity:         requested,
		DescriptionHTML:  html,
		DescriptionPlain: plain,
		Location:         location,
		WorkplaceType:    workplaceType,
		EmploymentType:   employmentType,
		SalarySummary:    salary,
	}, metadata.Description)
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

func ashbySalarySummary(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var compensation struct {
		SalarySummary ashbyOptionalString `json:"scrapeableCompensationSalarySummary"`
	}
	if json.Unmarshal(raw, &compensation) != nil {
		return ""
	}
	return strings.TrimSpace(string(compensation.SalarySummary))
}

// Direct facts are small provider values. Unknown nonempty enum strings remain
// available for honest display, while malformed or implausibly large values
// are omitted instead of invalidating an otherwise usable posting.
func ashbyDirectFact(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 || strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	return value
}

func ashbyJobMatchesIdentity(job ashbyJob, requested PostingIdentity) (bool, error) {
	idMatch := job.ID != "" && job.ID == requested.PostingID
	if job.JobURL == "" {
		if idMatch {
			return false, fmt.Errorf("Ashby selected posting %s has an ID but no qualified jobUrl", requested.PostingID)
		}
		return false, nil
	}

	returned, err := AshbyPostingIdentity(job.JobURL)
	if err != nil {
		if idMatch || ashbyURLHasPostingID(job.JobURL, requested.PostingID) {
			return false, fmt.Errorf("Ashby selected posting has an invalid jobUrl: %w", err)
		}
		return false, nil
	}
	urlHasRequestedID := returned.PostingID == requested.PostingID
	urlMatches := returned.Board == requested.Board && urlHasRequestedID
	if (idMatch && !urlMatches) || (urlHasRequestedID && returned.Board != requested.Board) {
		return false, fmt.Errorf("Ashby selected posting ID/jobUrl conflicts with requested board %q and posting %q", requested.Board, requested.PostingID)
	}
	if urlMatches && job.ID != "" && job.ID != requested.PostingID {
		return false, fmt.Errorf("Ashby selected posting id %q conflicts with matching jobUrl", job.ID)
	}
	return urlMatches, nil
}

func ashbyURLHasPostingID(rawURL, postingID string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	parts := pathSegments(u.Path)
	return len(parts) == 2 && parts[1] == postingID
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
	meta, err := extractAshbyMetadata(body, board, postingID)
	if err != nil {
		return Metadata{}, err
	}
	meta.Company = boardNameSuggestion(board)
	return meta, nil
}
