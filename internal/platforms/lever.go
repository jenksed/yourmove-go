package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	Text             string            `json:"text"`
	DescriptionPlain string            `json:"descriptionPlain"`
	DescriptionHTML  string            `json:"description"`
	DescriptionBody  string            `json:"descriptionBody"`
	OpeningPlain     string            `json:"openingPlain"`
	OpeningHTML      string            `json:"opening"`
	AdditionalPlain  string            `json:"additionalPlain"`
	AdditionalHTML   string            `json:"additional"`
	SalaryRange      *leverSalaryRange `json:"salaryRange"`
	Lists            []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
}

func fetchLever(ctx context.Context, client *http.Client, site, postingID, apiHost string) (Metadata, error) {
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
	return Metadata{
		Title:       strings.TrimSpace(posting.Text),
		Company:     boardNameSuggestion(site),
		Description: strings.TrimSpace(strings.Join(sections, "\n\n")),
		SalaryText:  posting.SalaryRange.display(),
	}, nil
}
