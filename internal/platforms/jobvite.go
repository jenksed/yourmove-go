package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

const (
	jobviteMaxHTMLBytes  = 1 << 20
	jobviteMaxHTMLTokens = 100_000
	jobviteMaxFactBytes  = 500
)

type jobviteJSONLDPosting struct {
	Type               json.RawMessage `json:"@type"`
	Identifier         json.RawMessage `json:"identifier"`
	URL                string          `json:"url"`
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	HiringOrganization json.RawMessage `json:"hiringOrganization"`
	EmploymentType     json.RawMessage `json:"employmentType"`
	JobLocationType    json.RawMessage `json:"jobLocationType"`
	JobLocation        json.RawMessage `json:"jobLocation"`
	BaseSalary         json.RawMessage `json:"baseSalary"`
}

type jobvitePlace struct {
	Address struct {
		Locality string          `json:"addressLocality"`
		Region   string          `json:"addressRegion"`
		Country  json.RawMessage `json:"addressCountry"`
	} `json:"address"`
}

// parseJobviteURL accepts only a hosted public detail URL. Query parameters
// are preserved by intake but do not contribute to posting identity.
func parseJobviteURL(rawURL string) (site, postingID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse Jobvite URL: %w", err)
	}
	if u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "jobs.jobvite.com") ||
		u.Port() != "" || u.User != nil || u.RawPath != "" || u.Opaque != "" {
		return "", "", fmt.Errorf("unsupported Jobvite URL")
	}
	if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || strings.HasSuffix(u.Path, "/") {
		return "", "", fmt.Errorf("invalid Jobvite posting path")
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) != 3 || parts[1] != "job" || !safeToken(parts[0]) || !safeToken(parts[2]) {
		return "", "", fmt.Errorf("invalid Jobvite posting path")
	}
	return parts[0], parts[2], nil
}

func fetchJobvite(ctx context.Context, site, postingID string) (Metadata, error) {
	if !safeToken(site) || !safeToken(postingID) {
		return Metadata{}, fmt.Errorf("invalid Jobvite posting identity")
	}
	endpoint := "https://jobs.jobvite.com/" + url.PathEscape(site) + "/job/" + url.PathEscape(postingID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Metadata{}, fmt.Errorf("create Jobvite request: %w", err)
	}
	req.Header.Set("Accept", "text/html, application/xhtml+xml")
	client := postingClient()
	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch Jobvite posting: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return Metadata{}, fmt.Errorf("%w: Jobvite posting page returned HTTP %d", ErrClosed, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("Jobvite posting page returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, jobviteMaxHTMLBytes+1))
	if err != nil {
		return Metadata{}, fmt.Errorf("read Jobvite posting page: %w", err)
	}
	if len(body) > jobviteMaxHTMLBytes {
		return Metadata{}, fmt.Errorf("Jobvite posting page exceeds 1 MiB")
	}
	return extractJobviteMetadata(body, site, postingID)
}

func extractJobviteMetadata(body []byte, site, postingID string) (Metadata, error) {
	if !safeToken(site) || !safeToken(postingID) {
		return Metadata{}, fmt.Errorf("invalid Jobvite posting identity")
	}
	posting, err := jobvitePostingFromHTML(body)
	if err != nil {
		return Metadata{}, err
	}
	identifier, err := jobviteIdentifier(posting.Identifier)
	if err != nil {
		return Metadata{}, fmt.Errorf("Jobvite posting identifier: %w", err)
	}
	if identifier != postingID {
		return Metadata{}, fmt.Errorf("Jobvite posting identity mismatch: got %q, want %q", identifier, postingID)
	}
	if posting.URL != "" {
		if err := jobviteURLMatchesIdentity(posting.URL, site, postingID); err != nil {
			return Metadata{}, fmt.Errorf("Jobvite posting URL identity: %w", err)
		}
	}

	title := jobviteDirectFact(posting.Title)
	if title == "" {
		return Metadata{}, fmt.Errorf("Jobvite posting has no title")
	}
	descriptionHTML := posting.Description
	if !validJobviteText(descriptionHTML) {
		descriptionHTML = ""
	}
	canonical := descriptionText(descriptionHTML)
	company := jobviteOrganizationName(posting.HiringOrganization)
	employment := jobviteJSONString(posting.EmploymentType)
	workplace := jobviteWorkplaceType(jobviteJSONString(posting.JobLocationType))
	location := jobviteLocations(posting.JobLocation)
	structuredSalary := salaryFromJSONLD(posting.BaseSalary)

	metadata := completeMetadata(Metadata{
		Title:       title,
		Company:     company,
		Description: canonical,
		SalaryText:  structuredSalary,
	})
	if descriptionHTML == "" && location == "" && workplace == "" && employment == "" && structuredSalary == "" {
		metadata.Source = UnavailablePostingSource(PostingSourceAbsent)
		return metadata, nil
	}

	result, err := SealPostingSource(PostingSource{
		Version: PostingSourceFormatVersion,
		Identity: PostingIdentity{
			Provider:  "jobvite",
			Board:     site,
			PostingID: postingID,
		},
		DescriptionHTML: descriptionHTML,
		Location:        location,
		WorkplaceType:   workplace,
		EmploymentType:  employment,
		SalarySummary:   structuredSalary,
	}, metadata.Description)
	if err != nil {
		reason := PostingSourceInvalid
		if errors.Is(err, ErrPostingSourceTooLarge) {
			reason = PostingSourceTooLarge
		}
		metadata.Source = UnavailablePostingSource(reason)
		return metadata, nil
	}
	metadata.Source = AvailablePostingSource(result)
	return metadata, nil
}

func jobvitePostingFromHTML(body []byte) (jobviteJSONLDPosting, error) {
	if len(body) == 0 || len(body) > jobviteMaxHTMLBytes {
		return jobviteJSONLDPosting{}, fmt.Errorf("Jobvite HTML source is empty or exceeds 1 MiB")
	}
	if !utf8.Valid(body) {
		return jobviteJSONLDPosting{}, fmt.Errorf("Jobvite HTML source is not valid UTF-8")
	}

	tokenizer := xhtml.NewTokenizer(bytes.NewReader(body))
	tokens := 0
	postingCount := 0
	var selected jobviteJSONLDPosting
	for {
		tokenType := tokenizer.Next()
		tokens++
		if tokens > jobviteMaxHTMLTokens {
			return jobviteJSONLDPosting{}, fmt.Errorf("Jobvite HTML exceeds token limit")
		}
		if tokenType == xhtml.ErrorToken {
			if tokenizer.Err() == io.EOF {
				break
			}
			return jobviteJSONLDPosting{}, fmt.Errorf("parse Jobvite HTML: %w", tokenizer.Err())
		}
		if tokenType != xhtml.StartTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "script" || !jobviteJSONLDTag(token.Attr) {
			continue
		}

		var script strings.Builder
		for {
			tokenType = tokenizer.Next()
			tokens++
			if tokens > jobviteMaxHTMLTokens {
				return jobviteJSONLDPosting{}, fmt.Errorf("Jobvite HTML exceeds token limit")
			}
			switch tokenType {
			case xhtml.TextToken:
				script.Write(tokenizer.Text())
			case xhtml.EndTagToken:
				if tokenizer.Token().Data == "script" {
					goto scriptComplete
				}
			case xhtml.ErrorToken:
				if tokenizer.Err() != io.EOF {
					return jobviteJSONLDPosting{}, fmt.Errorf("parse Jobvite JSON-LD: %w", tokenizer.Err())
				}
				return jobviteJSONLDPosting{}, fmt.Errorf("unterminated Jobvite JSON-LD script")
			}
		}

	scriptComplete:
		records, err := jobvitePostingRecords([]byte(script.String()))
		if err != nil {
			return jobviteJSONLDPosting{}, fmt.Errorf("decode Jobvite JSON-LD: %w", err)
		}
		for _, record := range records {
			postingCount++
			if postingCount > 1 {
				return jobviteJSONLDPosting{}, fmt.Errorf("Jobvite page has ambiguous JobPosting records")
			}
			selected = record
		}
	}
	if postingCount != 1 {
		return jobviteJSONLDPosting{}, fmt.Errorf("Jobvite page has no unique JobPosting JSON-LD record")
	}
	return selected, nil
}

func jobviteJSONLDTag(attrs []xhtml.Attribute) bool {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Key, "type") {
			mediaType := strings.TrimSpace(strings.SplitN(attr.Val, ";", 2)[0])
			return strings.EqualFold(mediaType, "application/ld+json")
		}
	}
	return false
}

func jobvitePostingRecords(raw []byte) ([]jobviteJSONLDPosting, error) {
	var nodes []json.RawMessage
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty JSON-LD script")
	}
	switch trimmed[0] {
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, err
		}
		if graphRaw, ok := object["@graph"]; ok {
			if err := json.Unmarshal(graphRaw, &nodes); err != nil {
				return nil, fmt.Errorf("decode @graph: %w", err)
			}
		} else {
			nodes = []json.RawMessage{trimmed}
		}
	case '[':
		if err := json.Unmarshal(trimmed, &nodes); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("JSON-LD root is not an object or array")
	}

	var records []jobviteJSONLDPosting
	for _, node := range nodes {
		var typeOnly struct {
			Type json.RawMessage `json:"@type"`
		}
		if err := json.Unmarshal(node, &typeOnly); err != nil {
			return nil, fmt.Errorf("decode JSON-LD node: %w", err)
		}
		if !jobviteHasPostingType(typeOnly.Type) {
			continue
		}
		var record jobviteJSONLDPosting
		if err := json.Unmarshal(node, &record); err != nil {
			return nil, fmt.Errorf("decode JobPosting record: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}

func jobviteHasPostingType(raw json.RawMessage) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == "JobPosting"
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) == nil {
		for _, item := range multiple {
			if item == "JobPosting" {
				return true
			}
		}
	}
	return false
}

func jobviteIdentifier(raw json.RawMessage) (string, error) {
	var value string
	if json.Unmarshal(raw, &value) == nil && value != "" {
		return value, nil
	}
	var property struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(raw, &property) == nil && property.Value != "" {
		return property.Value, nil
	}
	return "", fmt.Errorf("missing or unsupported identifier")
}

func jobviteURLMatchesIdentity(rawURL, site, postingID string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if !parsed.IsAbs() {
		base, _ := url.Parse("https://jobs.jobvite.com/" + url.PathEscape(site) + "/job/" + url.PathEscape(postingID))
		parsed = base.ResolveReference(parsed)
	}
	gotSite, gotID, err := parseJobviteURL(parsed.String())
	if err != nil {
		return err
	}
	if gotSite != site || gotID != postingID {
		return fmt.Errorf("got %q/%q, want %q/%q", gotSite, gotID, site, postingID)
	}
	return nil
}

func jobviteDirectFact(value string) string {
	if !validJobviteText(value) || len(value) > jobviteMaxFactBytes {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func validJobviteText(value string) bool {
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func jobviteJSONString(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return jobviteDirectFact(value)
}

func jobviteWorkplaceType(value string) string {
	switch strings.ToUpper(strings.NewReplacer("_", "", "-", "", " ", "").Replace(value)) {
	case "TELECOMMUTE", "REMOTE":
		return "remote"
	case "HYBRID":
		return "hybrid"
	case "ONSITE":
		return "onsite"
	default:
		return jobviteDirectFact(value)
	}
}

func jobviteOrganizationName(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return jobviteDirectFact(value)
	}
	var organization struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &organization) == nil {
		return jobviteDirectFact(organization.Name)
	}
	return ""
}

func jobviteCountryName(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return jobviteDirectFact(value)
	}
	var country struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &country) == nil {
		return jobviteDirectFact(country.Name)
	}
	return ""
}

func jobviteLocations(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var locations []jobvitePlace
	if err := json.Unmarshal(raw, &locations); err != nil {
		var single jobvitePlace
		if json.Unmarshal(raw, &single) != nil {
			return ""
		}
		locations = []jobvitePlace{single}
	}

	parts := make([]string, 0, len(locations))
	for _, location := range locations {
		components := []string{
			jobviteDirectFact(location.Address.Locality),
			jobviteDirectFact(location.Address.Region),
			jobviteCountryName(location.Address.Country),
		}
		clean := components[:0]
		for _, component := range components {
			if component != "" {
				clean = append(clean, component)
			}
		}
		if len(clean) > 0 {
			parts = append(parts, strings.Join(clean, ", "))
		}
	}
	result := strings.Join(parts, "; ")
	if len(result) > jobviteMaxFactBytes {
		return ""
	}
	return result
}
