package platforms

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/jobdocument"
)

func TestParseLeverURL(t *testing.T) {
	tests := []struct {
		url, site, id, apiHost string
		wantErr                bool
	}{
		{"https://jobs.lever.co/example/abc-123", "example", "abc-123", "api.lever.co", false},
		{"https://jobs.eu.lever.co/lever/f00a01?lever-source=site", "lever", "f00a01", "api.eu.lever.co", false},
		{"https://jobs.lever.co/example", "", "", "", true},
		{"https://jobs.lever.co.example.test/example/abc", "", "", "", true},
		{"https://jobs.lever.co/example/../admin", "", "", "", true},
		{"https://user@jobs.lever.co/example/abc", "", "", "", true},
		{"https://jobs.lever.co:443/example/abc", "", "", "", true},
		{"https://jobs.lever.co.evil.test/example/abc", "", "", "", true},
		{"https://jobs.eu.lever.co/lever/abc/apply", "", "", "", true},
		{"http://jobs.lever.co/example/abc", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			site, id, host, err := parseLeverURL(tt.url)
			if (err != nil) != tt.wantErr || (err == nil && (site != tt.site || id != tt.id || host != tt.apiHost)) {
				t.Fatalf("got (%q, %q, %q, %v), expected (%q, %q, %q, err=%v)", site, id, host, err, tt.site, tt.id, tt.apiHost, tt.wantErr)
			}
		})
	}
}

func TestFetchLever(t *testing.T) {
	client := testPostingClient(t, "api.eu.lever.co", "/v0/postings/lever/abc-123", `{
		"id":"abc-123",
		"text": "Implementation Engineer",
		"descriptionPlain": "Work with customers.",
		"lists": [{"text": "Responsibilities", "content": "<li>Configure systems</li><li>Debug issues</li>"}],
		"additionalPlain": "Equal opportunity employer.",
        "salaryRange": {"currency":"USD", "interval":"year", "min": 90000, "max": 125000}
	}`)
	got, err := fetchLever(context.Background(), client, "lever", "abc-123", "api.eu.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	want := "Work with customers.\n\nResponsibilities\n• Configure systems\n• Debug issues\n\nEqual opportunity employer."
	if got.Title != "Implementation Engineer" || got.Description != want || got.SalaryText != "USD 90000–125000 / year" {
		t.Fatalf("unexpected Lever metadata: %+v, want description %q", got, want)
	}
}

func TestLeverSalaryUnknownOnMissingOrInvalidRange(t *testing.T) {
	for _, r := range []*leverSalaryRange{
		nil, {Currency: "USD", Interval: "year", Min: 0, Max: 100000},
		{Currency: "USD", Interval: "year", Min: 90000, Max: 80000},
		{Currency: "", Interval: "year", Min: 90000, Max: 100000},
	} {
		if got := r.display(); got != "" {
			t.Errorf("invalid range shown as %q", got)
		}
	}
}

func TestLeverHTMLDescriptionFallbackAndBoardSuggestion(t *testing.T) {
	client := testPostingClient(t, "api.lever.co", "/v0/postings/zeneducate/87edc0c1-447a-47df-8cda-a921b81bb33e", `{
		"id":"87edc0c1-447a-47df-8cda-a921b81bb33e",
        "text":"Workers' Compensation Coordinator",
        "descriptionPlain":"",
        "description":"<p>Coordinate worker claims.</p><p>Handle benefits &amp; follow up.</p>",
        "additional":"<p>Equal opportunity employer.</p>"
    }`)
	got, err := fetchLever(context.Background(), client, "zeneducate", "87edc0c1-447a-47df-8cda-a921b81bb33e", "api.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != "zeneducate" || got.Description != "Coordinate worker claims.\nHandle benefits & follow up.\n\nEqual opportunity employer." {
		t.Fatalf("Lever fallback: %+v", got)
	}
}

func TestLeverOrderedPostingSourcePreservesNativeSectionsAndFacts(t *testing.T) {
	const postingID = "11111111-2222-4333-8444-555555555555"
	fixture, err := os.ReadFile(filepath.Join("testdata", "job-intelligence", "lever-ordered-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	client := testPostingClient(t, "api.eu.lever.co", "/v0/postings/lever/"+postingID, string(fixture))
	got, err := fetchLever(context.Background(), client, "lever", postingID, "api.eu.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	want := "Opening\nBuild reliable tools for teams that operate important services.\n\nRole overview\nPartner with product and operations to improve the service platform.\n\nResponsibilities\n• Design dependable service boundaries.\n• Improve observability and incident recovery.\n\nQualifications\n• Production Go experience.\n• Clear technical communication.\n\nApplication instructions\nInterview steps will be explained before they begin."
	if got.Description != want {
		t.Fatalf("Lever canonical projection drifted:\ngot:  %q\nwant: %q", got.Description, want)
	}
	if got.Title != "Senior Platform Engineer" || got.Company != "lever" || got.SalaryText != "CAD 150000–190000 / year" || got.SalaryOrigin != "employer" {
		t.Fatalf("Lever metadata = %+v", got)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("Lever rich source unavailable: %+v", got.Source)
	}
	source := got.Source.Source
	if source.Identity != (PostingIdentity{Provider: "lever", Region: "eu", Board: "lever", PostingID: postingID}) {
		t.Fatalf("source identity = %+v", source.Identity)
	}
	if source.DescriptionPlain != want || strings.Count(source.DescriptionHTML, "<h2>Opening</h2>") != 1 {
		t.Fatalf("source plain/order lost or combined description duplicated: plain=%q html=%q", source.DescriptionPlain, source.DescriptionHTML)
	}
	for _, token := range []string{"<h2>Responsibilities</h2>", "<h2>Qualifications</h2>", "<h2>Application instructions</h2>", "<li>Design dependable service boundaries.</li>", "<li>Production Go experience.</li>"} {
		if !strings.Contains(source.DescriptionHTML, token) {
			t.Errorf("source HTML missing %q: %s", token, source.DescriptionHTML)
		}
	}
	if source.Location != "Toronto, Ontario" || source.WorkplaceType != "hybrid" || source.EmploymentType != "Full-time" || source.SalarySummary != "CAD 150000–190000 / year" {
		t.Fatalf("direct source facts = %+v", source)
	}
	if err := source.Validate("https://jobs.eu.lever.co/lever/"+postingID+"?lever-source=fixture", got.Description); err != nil {
		t.Fatalf("Lever source association/digests: %v", err)
	}
	if err := source.Validate("https://jobs.lever.co/lever/"+postingID, got.Description); err == nil {
		t.Fatal("EU Lever source was accepted for a global posting URL")
	}
	doc, err := jobdocument.ParseDescription(source.DescriptionHTML, source.DescriptionPlain, "https://jobs.eu.lever.co/lever/"+postingID)
	if err != nil {
		t.Fatalf("parse retained Lever rich source: %v", err)
	}
	var headings []string
	listBlocks := 0
	for _, block := range doc.Blocks {
		switch block.Kind {
		case jobdocument.BlockHeading:
			var text strings.Builder
			for _, inline := range block.Content {
				text.WriteString(inline.Text)
			}
			headings = append(headings, text.String())
		case jobdocument.BlockList:
			listBlocks++
		}
	}
	wantHeadings := []string{"Opening", "Role overview", "Responsibilities", "Qualifications", "Application instructions"}
	if strings.Join(headings, "|") != strings.Join(wantHeadings, "|") || listBlocks != 2 {
		t.Fatalf("parsed ordered source headings=%q list blocks=%d, want headings=%q and 2 lists", headings, listBlocks, wantHeadings)
	}
}

func TestFetchJobLeverURLUsesRegionalProviderAndRetainsSource(t *testing.T) {
	const postingID = "11111111-2222-4333-8444-555555555555"
	fixture, err := os.ReadFile(filepath.Join("testdata", "job-intelligence", "lever-ordered-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	previousTransport := http.DefaultTransport
	http.DefaultTransport = testTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != "api.eu.lever.co" || r.URL.Path != "/v0/postings/lever/"+postingID || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected provider dispatch request: %s %s Accept=%q", r.Method, r.URL, r.Header.Get("Accept"))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(fixture))), Header: make(http.Header), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	got, err := FetchJob(context.Background(), "https://jobs.eu.lever.co/lever/"+postingID+"?lever-source=fixture")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil || got.Source.Source.Identity.Region != "eu" || got.Source.Source.Identity.PostingID != postingID {
		t.Fatalf("FetchJob Lever source = %+v", got.Source)
	}
}

func TestLeverComponentFallbackRetainsOrderedBodyAndClosingContent(t *testing.T) {
	const postingID = "abc-123"
	client := testPostingClient(t, "api.lever.co", "/v0/postings/acme/"+postingID, `{
		"id":"abc-123",
		"text":"Platform Engineer",
		"openingPlain":"Opening text.",
		"descriptionBody":"<h2>Body section</h2><p>Body text.</p>",
		"lists":[{"text":"Requirements","content":"<li>One requirement.</li>"}],
		"additionalPlain":"Closing instructions."
	}`)
	got, err := fetchLever(context.Background(), client, "acme", postingID, "api.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	want := "Opening text.\n\nBody section\nBody text.\n\nRequirements\n• One requirement.\n\nClosing instructions."
	if got.Description != want {
		t.Fatalf("component fallback order = %q, want %q", got.Description, want)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("component source unavailable: %+v", got.Source)
	}
	for _, token := range []string{"Opening text.", "<h2>Body section</h2>", "<h2>Requirements</h2>", "Closing instructions."} {
		if !strings.Contains(got.Source.Source.DescriptionHTML, token) {
			t.Errorf("ordered source missing %q: %s", token, got.Source.Source.DescriptionHTML)
		}
	}
}

func TestLeverBodyPlainFallbackWhenBodyHTMLIsMissing(t *testing.T) {
	client := testPostingClient(t, "api.lever.co", "/v0/postings/acme/plain-body-id", `{
		"id":"plain-body-id",
		"text":"Platform Engineer",
		"openingPlain":"Opening text.",
		"descriptionBodyPlain":"Body section\nBody text."
	}`)
	got, err := fetchLever(context.Background(), client, "acme", "plain-body-id", "api.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Opening text." {
		t.Fatalf("plain body fallback = %q", got.Description)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("plain body source unavailable: %+v", got.Source)
	}
	if got.Source.Source.DescriptionPlain != got.Description || !strings.Contains(got.Source.Source.DescriptionHTML, "Body section") || !strings.Contains(got.Source.Source.DescriptionHTML, "Body text.") {
		t.Fatalf("plain body was lost from rich source or changed canonical fallback: metadata=%+v source=%+v", got, got.Source.Source)
	}
}

func TestLeverIdentityMismatchAndEmptySourceStayDistinct(t *testing.T) {
	client := testPostingClient(t, "api.lever.co", "/v0/postings/acme/requested-id", `{"id":"other-id","text":"Engineer"}`)
	if _, err := fetchLever(context.Background(), client, "acme", "requested-id", "api.lever.co"); err == nil || strings.Contains(err.Error(), "closed") {
		t.Fatalf("mismatched posting identity was accepted or called closed: %v", err)
	}
	client = testPostingClient(t, "api.lever.co", "/v0/postings/acme/requested-id", `{"text":"Engineer"}`)
	if _, err := fetchLever(context.Background(), client, "acme", "requested-id", "api.lever.co"); err == nil {
		t.Fatal("response with missing required posting ID was accepted")
	}

	client = testPostingClient(t, "api.lever.co", "/v0/postings/acme/requested-id", `{"id":"requested-id","text":"Engineer","unknownOptionalField":{"x":1}}`)
	got, err := fetchLever(context.Background(), client, "acme", "requested-id", "api.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceAbsent || got.Description != "" {
		t.Fatalf("empty successful source = %+v, description=%q", got.Source, got.Description)
	}
}

func TestLeverOversizedRichSourceUsesCanonicalFallback(t *testing.T) {
	const postingID = "requested-id"
	plain := strings.Repeat("x", MaxPostingSourceInputBytes+1)
	client := testPostingClient(t, "api.lever.co", "/v0/postings/acme/"+postingID, `{"id":"requested-id","text":"Engineer","descriptionPlain":"`+plain+`"}`)
	got, err := fetchLever(context.Background(), client, "acme", postingID, "api.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Description) != len(plain) || got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceTooLarge {
		t.Fatalf("oversized canonical fallback: description bytes=%d source=%+v", len(got.Description), got.Source)
	}
}

func TestLeverMalformedOptionalFactsAreDroppedIndividually(t *testing.T) {
	client := testPostingClient(t, "api.lever.co", "/v0/postings/acme/malformed-facts", `{
		"id":"malformed-facts",
		"text":"Platform Engineer",
		"descriptionPlain":"Build reliable services.",
		"categories":{"location":"Tor\u0000onto","commitment":"Full-time\u0000"},
		"workplaceType":"hybrid\u0000"
	}`)
	got, err := fetchLever(context.Background(), client, "acme", "malformed-facts", "api.lever.co")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Build reliable services." {
		t.Fatalf("canonical description changed: %q", got.Description)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("malformed optional facts invalidated usable source: %+v", got.Source)
	}
	if got.Source.Source.Location != "" || got.Source.Source.EmploymentType != "" || got.Source.Source.WorkplaceType != "" {
		t.Fatalf("malformed optional facts were retained: %+v", got.Source.Source)
	}
}
