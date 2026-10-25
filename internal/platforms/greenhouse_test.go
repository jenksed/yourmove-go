package platforms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type testTransport func(*http.Request) (*http.Response, error)

func (fn testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func testPostingClient(t *testing.T, expectedHost, expectedPath, response string) *http.Client {
	t.Helper()
	return &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Host != expectedHost || r.URL.Path != expectedPath || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected API request: %s %s Accept=%q", r.Method, r.URL, r.Header.Get("Accept"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(response)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}
}

func TestParseGreenhouseURL(t *testing.T) {
	tests := []struct {
		url     string
		board   string
		id      string
		wantErr bool
	}{
		{"https://job-boards.greenhouse.io/acme/jobs/1234567", "acme", "1234567", false},
		{"https://boards.greenhouse.io/acme/jobs/789?gh_src=email", "acme", "789", false},
		{"https://job-boards.greenhouse.io/acme", "", "", true},
		{"https://job-boards.greenhouse.io/acme/jobs/nope", "", "", true},
		{"https://job-boards.greenhouse.io/acme/jobs/123/extra", "", "", true},
		{"https://job-boards.greenhouse.io/acme%2Fother/jobs/123", "", "", true},
		{"https://job-boards.greenhouse.io:443/acme/jobs/123", "", "", true},
		{"https://user@job-boards.greenhouse.io/acme/jobs/123", "", "", true},
		{"https://job-boards.greenhouse.io.evil.test/acme/jobs/123", "", "", true},
		{"http://boards.greenhouse.io/acme/jobs/123", "", "", true},
		{"https://boards.greenhouse.io@evil.test/acme/jobs/123", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			board, id, err := parseGreenhouseURL(tt.url)
			if (err != nil) != tt.wantErr || (err == nil && (board != tt.board || id != tt.id)) {
				t.Fatalf("got (%q, %q, %v), expected (%q, %q, err=%v)", board, id, err, tt.board, tt.id, tt.wantErr)
			}
		})
	}
}

func TestFetchGreenhouse(t *testing.T) {
	fixture, err := os.ReadFile("testdata/job-intelligence/greenhouse-encoded-posting-synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	client := testPostingClient(t, "boards-api.greenhouse.io", "/v1/boards/acme/jobs/1234567", string(fixture))
	got, err := fetchGreenhouse(context.Background(), client, "acme", "1234567")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Support Engineer" || got.Description != "Debug API issues\n• Read logs" {
		t.Fatalf("unexpected Greenhouse metadata: %+v", got)
	}
	if got.Company != "acme" {
		t.Fatalf("canonical company suggestion changed: %q", got.Company)
	}
	if got.SalaryText != "" || got.SalaryOrigin != "" {
		t.Fatalf("unqualified Greenhouse salary was inferred: %+v", got)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("documented content did not produce a retained source: %+v", got.Source)
	}
	source := *got.Source.Source
	if source.Identity != (PostingIdentity{Provider: "greenhouse", Board: "acme", PostingID: "1234567"}) ||
		source.DescriptionHTML != "<p>Debug API issues</p><ul><li>Read logs</li></ul>" ||
		source.DescriptionPlain != "" || source.Location != "Remote - United States" {
		t.Fatalf("retained Greenhouse source fields changed: %+v", source)
	}
	savedURL := "https://job-boards.greenhouse.io/acme/jobs/1234567?gh_src=email"
	encoded, err := EncodePostingSource(source, savedURL, got.Description)
	if err != nil {
		t.Fatalf("encode retained Greenhouse source: %v", err)
	}
	decoded, err := DecodePostingSource(encoded, savedURL, got.Description)
	if err != nil {
		t.Fatalf("decode retained Greenhouse source: %v", err)
	}
	if decoded != source {
		t.Fatalf("Greenhouse source did not round-trip: got %+v, want %+v", decoded, source)
	}
}

func TestGreenhouseSourceCandidatePreservesDecodedHTMLAndDirectLocation(t *testing.T) {
	content := `&amp;lt;h2&amp;gt;What you'll do&amp;lt;/h2&amp;gt;&amp;lt;ul&amp;gt;&amp;lt;li&amp;gt;Build carefully&amp;lt;/li&amp;gt;&amp;lt;/ul&amp;gt;`
	candidate := greenhouseSourceCandidate(content, "Remote - United States", "acme", "1234567")
	if candidate.Identity != (PostingIdentity{Provider: "greenhouse", Board: "acme", PostingID: "1234567"}) {
		t.Fatalf("candidate identity = %+v", candidate.Identity)
	}
	if candidate.DescriptionHTML != "<h2>What you'll do</h2><ul><li>Build carefully</li></ul>" {
		t.Fatalf("decoded source HTML = %q", candidate.DescriptionHTML)
	}
	if candidate.DescriptionPlain != "" || candidate.Location != "Remote - United States" || candidate.WorkplaceType != "" || candidate.EmploymentType != "" {
		t.Fatalf("candidate invented or lost provider fields: %+v", candidate)
	}
}

func TestGreenhouseCanonicalOutputMatchesLegacyDescriptionProjection(t *testing.T) {
	content := `&amp;lt;h2&amp;gt;Role&amp;lt;/h2&amp;gt;&amp;lt;p&amp;gt;Salary: €120K - €150K&amp;lt;/p&amp;gt;&amp;lt;ul&amp;gt;&amp;lt;li&amp;gt;Write carefully&amp;lt;/li&amp;gt;&amp;lt;/ul&amp;gt;`
	got, err := extractGreenhouseMetadata([]byte(`{"id":1234567,"title":" Senior Engineer ","content":"`+content+`","unknown":{"anything":true}}`), "acme", "1234567")
	if err != nil {
		t.Fatal(err)
	}
	if want := descriptionText(content); got.Description != want || got.Description != "Role\nSalary: €120K - €150K\n• Write carefully" {
		t.Fatalf("canonical description changed: got %q, want legacy projection %q", got.Description, want)
	}
	if got.Title != "Senior Engineer" || got.Company != "acme" || got.SalaryText != "Salary: €120K - €150K" || got.SalaryOrigin != "description" {
		t.Fatalf("canonical metadata projection changed: %+v", got)
	}
}

func TestGreenhouseIdentityRejectsMissingMalformedAndMismatchedIDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing", body: `{"title":"Engineer","content":"<p>Role</p>"}`},
		{name: "string ID", body: `{"id":"1234567","title":"Engineer"}`},
		{name: "zero ID", body: `{"id":0,"title":"Engineer"}`},
		{name: "mismatch", body: `{"id":7654321,"title":"Engineer"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extractGreenhouseMetadata([]byte(tc.body), "acme", "1234567"); err == nil {
				t.Fatal("unverified Greenhouse response identity was accepted")
			}
		})
	}
}

func TestGreenhouseMalformedOptionalSourceIsAbsentWithoutBreakingCanonicalMetadata(t *testing.T) {
	got, err := extractGreenhouseMetadata([]byte(`{"id":1234567,"title":"Engineer","content":17,"location":"not-an-object","unknown":"ignored"}`), "acme", "1234567")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Engineer" || got.Description != "" || got.Company != "acme" {
		t.Fatalf("malformed optional fields broke canonical metadata: %+v", got)
	}
	if got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceAbsent {
		t.Fatalf("malformed optional source should be absent: %+v", got.Source)
	}
}

func TestGreenhouseLocationVariantsAreConservative(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "documented name", json: `{"name":" Dublin, Ireland "}`, want: "Dublin, Ireland"},
		{name: "missing", json: `{}`, want: ""},
		{name: "malformed type", json: `"Dublin"`, want: ""},
		{name: "malformed value", json: `{"name":false}`, want: ""},
		{name: "oversized", json: `{"name":"` + strings.Repeat("x", 513) + `"}`, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage = []byte(tc.json)
			if got := greenhouseLocationName(raw); got != tc.want {
				t.Fatalf("greenhouseLocationName(%s) = %q, want %q", tc.json, got, tc.want)
			}
		})
	}
}

func TestDescriptionTextSkipsScripts(t *testing.T) {
	got := descriptionText("<p>First</p><script>alert('x')</script><p>Second &amp; third</p>")
	if got != "First\nSecond & third" {
		t.Fatalf("description text = %q", got)
	}
}
