package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	smartRecruitersJobHost = "jobs.smartrecruiters.com"
	smartRecruitersAPIHost = "api.smartrecruiters.com"
)

type smartRecruitersSection struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type smartRecruitersSections struct {
	CompanyDescription    smartRecruitersSection `json:"companyDescription"`
	JobDescription        smartRecruitersSection `json:"jobDescription"`
	Qualifications        smartRecruitersSection `json:"qualifications"`
	AdditionalInformation smartRecruitersSection `json:"additionalInformation"`
}

type smartRecruitersCompany struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}

type smartRecruitersLocation struct {
	City         string `json:"city"`
	Region       string `json:"region"`
	Country      string `json:"country"`
	FullLocation string `json:"fullLocation"`
	Remote       *bool  `json:"remote"`
	Hybrid       *bool  `json:"hybrid"`
}

type smartRecruitersEmployment struct {
	Label string `json:"label"`
}

type smartRecruitersPosting struct {
	ID               string                    `json:"id"`
	UUID             string                    `json:"uuid"`
	Name             string                    `json:"name"`
	Company          smartRecruitersCompany    `json:"company"`
	Location         smartRecruitersLocation   `json:"location"`
	TypeOfEmployment smartRecruitersEmployment `json:"typeOfEmployment"`
	Compensation     json.RawMessage           `json:"compensation"`
	JobAd            struct {
		Sections smartRecruitersSections `json:"sections"`
	} `json:"jobAd"`
	PostingURL string `json:"postingUrl"`
	Active     *bool  `json:"active"`
	Visibility string `json:"visibility"`
}

type smartRecruitersCompensation struct {
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Currency string   `json:"currency"`
	Period   string   `json:"period"`
}

type smartRecruitersSectionSpec struct {
	key          string
	defaultTitle string
	section      smartRecruitersSection
}

// parseSmartRecruitersURL accepts only the provider's public employer posting
// route. Query tracking and fragments do not participate in posting identity.
func parseSmartRecruitersURL(rawURL string) (company, postingID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse SmartRecruiters URL: %w", err)
	}
	if u.Scheme != "https" || !strings.EqualFold(u.Hostname(), smartRecruitersJobHost) ||
		u.Port() != "" || u.User != nil || u.RawPath != "" || u.Opaque != "" {
		return "", "", fmt.Errorf("unsupported SmartRecruiters URL")
	}
	if !strings.HasPrefix(u.Path, "/") || strings.HasSuffix(u.Path, "/") {
		return "", "", fmt.Errorf("invalid SmartRecruiters posting path")
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) != 2 || !safeToken(parts[0]) {
		return "", "", fmt.Errorf("invalid SmartRecruiters posting path")
	}

	identitySegment := parts[1]
	postingID = identitySegment
	if prefix, slug, hasSlug := strings.Cut(identitySegment, "-"); hasSlug {
		if !smartRecruitersSlug(slug) {
			return "", "", fmt.Errorf("invalid SmartRecruiters posting slug")
		}
		postingID = prefix
	}
	parsedID, parseErr := strconv.ParseUint(postingID, 10, 64)
	if parseErr != nil || parsedID == 0 {
		return "", "", fmt.Errorf("invalid SmartRecruiters posting ID")
	}
	return parts[0], postingID, nil
}

func smartRecruitersSlug(value string) bool {
	if value == "" || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	hasAlphaNumeric := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			hasAlphaNumeric = true
			continue
		}
		if r != '-' && r != '_' {
			return false
		}
	}
	return hasAlphaNumeric
}

func fetchSmartRecruiters(ctx context.Context, client *http.Client, company, postingID string) (Metadata, error) {
	if client == nil {
		return Metadata{}, fmt.Errorf("SmartRecruiters posting client is nil")
	}
	if !safeToken(company) {
		return Metadata{}, fmt.Errorf("invalid SmartRecruiters company identity")
	}
	parsedID, err := strconv.ParseUint(postingID, 10, 64)
	if err != nil || parsedID == 0 {
		return Metadata{}, fmt.Errorf("invalid SmartRecruiters posting ID")
	}
	endpoint := "https://" + smartRecruitersAPIHost + "/v1/companies/" +
		url.PathEscape(company) + "/postings/" + url.PathEscape(postingID)
	data, err := getPostingJSON(ctx, client, endpoint)
	if err != nil {
		return Metadata{}, err
	}
	return extractSmartRecruitersMetadata(data, company, postingID)
}

func extractSmartRecruitersMetadata(data []byte, requestedCompany, requestedID string) (Metadata, error) {
	var posting smartRecruitersPosting
	if err := json.Unmarshal(data, &posting); err != nil {
		return Metadata{}, fmt.Errorf("decode SmartRecruiters posting: %w", err)
	}
	if posting.Company.Identifier != requestedCompany {
		return Metadata{}, fmt.Errorf("SmartRecruiters company identity mismatch: got %q, want %q", posting.Company.Identifier, requestedCompany)
	}
	if posting.ID != requestedID {
		return Metadata{}, fmt.Errorf("SmartRecruiters posting identity mismatch: got %q, want %q", posting.ID, requestedID)
	}
	if posting.UUID != "" && !smartRecruitersUUID(posting.UUID) {
		return Metadata{}, fmt.Errorf("SmartRecruiters response has an invalid posting UUID")
	}
	if posting.PostingURL != "" {
		returnedCompany, returnedID, parseErr := parseSmartRecruitersURL(posting.PostingURL)
		if parseErr != nil || returnedCompany != requestedCompany || returnedID != requestedID {
			return Metadata{}, fmt.Errorf("SmartRecruiters returned posting URL does not match requested identity")
		}
	}
	if posting.Active == nil {
		return Metadata{}, fmt.Errorf("SmartRecruiters posting has no active status")
	}
	if !*posting.Active {
		return Metadata{}, fmt.Errorf("%w: SmartRecruiters posting is unpublished", ErrClosed)
	}
	if !strings.EqualFold(strings.TrimSpace(posting.Visibility), "PUBLIC") {
		return Metadata{}, fmt.Errorf("SmartRecruiters posting is not public")
	}

	title := smartRecruitersDirectFact(posting.Name, 512)
	companyName := smartRecruitersDirectFact(posting.Company.Name, 512)
	if title == "" {
		return Metadata{}, fmt.Errorf("SmartRecruiters posting has no title")
	}
	if companyName == "" {
		return Metadata{}, fmt.Errorf("SmartRecruiters posting has no company name")
	}

	htmlSource, err := smartRecruitersSectionsHTML(posting.JobAd.Sections)
	if err != nil {
		return Metadata{}, fmt.Errorf("SmartRecruiters job ad sections: %w", err)
	}
	location := smartRecruitersLocationName(posting.Location)
	workplace := smartRecruitersWorkplaceType(posting.Location.Remote, posting.Location.Hybrid)
	employment := smartRecruitersDirectFact(posting.TypeOfEmployment.Label, 256)
	salary := smartRecruitersSalarySummary(posting.Compensation)

	metadata := Metadata{Title: title, Company: companyName, SalaryText: salary}
	if len(htmlSource) > MaxPostingSourceInputBytes {
		metadata = completeMetadata(metadata)
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	metadata.Description = descriptionText(htmlSource)
	if len(metadata.Description) > MaxPostingSourceInputBytes {
		metadata.Description = ""
		metadata = completeMetadata(metadata)
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	metadata = completeMetadata(metadata)

	if len(htmlSource)+len(metadata.Description) > MaxPostingSourceInputBytes {
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	if htmlSource == "" && metadata.Description == "" && location == "" && workplace == "" && employment == "" && salary == "" {
		metadata.Source = UnavailablePostingSource(PostingSourceAbsent)
		return metadata, nil
	}

	source, err := SealPostingSource(PostingSource{
		Identity: PostingIdentity{
			Provider:  "smartrecruiters",
			Board:     requestedCompany,
			PostingID: requestedID,
		},
		DescriptionHTML:  htmlSource,
		DescriptionPlain: metadata.Description,
		Location:         location,
		WorkplaceType:    workplace,
		EmploymentType:   employment,
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
	encoded, err := json.Marshal(source)
	if err != nil {
		metadata.Source = UnavailablePostingSource(PostingSourceInvalid)
		return metadata, nil
	}
	if len(encoded) > MaxPostingSourceJSONBytes {
		metadata.Source = UnavailablePostingSource(PostingSourceTooLarge)
		return metadata, nil
	}
	metadata.Source = AvailablePostingSource(source)
	return metadata, nil
}

func smartRecruitersUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
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

func smartRecruitersSectionsHTML(sections smartRecruitersSections) (string, error) {
	ordered := []smartRecruitersSectionSpec{
		{key: "companyDescription", defaultTitle: "Company Description", section: sections.CompanyDescription},
		{key: "jobDescription", defaultTitle: "Job Description", section: sections.JobDescription},
		{key: "qualifications", defaultTitle: "Qualifications", section: sections.Qualifications},
		{key: "additionalInformation", defaultTitle: "Additional Information", section: sections.AdditionalInformation},
	}
	var out strings.Builder
	for _, item := range ordered {
		text := strings.TrimSpace(item.section.Text)
		if text == "" {
			continue
		}
		if !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
			return "", fmt.Errorf("%s contains invalid text", item.key)
		}
		title := smartRecruitersDirectFact(item.section.Title, 256)
		if title == "" {
			title = item.defaultTitle
		}
		out.WriteString("<section><h2>")
		out.WriteString(stdhtml.EscapeString(title))
		out.WriteString("</h2>")
		out.WriteString(text)
		out.WriteString("</section>")
		if out.Len() > MaxPostingSourceInputBytes {
			return out.String(), nil
		}
	}
	return out.String(), nil
}

func smartRecruitersDirectFact(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func smartRecruitersLocationName(location smartRecruitersLocation) string {
	if full := smartRecruitersDirectFact(location.FullLocation, 512); full != "" {
		parts := strings.Split(full, ",")
		clean := make([]string, 0, len(parts))
		for _, part := range parts {
			if part = strings.TrimSpace(part); part != "" {
				clean = append(clean, part)
			}
		}
		if len(clean) > 0 {
			return strings.Join(clean, ", ")
		}
	}
	parts := []string{
		smartRecruitersDirectFact(location.City, 256),
		smartRecruitersDirectFact(location.Region, 256),
		smartRecruitersDirectFact(location.Country, 128),
	}
	clean := parts[:0]
	for _, part := range parts {
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, ", ")
}

func smartRecruitersWorkplaceType(remote, hybrid *bool) string {
	if remote != nil && hybrid != nil && *remote && *hybrid {
		return ""
	}
	if remote != nil && *remote {
		return "Remote"
	}
	if hybrid != nil && *hybrid {
		return "Hybrid"
	}
	return ""
}

func smartRecruitersSalarySummary(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var compensation smartRecruitersCompensation
	if json.Unmarshal(raw, &compensation) != nil || compensation.Min == nil || compensation.Max == nil {
		return ""
	}
	period := strings.ToLower(strings.TrimSpace(compensation.Period))
	switch period {
	case "hourly", "daily", "weekly", "monthly", "yearly":
	default:
		return ""
	}
	currency := strings.ToUpper(strings.TrimSpace(compensation.Currency))
	if *compensation.Min <= 0 || *compensation.Max < *compensation.Min ||
		math.IsNaN(*compensation.Min) || math.IsNaN(*compensation.Max) ||
		math.IsInf(*compensation.Min, 0) || math.IsInf(*compensation.Max, 0) || len(currency) != 3 {
		return ""
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return ""
		}
	}
	format := func(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
	return currency + " " + format(*compensation.Min) + "–" + format(*compensation.Max) + " / " + period
}
