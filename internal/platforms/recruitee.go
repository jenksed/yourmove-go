package platforms

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
)

func RecruiteePostingIdentity(raw string) (PostingIdentity, error) {
	u, err := vendorPostingURL(raw, ".recruitee.com")
	if err != nil {
		return PostingIdentity{}, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "o" || !safeToken(parts[1]) {
		return PostingIdentity{}, fmt.Errorf("unsupported Recruitee posting path")
	}
	return PostingIdentity{Provider: "recruitee", Board: u.Host, PostingID: parts[1]}, nil
}

type recruiteeOffer struct {
	ID           string `xml:"id"`
	Slug         string `xml:"slug"`
	Title        string `xml:"title"`
	Description  string `xml:"description"`
	Requirements string `xml:"requirements"`
	Company      string `xml:"company_name"`
	URL          string `xml:"careers_url"`
	City         string `xml:"city"`
	Country      string `xml:"country"`
	Remote       string `xml:"remote"`
	Hybrid       string `xml:"hybrid"`
	OnSite       string `xml:"on_site"`
	Employment   string `xml:"employment_type_code"`
}

func fetchRecruitee(ctx context.Context, client *http.Client, raw string) (Metadata, error) {
	identity, err := RecruiteePostingIdentity(raw)
	if err != nil {
		return Metadata{}, err
	}
	// The documented XML feed is explicitly exempt from the Careers API token
	// requirement. Never substitute /api/offers JSON or employer credentials.
	data, err := readPublicJobFeed(ctx, client, "https://"+identity.Board+"/api/feeds/offers.xml")
	if err != nil {
		return Metadata{}, err
	}
	return extractRecruiteeMetadata(data, identity)
}
func extractRecruiteeMetadata(data []byte, identity PostingIdentity) (Metadata, error) {
	if len(data) > maxPostingResponse {
		return Metadata{}, fmt.Errorf("Recruitee feed exceeds limit")
	}
	var feed struct {
		XMLName xml.Name         `xml:"offers"`
		Offers  []recruiteeOffer `xml:"offer"`
	}
	if err := xml.Unmarshal(data, &feed); err != nil {
		return Metadata{}, err
	}
	var selected *recruiteeOffer
	for i := range feed.Offers {
		item := &feed.Offers[i]
		if item.Slug != identity.PostingID {
			continue
		}
		got, err := RecruiteePostingIdentity(item.URL)
		if err != nil || got != identity || !decimalPostingID.MatchString(item.ID) {
			return Metadata{}, fmt.Errorf("Recruitee posting identity mismatch")
		}
		if selected != nil {
			return Metadata{}, fmt.Errorf("ambiguous Recruitee posting identity")
		}
		selected = item
	}
	if selected == nil {
		return Metadata{}, fmt.Errorf("Recruitee posting absent from public feed")
	}
	rich := selected.Description
	if strings.TrimSpace(selected.Requirements) != "" {
		rich += "<h2>Job requirements</h2>" + selected.Requirements
	}
	canonical := descriptionText(rich)
	if strings.TrimSpace(selected.Title) == "" || canonical == "" {
		return Metadata{}, fmt.Errorf("Recruitee posting lacks readable title or description")
	}
	// Multiple advertised modes must not be collapsed into a stronger claim.
	workplace := ""
	if selected.Remote == "true" && selected.Hybrid != "true" && selected.OnSite != "true" {
		workplace = "Remote"
	}
	if selected.Hybrid == "true" && selected.Remote != "true" && selected.OnSite != "true" {
		workplace = "Hybrid"
	}
	if selected.OnSite == "true" && selected.Remote != "true" && selected.Hybrid != "true" {
		workplace = "OnSite"
	}
	metadata := Metadata{Title: workablePublicText(selected.Title), Company: workablePublicText(selected.Company), Description: canonical}
	if len(metadata.Title) > 200 || len(metadata.Company) > 200 {
		return Metadata{}, fmt.Errorf("public posting title or employer exceeds 200 bytes")
	}
	source, err := SealPostingSource(PostingSource{Version: PostingSourceFormatVersion, Identity: identity, DescriptionHTML: rich, Location: workableJoinLocation(selected.City, selected.Country), WorkplaceType: workplace, EmploymentType: workablePublicText(selected.Employment)}, canonical)
	if err != nil {
		return Metadata{}, err
	}
	metadata.Source = AvailablePostingSource(source)
	return completeMetadata(metadata), nil
}
