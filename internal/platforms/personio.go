package platforms

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var decimalPostingID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// vendorPostingURL accepts only a single tenant label below a fixed vendor
// suffix. In particular, a saved arbitrary URL never becomes a fetch target.
func vendorPostingURL(raw string, suffixes ...string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" || u.RawPath != "" {
		return nil, fmt.Errorf("unsupported public posting URL")
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range suffixes {
		tenant := strings.TrimSuffix(host, suffix)
		if tenant != host && safeToken(tenant) && !strings.Contains(tenant, ".") && tenant != "" {
			u.Host = host
			return u, nil
		}
	}
	return nil, fmt.Errorf("unsupported public posting host")
}

func PersonioPostingIdentity(raw string) (PostingIdentity, error) {
	u, err := vendorPostingURL(raw, ".jobs.personio.de", ".jobs.personio.com")
	if err != nil {
		return PostingIdentity{}, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "job" || !decimalPostingID.MatchString(parts[1]) {
		return PostingIdentity{}, fmt.Errorf("unsupported Personio posting path")
	}
	language := u.Query().Get("language")
	if language != "" && !strings.Contains("|de|en|fr|es|nl|it|pt|", "|"+language+"|") {
		return PostingIdentity{}, fmt.Errorf("unsupported Personio language")
	}
	if len(u.Query()["language"]) > 1 {
		return PostingIdentity{}, fmt.Errorf("ambiguous Personio language")
	}
	return PostingIdentity{Provider: "personio", Board: u.Host, Region: language, PostingID: parts[1]}, nil
}

// readPublicJobFeed is transport-only: callers construct a fixed vendor route.
// It sends no cookies or credentials, rejects every redirect, and does not infer
// posting closure from the absence or failure of a whole-board feed.
func readPublicJobFeed(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	if client == nil {
		client = postingClient()
	}
	bounded := *client
	bounded.Timeout = 10 * time.Second
	bounded.Jar = nil
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml, application/rss+xml, text/xml")
	resp, err := bounded.Do(req)
	if err != nil {
		return nil, fmt.Errorf("public job feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("public job feed HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPostingResponse+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxPostingResponse {
		return nil, fmt.Errorf("public job feed exceeds 4 MiB")
	}
	return body, nil
}

type personioPosition struct {
	ID       string `xml:"id"`
	Title    string `xml:"name"`
	Company  string `xml:"subcompany"`
	Office   string `xml:"office"`
	Schedule string `xml:"schedule"`
	Sections []struct {
		Name  string `xml:"name"`
		Value string `xml:"value"`
	} `xml:"jobDescriptions>jobDescription"`
}

func fetchPersonio(ctx context.Context, client *http.Client, raw string) (Metadata, error) {
	identity, err := PersonioPostingIdentity(raw)
	if err != nil {
		return Metadata{}, err
	}
	endpoint := "https://" + identity.Board + "/xml"
	if identity.Region != "" {
		endpoint += "?language=" + identity.Region
	}
	data, err := readPublicJobFeed(ctx, client, endpoint)
	if err != nil {
		return Metadata{}, err
	}
	return extractPersonioMetadata(data, identity)
}
func extractPersonioMetadata(data []byte, identity PostingIdentity) (Metadata, error) {
	if len(data) > maxPostingResponse {
		return Metadata{}, fmt.Errorf("Personio feed exceeds limit")
	}
	var feed struct {
		XMLName   xml.Name           `xml:"workzag-jobs"`
		Positions []personioPosition `xml:"position"`
	}
	if err := xml.Unmarshal(data, &feed); err != nil {
		return Metadata{}, fmt.Errorf("decode Personio XML: %w", err)
	}
	var selected *personioPosition
	for i := range feed.Positions {
		if feed.Positions[i].ID == identity.PostingID {
			if selected != nil {
				return Metadata{}, fmt.Errorf("ambiguous Personio posting identity")
			}
			selected = &feed.Positions[i]
		}
	}
	if selected == nil {
		return Metadata{}, fmt.Errorf("Personio posting absent from public feed")
	}
	var rich strings.Builder
	for _, section := range selected.Sections {
		if strings.TrimSpace(section.Value) == "" {
			continue
		}
		if strings.TrimSpace(section.Name) != "" {
			rich.WriteString("<h2>" + html.EscapeString(section.Name) + "</h2>")
		}
		rich.WriteString(section.Value)
	}
	canonical := descriptionText(rich.String())
	if strings.TrimSpace(selected.Title) == "" || canonical == "" {
		return Metadata{}, fmt.Errorf("Personio posting lacks readable title or description")
	}
	metadata := Metadata{Title: workablePublicText(selected.Title), Company: workablePublicText(selected.Company), Description: canonical}
	if len(metadata.Title) > 200 || len(metadata.Company) > 200 {
		return Metadata{}, fmt.Errorf("public posting title or employer exceeds 200 bytes")
	}
	source, err := SealPostingSource(PostingSource{Version: PostingSourceFormatVersion, Identity: identity, DescriptionHTML: rich.String(), Location: workablePublicText(selected.Office), EmploymentType: workablePublicText(selected.Schedule)}, canonical)
	if err != nil {
		return Metadata{}, err
	}
	metadata.Source = AvailablePostingSource(source)
	return completeMetadata(metadata), nil
}
