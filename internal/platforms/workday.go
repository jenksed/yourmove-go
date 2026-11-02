package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

type workdayPostingURL struct {
	host      string
	tenant    string
	site      string
	locale    string
	postingID string
}

// WorkdayPostingIdentity identifies the supported public Workday detail URL
// families. PostingID retains the complete path below /job/ because a location
// segment may be present and jobReqId is not always the URL's posting key.
func WorkdayPostingIdentity(rawURL string) (PostingIdentity, error) {
	posting, err := parseWorkdayURL(rawURL)
	if err != nil {
		return PostingIdentity{}, err
	}
	return posting.identity(), nil
}

func parseWorkdayURL(rawURL string) (workdayPostingURL, error) {
	if len(rawURL) == 0 || len(rawURL) > 4096 {
		return workdayPostingURL{}, fmt.Errorf("invalid Workday URL size")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return workdayPostingURL{}, fmt.Errorf("parse Workday URL: %w", err)
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" ||
		u.RawPath != "" || !strings.HasPrefix(u.Path, "/") || strings.Contains(u.Path, "//") ||
		strings.Contains(u.Path, `\`) || strings.Contains(u.EscapedPath(), "%") {
		return workdayPostingURL{}, fmt.Errorf("unsupported Workday URL")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return workdayPostingURL{}, fmt.Errorf("unsupported Workday host")
	}

	parts, ok := workdayURLPathSegments(u.Path)
	if !ok {
		return workdayPostingURL{}, fmt.Errorf("invalid Workday posting path")
	}

	posting := workdayPostingURL{host: host}
	switch {
	case workdayJobsHost(host):
		labels := strings.Split(host, ".")
		posting.tenant = labels[0]
		jobIndex := 1
		if len(parts) > 2 && workdayLocale(parts[0]) && parts[2] == "job" {
			posting.locale = parts[0]
			posting.site = parts[1]
			jobIndex = 2
		} else if len(parts) > 1 && parts[1] == "job" {
			posting.site = parts[0]
		} else {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday myworkdayjobs posting path")
		}
		if !safeToken(posting.tenant) || !safeToken(posting.site) {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday tenant or site")
		}
		if jobIndex >= len(parts) || parts[jobIndex] != "job" {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday posting route")
		}
		posting.postingID = strings.Join(parts[jobIndex+1:], "/")
	case workdayMyWorkdaySiteHost(host):
		jobIndex := 3
		if len(parts) >= 6 && workdayLocale(parts[0]) && parts[1] == "recruiting" {
			posting.locale = parts[0]
			posting.tenant = parts[2]
			posting.site = parts[3]
			jobIndex = 4
		} else if len(parts) > 3 && parts[0] == "recruiting" {
			posting.tenant = parts[1]
			posting.site = parts[2]
		} else {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday myworkdaysite recruiting path")
		}
		if !safeToken(posting.tenant) || !safeToken(posting.site) {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday tenant or site")
		}
		if jobIndex >= len(parts) || parts[jobIndex] != "job" {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday posting route")
		}
		posting.postingID = strings.Join(parts[jobIndex+1:], "/")
	default:
		return workdayPostingURL{}, fmt.Errorf("unsupported Workday host")
	}

	posting.postingID = strings.TrimSuffix(posting.postingID, "/")
	postingParts := strings.Split(posting.postingID, "/")
	if len(postingParts) < 1 || len(postingParts) > 2 {
		return workdayPostingURL{}, fmt.Errorf("invalid Workday posting identity path")
	}
	for _, part := range postingParts {
		if !safeToken(part) {
			return workdayPostingURL{}, fmt.Errorf("invalid Workday posting identity path")
		}
	}
	return posting, nil
}

func (posting workdayPostingURL) identity() PostingIdentity {
	return PostingIdentity{
		Provider:  "workday",
		Board:     posting.host + "/" + posting.tenant + "/" + posting.site,
		Region:    posting.locale,
		PostingID: posting.postingID,
	}
}

func (posting workdayPostingURL) detailEndpoint() string {
	parts := []string{"wday", "cxs", posting.tenant, posting.site, "job"}
	for _, part := range strings.Split(posting.postingID, "/") {
		parts = append(parts, part)
	}
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return "https://" + posting.host + "/" + strings.Join(parts, "/")
}

func workdayURLPathSegments(value string) ([]string, bool) {
	if strings.HasSuffix(value, "//") {
		return nil, false
	}
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimSuffix(value, "/")
	if value == "" || strings.Contains(value, "//") {
		return nil, false
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if !safeToken(part) {
			return nil, false
		}
	}
	return parts, true
}

func workdayJobsHost(host string) bool {
	labels := strings.Split(host, ".")
	return len(labels) == 4 && workdayDNSLabel(labels[0]) && workdayPod(labels[1]) &&
		labels[2] == "myworkdayjobs" && labels[3] == "com"
}

func workdayMyWorkdaySiteHost(host string) bool {
	labels := strings.Split(host, ".")
	return len(labels) == 3 && workdayPod(labels[0]) && labels[1] == "myworkdaysite" && labels[2] == "com"
}

func workdayPod(value string) bool {
	if len(value) < 3 || len(value) > 63 || !strings.HasPrefix(value, "wd") {
		return false
	}
	for _, r := range value[2:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func workdayDNSLabel(value string) bool {
	if value == "" || len(value) > 63 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func workdayLocale(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 3 || len(parts[0]) < 2 || len(parts[0]) > 3 {
		return false
	}
	for _, r := range parts[0] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	for _, part := range parts[1:] {
		if len(part) < 2 || len(part) > 8 {
			return false
		}
		for _, r := range part {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return false
			}
		}
	}
	return safeToken(value)
}

type workdayDetailResponse struct {
	JobPostingInfo     *workdayPostingInfo `json:"jobPostingInfo"`
	HiringOrganization json.RawMessage     `json:"hiringOrganization"`
}

type workdayPostingInfo struct {
	Title               workdayOptionalString `json:"title"`
	JobPostingID        workdayOptionalString `json:"jobPostingId"`
	JobDescription      workdayOptionalString `json:"jobDescription"`
	Location            workdayOptionalString `json:"location"`
	AdditionalLocations json.RawMessage       `json:"additionalLocations"`
	RemoteType          workdayOptionalString `json:"remoteType"`
	TimeType            workdayOptionalString `json:"timeType"`
}

// Workday detail payloads vary by employer. A malformed optional string is
// treated as absent while title and posting identity remain required.
type workdayOptionalString string

func (value *workdayOptionalString) UnmarshalJSON(data []byte) error {
	var decoded string
	if json.Unmarshal(data, &decoded) != nil {
		*value = ""
		return nil
	}
	if !utf8.ValidString(decoded) || strings.IndexByte(decoded, 0) >= 0 {
		*value = ""
		return nil
	}
	*value = workdayOptionalString(decoded)
	return nil
}

func fetchWorkday(ctx context.Context, rawURL string) (Metadata, error) {
	return fetchWorkdayWithClient(ctx, rawURL, postingClient())
}

func fetchWorkdayWithClient(ctx context.Context, rawURL string, client *http.Client) (Metadata, error) {
	posting, err := parseWorkdayURL(rawURL)
	if err != nil {
		return Metadata{}, err
	}
	if client == nil {
		return Metadata{}, errors.New("fetch Workday posting: nil HTTP client")
	}
	data, err := getPostingJSON(ctx, client, posting.detailEndpoint())
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch Workday posting: %w", err)
	}
	return extractWorkdayMetadata(data, posting)
}

func extractWorkdayMetadata(data []byte, requested workdayPostingURL) (Metadata, error) {
	var response workdayDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return Metadata{}, fmt.Errorf("decode Workday posting: %w", err)
	}
	if response.JobPostingInfo == nil {
		return Metadata{}, errors.New("Workday response has no jobPostingInfo")
	}
	info := response.JobPostingInfo
	title := workdayDirectFact(string(info.Title))
	if title == "" {
		return Metadata{}, errors.New("Workday posting has no title")
	}
	requestedID := pathLastSegment(requested.postingID)
	returnedID := strings.TrimSpace(string(info.JobPostingID))
	if returnedID == "" || returnedID != requestedID {
		return Metadata{}, fmt.Errorf("Workday posting identity mismatch: got %q, want %q", returnedID, requestedID)
	}

	descriptionHTML := string(info.JobDescription)
	location, tooManyLocations := workdayLocation(info.Location, info.AdditionalLocations)
	workplace := workdayDirectFact(string(info.RemoteType))
	employment := workdayDirectFact(string(info.TimeType))
	metadata := Metadata{
		Title:   title,
		Company: workdayHiringOrganizationName(response.HiringOrganization),
	}
	if len(descriptionHTML) > MaxPostingSourceInputBytes {
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	metadata.Description = descriptionText(descriptionHTML)
	if strings.TrimSpace(descriptionHTML) == "" && location == "" && workplace == "" && employment == "" && !tooManyLocations {
		metadata.Source = UnavailablePostingSource(PostingSourceAbsent)
		return metadata, nil
	}
	if tooManyLocations {
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	source, err := SealPostingSource(PostingSource{
		Version:         PostingSourceFormatVersion,
		Identity:        requested.identity(),
		DescriptionHTML: descriptionHTML,
		Location:        location,
		WorkplaceType:   workplace,
		EmploymentType:  employment,
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

func workdayHiringOrganizationName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var organization struct {
		Name workdayOptionalString `json:"name"`
	}
	if err := json.Unmarshal(raw, &organization); err != nil {
		return ""
	}
	return workdayDirectFact(string(organization.Name))
}

func pathLastSegment(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func workdayLocation(primary workdayOptionalString, additionalRaw json.RawMessage) (string, bool) {
	locations := make([]string, 0, 4)
	if value := workdayDirectFact(string(primary)); value != "" {
		locations = append(locations, value)
	}
	if len(additionalRaw) > 0 && !bytes.Equal(bytes.TrimSpace(additionalRaw), []byte("null")) {
		decoder := json.NewDecoder(bytes.NewReader(additionalRaw))
		opening, err := decoder.Token()
		if err != nil || opening != json.Delim('[') {
			return strings.Join(locations, "; "), false
		}
		for decoder.More() {
			if len(locations) >= 256 {
				return "", true
			}
			var value workdayOptionalString
			if err := decoder.Decode(&value); err != nil {
				return strings.Join(locations, "; "), false
			}
			fact := workdayDirectFact(string(value))
			if fact != "" {
				locations = append(locations, fact)
			}
		}
		if _, err := decoder.Token(); err != nil {
			return strings.Join(locations, "; "), false
		}
	}
	return strings.Join(locations, "; "), false
}

func workdayDirectFact(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 || strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	return value
}
