package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

func parseLeverURL(rawURL string) (site, postingID, apiHost string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("parse Lever URL: %w", err)
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return "", "", "", fmt.Errorf("unsupported Lever URL")
	}
	switch u.Hostname() {
	case "jobs.lever.co":
		apiHost = "api.lever.co"
	case "jobs.eu.lever.co":
		apiHost = "api.eu.lever.co"
	default:
		return "", "", "", fmt.Errorf("unsupported Lever URL")
	}
	parts := pathSegments(u.Path)
	if len(parts) != 2 || !safeToken(parts[0]) || !safeToken(parts[1]) {
		return "", "", "", fmt.Errorf("invalid Lever posting path")
	}
	return parts[0], parts[1], apiHost, nil
}

type leverSalaryRange struct {
	Currency string  `json:"currency"`
	Interval string  `json:"interval"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
}

func (r *leverSalaryRange) display() string {
	if r == nil || r.Min <= 0 || r.Max < r.Min || math.IsNaN(r.Min) || math.IsNaN(r.Max) || math.IsInf(r.Min, 0) || math.IsInf(r.Max, 0) ||
		r.Currency == "" || r.Interval == "" || len(r.Currency) != 3 {
		return ""
	}
	for _, c := range r.Currency {
		if c < 'A' || c > 'Z' {
			return ""
		}
	}
	// Preserve the vendor interval instead of guessing annual versus hourly pay.
	amount := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return r.Currency + " " + amount(r.Min) + "–" + amount(r.Max) + " / " + r.Interval
}

type leverPosting struct {
	ID                   string `json:"id"`
	Text                 string `json:"text"`
	DescriptionPlain     string `json:"descriptionPlain"`
	DescriptionHTML      string `json:"description"`
	DescriptionBody      string `json:"descriptionBody"`
	DescriptionBodyPlain string `json:"descriptionBodyPlain"`
	OpeningPlain         string `json:"openingPlain"`
	OpeningHTML          string `json:"opening"`
	AdditionalPlain      string `json:"additionalPlain"`
	AdditionalHTML       string `json:"additional"`
	Categories           struct {
		Location   string `json:"location"`
		Commitment string `json:"commitment"`
	} `json:"categories"`
	WorkplaceType string            `json:"workplaceType"`
	SalaryRange   *leverSalaryRange `json:"salaryRange"`
	Lists         []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
}

func fetchLever(ctx context.Context, client *http.Client, site, postingID, apiHost string) (Metadata, error) {
	if apiHost != "api.lever.co" && apiHost != "api.eu.lever.co" {
		return Metadata{}, fmt.Errorf("unsupported Lever API host")
	}
	if !safeToken(site) || !safeToken(postingID) {
		return Metadata{}, fmt.Errorf("invalid Lever posting identity")
	}
	endpoint := "https://" + apiHost + "/v0/postings/" +
		url.PathEscape(site) + "/" + url.PathEscape(postingID)
	data, err := getPostingJSON(ctx, client, endpoint)
	if err != nil {
		return Metadata{}, err
	}
	var posting leverPosting
	if err := json.Unmarshal(data, &posting); err != nil {
		return Metadata{}, fmt.Errorf("decode Lever posting: %w", err)
	}
	if strings.TrimSpace(posting.Text) == "" {
		return Metadata{}, fmt.Errorf("Lever posting has no title")
	}
	if strings.TrimSpace(posting.ID) != postingID {
		return Metadata{}, fmt.Errorf("Lever posting identity mismatch: got %q, want %q", posting.ID, postingID)
	}
	description := leverCanonicalDescription(posting)
	salary := posting.SalaryRange.display()
	metadata := completeMetadata(Metadata{
		Title:       strings.TrimSpace(posting.Text),
		Company:     boardNameSuggestion(site),
		Description: description,
		SalaryText:  salary,
	})
	metadata.Source = leverPostingSource(posting, site, postingID, apiHost, metadata.Description)
	return metadata, nil
}

// leverCanonicalDescription preserves the existing canonical projection:
// combined plain text wins, then combined HTML, then opening/body fallback;
// named lists and closing text follow in provider order.
func leverCanonicalDescription(posting leverPosting) string {
	description := strings.TrimSpace(posting.DescriptionPlain)
	if description == "" {
		// Published Lever postings sometimes omit the plain-text field while
		// still returning the populated HTML description.
		description = descriptionText(posting.DescriptionHTML)
	}
	if description == "" {
		description = strings.TrimSpace(posting.OpeningPlain)
		if description == "" {
			description = descriptionText(posting.OpeningHTML)
		}
		if body := descriptionText(posting.DescriptionBody); body != "" {
			if description != "" {
				description += "\n\n"
			}
			description += body
		}
	}
	sections := []string{}
	if description != "" {
		sections = append(sections, description)
	}
	for _, list := range posting.Lists {
		content := descriptionText(list.Content)
		if content != "" {
			sections = append(sections, strings.TrimSpace(list.Text)+"\n"+content)
		}
	}
	additional := strings.TrimSpace(posting.AdditionalPlain)
	if additional == "" {
		additional = descriptionText(posting.AdditionalHTML)
	}
	if additional != "" {
		sections = append(sections, additional)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func leverPostingSource(posting leverPosting, site, postingID, apiHost, canonical string) PostingSourceResult {
	htmlSource := leverSourceHTML(posting)
	plainSource := canonical
	location := leverDirectFact(posting.Categories.Location)
	workplace := leverDirectFact(posting.WorkplaceType)
	employment := leverDirectFact(posting.Categories.Commitment)
	salary := leverDirectFact(posting.SalaryRange.display())
	if htmlSource == "" && plainSource == "" && location == "" && workplace == "" && employment == "" && salary == "" {
		return UnavailablePostingSource(PostingSourceAbsent)
	}
	if len(htmlSource)+len(plainSource) > MaxPostingSourceInputBytes {
		return UnavailablePostingSource(PostingSourceTooLarge)
	}
	source, err := SealPostingSource(PostingSource{
		Version:          PostingSourceFormatVersion,
		Identity:         PostingIdentity{Provider: "lever", Region: leverRegion(apiHost), Board: site, PostingID: postingID},
		DescriptionHTML:  htmlSource,
		DescriptionPlain: plainSource,
		Location:         location,
		WorkplaceType:    workplace,
		EmploymentType:   employment,
		SalarySummary:    salary,
	}, canonical)
	if err != nil {
		if errors.Is(err, ErrPostingSourceTooLarge) {
			return UnavailablePostingSource(PostingSourceTooLarge)
		}
		return UnavailablePostingSource(PostingSourceInvalid)
	}
	jobHost := "jobs.lever.co"
	if apiHost == "api.eu.lever.co" {
		jobHost = "jobs.eu.lever.co"
	}
	savedPostingURL := "https://" + jobHost + "/" + url.PathEscape(site) + "/" + url.PathEscape(postingID)
	if _, err := EncodePostingSource(source, savedPostingURL, canonical); err != nil {
		if errors.Is(err, ErrPostingSourceTooLarge) {
			return UnavailablePostingSource(PostingSourceTooLarge)
		}
		return UnavailablePostingSource(PostingSourceInvalid)
	}
	return AvailablePostingSource(source)
}

func leverRegion(apiHost string) string {
	if apiHost == "api.eu.lever.co" {
		return "eu"
	}
	return "global"
}

func leverSourceHTML(posting leverPosting) string {
	base := strings.TrimSpace(posting.DescriptionHTML)
	if base == "" {
		switch {
		case strings.TrimSpace(posting.DescriptionPlain) != "":
			base = leverPlainTextHTML(posting.DescriptionPlain)
		default:
			opening := strings.TrimSpace(posting.OpeningHTML)
			if opening == "" {
				opening = leverPlainTextHTML(posting.OpeningPlain)
			}
			body := strings.TrimSpace(posting.DescriptionBody)
			if body == "" && strings.TrimSpace(posting.DescriptionBodyPlain) != "" {
				body = leverPlainTextHTML(posting.DescriptionBodyPlain)
			}
			base = joinLeverHTML(opening, body)
		}
	}

	var b strings.Builder
	b.WriteString(base)
	for _, list := range posting.Lists {
		name := strings.TrimSpace(list.Text)
		content := strings.TrimSpace(list.Content)
		if name == "" && content == "" {
			continue
		}
		b.WriteString("<section>")
		if name != "" {
			b.WriteString("<h2>")
			b.WriteString(html.EscapeString(name))
			b.WriteString("</h2>")
		}
		if content != "" {
			b.WriteString(leverListHTML(content))
		}
		b.WriteString("</section>")
	}
	additional := strings.TrimSpace(posting.AdditionalHTML)
	if additional == "" {
		additional = leverPlainTextHTML(posting.AdditionalPlain)
	}
	b.WriteString(additional)
	return b.String()
}

func joinLeverHTML(left, right string) string {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	switch {
	case left == "":
		return right
	case right == "":
		return left
	default:
		return left + right
	}
}

func leverPlainTextHTML(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, paragraph := range strings.Split(value, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(html.EscapeString(paragraph), "\n", "<br>"))
		b.WriteString("</p>")
	}
	return b.String()
}

func leverListHTML(content string) string {
	trimmed := strings.TrimSpace(content)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "<ul") || strings.HasPrefix(lower, "<ol") {
		return trimmed
	}
	if strings.HasPrefix(lower, "<li") {
		return "<ul>" + trimmed + "</ul>"
	}
	return trimmed
}

func leverDirectFact(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 512 || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	return value
}
