package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// ICIMSPostingIdentity covers classic vendor-hosted job pages only. Escaped
// punctuation in the decorative slug is harmless; escaped separators are not.
func ICIMSPostingIdentity(raw string) (PostingIdentity, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" {
		return PostingIdentity{}, fmt.Errorf("unsupported iCIMS posting URL")
	}
	host := strings.ToLower(u.Hostname())
	tenant := strings.TrimSuffix(host, ".icims.com")
	if tenant == host || !safeToken(tenant) || strings.Contains(tenant, ".") {
		return PostingIdentity{}, fmt.Errorf("unsupported iCIMS posting host")
	}
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.Contains(escaped, "%00") {
		return PostingIdentity{}, fmt.Errorf("unsafe iCIMS path")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if (len(parts) != 3 && len(parts) != 4) || parts[0] != "jobs" || !decimalPostingID.MatchString(parts[1]) || parts[len(parts)-1] != "job" {
		return PostingIdentity{}, fmt.Errorf("unsupported iCIMS classic posting path")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00\r\n") {
			return PostingIdentity{}, fmt.Errorf("unsafe iCIMS path segment")
		}
	}
	return PostingIdentity{Provider: "icims", Board: host, PostingID: parts[1]}, nil
}
func fetchICIMS(ctx context.Context, client *http.Client, raw string) (Metadata, error) {
	identity, err := ICIMSPostingIdentity(raw)
	if err != nil {
		return Metadata{}, err
	}
	// This publicly served detail variant was independently verified against two
	// employers. Never read the credentialed api.icims.com Job Portal API.
	endpoint := "https://" + identity.Board + "/jobs/" + identity.PostingID + "/job?in_iframe=1"
	data, err := readPublicJobFeed(ctx, client, endpoint)
	if err != nil {
		return Metadata{}, err
	}
	return extractICIMSMetadata(data, identity)
}

type icimsPublicPosting struct {
	Type         string `json:"@type"`
	URL          string `json:"url"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Employment   string `json:"employmentType"`
	LocationType string `json:"jobLocationType"`
	Organization struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
	Locations []struct {
		Address struct {
			City    string `json:"addressLocality"`
			Region  string `json:"addressRegion"`
			Country string `json:"addressCountry"`
		} `json:"address"`
	} `json:"jobLocation"`
}

func extractICIMSMetadata(data []byte, identity PostingIdentity) (Metadata, error) {
	if len(data) > maxPostingResponse {
		return Metadata{}, fmt.Errorf("iCIMS public page exceeds limit")
	}
	tokenizer := html.NewTokenizer(bytes.NewReader(data))
	var selected *icimsPublicPosting
	scripts := 0
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if tokenizer.Err() != io.EOF {
				return Metadata{}, tokenizer.Err()
			}
			break
		}
		if tokenType != html.StartTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "script" {
			continue
		}
		ld := false
		for _, attribute := range token.Attr {
			if attribute.Key == "type" && strings.EqualFold(strings.TrimSpace(attribute.Val), "application/ld+json") {
				ld = true
			}
		}
		if !ld {
			continue
		}
		scripts++
		if scripts > 16 {
			return Metadata{}, fmt.Errorf("too many iCIMS structured sources")
		}
		if tokenizer.Next() != html.TextToken {
			continue
		}
		payload := tokenizer.Text()
		if len(payload) > MaxPostingSourceInputBytes {
			return Metadata{}, fmt.Errorf("iCIMS structured source exceeds limit")
		}
		var candidate icimsPublicPosting
		if err := json.Unmarshal(payload, &candidate); err != nil {
			continue
		}
		if candidate.Type != "JobPosting" {
			continue
		}
		got, err := ICIMSPostingIdentity(candidate.URL)
		if err != nil || got != identity {
			continue
		}
		if selected != nil {
			return Metadata{}, fmt.Errorf("ambiguous iCIMS posting identity")
		}
		selected = &candidate
	}
	if selected == nil {
		return Metadata{}, fmt.Errorf("iCIMS exact public posting source unavailable")
	}
	canonical := descriptionText(selected.Description)
	metadata := Metadata{Title: workablePublicText(selected.Title), Company: workablePublicText(selected.Organization.Name), Description: canonical}
	if metadata.Title == "" || canonical == "" || len(metadata.Title) > 200 || len(metadata.Company) > 200 {
		return Metadata{}, fmt.Errorf("iCIMS posting lacks bounded readable content")
	}
	clean := func(value string) string {
		if strings.EqualFold(value, "UNAVAILABLE") {
			return ""
		}
		return workablePublicText(value)
	}
	locations := []string{}
	for _, location := range selected.Locations {
		value := workableJoinLocation(clean(location.Address.City), clean(location.Address.Region), clean(location.Address.Country))
		if value != "" {
			locations = append(locations, value)
		}
	}
	workplace := ""
	if selected.LocationType == "TELECOMMUTE" {
		workplace = "Remote"
	}
	source, err := SealPostingSource(PostingSource{Version: PostingSourceFormatVersion, Identity: identity, DescriptionHTML: selected.Description, Location: strings.Join(locations, "; "), WorkplaceType: workplace, EmploymentType: workablePublicText(selected.Employment)}, canonical)
	if err != nil {
		return Metadata{}, err
	}
	metadata.Source = AvailablePostingSource(source)
	return completeMetadata(metadata), nil
}
