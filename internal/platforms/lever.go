package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

type leverPosting struct {
	Text             string `json:"text"`
	DescriptionPlain string `json:"descriptionPlain"`
	AdditionalPlain  string `json:"additionalPlain"`
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
	sections := []string{strings.TrimSpace(posting.DescriptionPlain)}
	for _, list := range posting.Lists {
		content := descriptionText(list.Content)
		if content != "" {
			sections = append(sections, strings.TrimSpace(list.Text)+"\n"+content)
		}
	}
	if additional := strings.TrimSpace(posting.AdditionalPlain); additional != "" {
		sections = append(sections, additional)
	}
	return Metadata{
		Title:       strings.TrimSpace(posting.Text),
		Description: strings.TrimSpace(strings.Join(sections, "\n\n")),
	}, nil
}
