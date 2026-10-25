package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const workablePublicPostingHost = "apply.workable.com"

type workableAccount struct {
	Name string            `json:"name"`
	Jobs []workablePosting `json:"jobs"`
}

type workablePosting struct {
	Title          string             `json:"title"`
	Shortcode      string             `json:"shortcode"`
	Description    string             `json:"description"`
	EmploymentType string             `json:"employment_type"`
	WorkplaceType  string             `json:"workplace_type"`
	Telecommuting  *bool              `json:"telecommuting"`
	City           string             `json:"city"`
	State          string             `json:"state"`
	Country        string             `json:"country"`
	Locations      []workableLocation `json:"locations"`
}

type workableLocation struct {
	City        string `json:"city"`
	Region      string `json:"region"`
	State       string `json:"state"`
	Country     string `json:"country"`
	CountryName string `json:"country_name"`
	Hidden      bool   `json:"hidden"`
}

func parseWorkableURL(rawURL string) (account, shortcode string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse Workable URL: %w", err)
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" || u.RawPath != "" ||
		!strings.EqualFold(u.Hostname(), workablePublicPostingHost) {
		return "", "", fmt.Errorf("unsupported Workable URL")
	}
	path := u.Path
	if strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || !safeToken(parts[0]) || parts[1] != "j" || !safeToken(parts[2]) {
		return "", "", fmt.Errorf("invalid Workable posting path")
	}
	return parts[0], parts[2], nil
}

// fetchWorkable retrieves the account's public widget response and selects only
// the exact URL shortcode. The widget endpoint is account scoped, so absence of
// a shortcode is not proof that the posting is closed.
func fetchWorkable(ctx context.Context, client *http.Client, account, shortcode string) (Metadata, error) {
	if !safeToken(account) || !safeToken(shortcode) {
		return Metadata{}, fmt.Errorf("invalid Workable posting identity")
	}
	if client == nil {
		client = &http.Client{}
	}
	boundedClient := *client
	boundedClient.Timeout = 10 * time.Second
	boundedClient.Jar = nil
	boundedClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	endpoint := "https://" + workablePublicPostingHost + "/api/v1/widget/accounts/" +
		url.PathEscape(account) + "?details=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Metadata{}, fmt.Errorf("create Workable posting request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := boundedClient.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch Workable public postings: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("Workable public posting endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostingResponse+1))
	if err != nil {
		return Metadata{}, fmt.Errorf("read Workable public postings: %w", err)
	}
	if len(body) > maxPostingResponse {
		return Metadata{}, fmt.Errorf("Workable public posting response exceeds 4 MiB")
	}
	return extractWorkableMetadata(body, account, shortcode)
}

func extractWorkableMetadata(data []byte, account, requestedShortcode string) (Metadata, error) {
	if !safeToken(account) || !safeToken(requestedShortcode) {
		return Metadata{}, fmt.Errorf("invalid Workable posting identity")
	}
	if !utf8.Valid(data) {
		return Metadata{}, fmt.Errorf("Workable response is not valid UTF-8")
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return Metadata{}, fmt.Errorf("Workable response contains NUL")
	}
	var response workableAccount
	if err := json.Unmarshal(data, &response); err != nil {
		return Metadata{}, fmt.Errorf("decode Workable public postings: %w", err)
	}

	selected := -1
	for i := range response.Jobs {
		if response.Jobs[i].Shortcode != requestedShortcode {
			continue
		}
		if selected >= 0 {
			return Metadata{}, fmt.Errorf("Workable posting identity is ambiguous for account %q and shortcode %q", account, requestedShortcode)
		}
		selected = i
	}
	if selected < 0 {
		return Metadata{}, fmt.Errorf("Workable posting %q was not found on public account %q", requestedShortcode, account)
	}

	posting := response.Jobs[selected]
	title := workablePublicText(posting.Title)
	if title == "" {
		return Metadata{}, fmt.Errorf("Workable selected posting %q has no title", requestedShortcode)
	}
	if len(title) > 512 {
		return Metadata{}, fmt.Errorf("Workable selected posting %q has a title larger than 512 bytes", requestedShortcode)
	}
	company := workablePublicText(response.Name)
	if len(company) > 512 {
		return Metadata{}, fmt.Errorf("Workable public account %q has a company name larger than 512 bytes", account)
	}
	descriptionHTML := posting.Description
	if !utf8.ValidString(descriptionHTML) || strings.IndexByte(descriptionHTML, 0) >= 0 {
		return Metadata{}, fmt.Errorf("Workable selected posting %q has invalid description text", requestedShortcode)
	}
	location := workableLocationSummary(posting)
	workplace := workableWorkplaceType(posting)
	employment := workablePublicText(posting.EmploymentType)
	metadata := Metadata{
		Title:   title,
		Company: company,
	}
	if len(descriptionHTML) > MaxPostingSourceInputBytes {
		// The source envelope has no separate facts field, so an over-limit
		// description keeps the normal title/company projection only.
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	canonical := descriptionText(descriptionHTML)
	metadata.Description = canonical
	metadata.Source = workablePostingSource(account, requestedShortcode, descriptionHTML, canonical, location, workplace, employment)
	return metadata, nil
}

func workablePostingSource(account, shortcode, descriptionHTML, canonical, location, workplace, employment string) PostingSourceResult {
	if strings.TrimSpace(canonical) == "" && location == "" && workplace == "" && employment == "" {
		return UnavailablePostingSource(PostingSourceAbsent)
	}
	source, err := SealPostingSource(PostingSource{
		Version: PostingSourceFormatVersion,
		Identity: PostingIdentity{
			Provider:  "workable",
			Board:     account,
			PostingID: shortcode,
		},
		DescriptionHTML: descriptionHTML,
		Location:        location,
		WorkplaceType:   workplace,
		EmploymentType:  employment,
	}, canonical)
	if err != nil {
		if errors.Is(err, ErrPostingSourceTooLarge) {
			return UnavailablePostingSource(PostingSourceTooLarge)
		}
		return UnavailablePostingSource(PostingSourceInvalid)
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		return UnavailablePostingSource(PostingSourceInvalid)
	}
	if len(encoded) > MaxPostingSourceJSONBytes {
		return UnavailablePostingSource(PostingSourceTooLarge)
	}
	return AvailablePostingSource(source)
}

func workableLocationSummary(posting workablePosting) string {
	if len(posting.Locations) > 0 {
		locations := make([]string, 0, len(posting.Locations))
		for _, location := range posting.Locations {
			if location.Hidden {
				continue
			}
			label := workableJoinLocation(
				location.City,
				workableFirstText(location.Region, location.State),
				workableFirstText(location.CountryName, location.Country),
			)
			if label != "" {
				locations = append(locations, label)
			}
		}
		if len(locations) > 0 {
			return strings.Join(locations, "; ")
		}
	}
	return workableJoinLocation(posting.City, posting.State, posting.Country)
}

func workableJoinLocation(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value = workablePublicText(value); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, ", ")
}

func workableFirstText(values ...string) string {
	for _, value := range values {
		if value = workablePublicText(value); value != "" {
			return value
		}
	}
	return ""
}

func workableWorkplaceType(posting workablePosting) string {
	if value := workablePublicText(posting.WorkplaceType); value != "" {
		return value
	}
	if posting.Telecommuting != nil && *posting.Telecommuting {
		return "remote"
	}
	return ""
}

func workablePublicText(value string) string {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
