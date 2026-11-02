package platforms

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func TeamtailorPostingIdentity(raw string) (PostingIdentity, error) {
	u, err := vendorPostingURL(raw, ".teamtailor.com")
	if err != nil {
		return PostingIdentity{}, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "jobs" {
		return PostingIdentity{}, fmt.Errorf("unsupported Teamtailor posting path")
	}
	id, _, _ := strings.Cut(parts[1], "-")
	if !decimalPostingID.MatchString(id) {
		return PostingIdentity{}, fmt.Errorf("invalid Teamtailor posting identity")
	}
	return PostingIdentity{Provider: "teamtailor", Board: u.Host, PostingID: id}, nil
}

type teamtailorItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Remote      string `xml:"remoteStatus"`
	Locations   []struct {
		City    string `xml:"city"`
		Country string `xml:"country"`
	} `xml:"locations>location"`
}

func fetchTeamtailor(ctx context.Context, client *http.Client, raw string) (Metadata, error) {
	identity, err := TeamtailorPostingIdentity(raw)
	if err != nil {
		return Metadata{}, err
	}
	// Bound pagination to two documented 100-item pages (8 MiB aggregate). Absence after that is
	// unavailable, never proof of closure. Exact URL identity selects each item.
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for page := 0; page < 2; page++ {
		endpoint := fmt.Sprintf("https://%s/jobs.rss?offset=%d&per_page=100", identity.Board, page*100)
		data, err := readPublicJobFeed(ctx, client, endpoint)
		if err != nil {
			return Metadata{}, err
		}
		result, count, found, err := extractTeamtailorMetadata(data, identity)
		if err != nil {
			return Metadata{}, err
		}
		if found {
			return result, nil
		}
		if count < 100 {
			break
		}
	}
	return Metadata{}, fmt.Errorf("Teamtailor posting absent from bounded public feed")
}
func extractTeamtailorMetadata(data []byte, identity PostingIdentity) (Metadata, int, bool, error) {
	if len(data) > maxPostingResponse {
		return Metadata{}, 0, false, fmt.Errorf("Teamtailor feed exceeds limit")
	}
	var feed struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Title string           `xml:"title"`
			Items []teamtailorItem `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(data, &feed); err != nil {
		return Metadata{}, 0, false, err
	}
	var selected *teamtailorItem
	for i := range feed.Channel.Items {
		item := &feed.Channel.Items[i]
		got, err := TeamtailorPostingIdentity(item.Link)
		if err == nil && got == identity {
			if selected != nil {
				return Metadata{}, len(feed.Channel.Items), false, fmt.Errorf("ambiguous Teamtailor posting identity")
			}
			selected = item
		}
	}
	count := len(feed.Channel.Items)
	if selected == nil {
		return Metadata{}, count, false, nil
	}
	canonical := descriptionText(selected.Description)
	if strings.TrimSpace(selected.Title) == "" || canonical == "" {
		return Metadata{}, count, false, fmt.Errorf("Teamtailor posting lacks readable title or description")
	}
	locations := []string{}
	for _, location := range selected.Locations {
		value := workableJoinLocation(location.City, location.Country)
		if value != "" {
			locations = append(locations, value)
		}
	}
	workplace := ""
	switch selected.Remote {
	case "fully":
		workplace = "Remote"
	case "hybrid":
		workplace = "Hybrid"
	case "none":
		workplace = "OnSite"
	}
	metadata := Metadata{Title: workablePublicText(selected.Title), Company: workablePublicText(feed.Channel.Title), Description: canonical}
	if len(metadata.Title) > 200 || len(metadata.Company) > 200 {
		return Metadata{}, count, false, fmt.Errorf("public posting title or employer exceeds 200 bytes")
	}
	source, err := SealPostingSource(PostingSource{Version: PostingSourceFormatVersion, Identity: identity, DescriptionHTML: selected.Description, Location: strings.Join(locations, "; "), WorkplaceType: workplace}, canonical)
	if err != nil {
		return Metadata{}, count, false, err
	}
	metadata.Source = AvailablePostingSource(source)
	return completeMetadata(metadata), count, true, nil
}
