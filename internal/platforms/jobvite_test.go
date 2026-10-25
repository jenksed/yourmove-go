package platforms

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type jobviteRoundTripper func(*http.Request) (*http.Response, error)

func (fn jobviteRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func jobviteFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/job-intelligence/jobvite-posting-synthetic.html")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func useJobviteTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func TestParseJobviteURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		site string
		id   string
		bad  bool
	}{
		{name: "hosted detail with tracking query", url: "https://jobs.jobvite.com/syntheticco/job/jobvite001?nl=1", site: "syntheticco", id: "jobvite001"},
		{name: "trailing slash", url: "https://jobs.jobvite.com/syntheticco/job/jobvite001/", bad: true},
		{name: "insecure scheme", url: "http://jobs.jobvite.com/syntheticco/job/jobvite001", bad: true},
		{name: "host suffix attack", url: "https://jobs.jobvite.com.evil.test/syntheticco/job/jobvite001", bad: true},
		{name: "custom host", url: "https://careers.example.test/syntheticco/job/jobvite001", bad: true},
		{name: "userinfo", url: "https://user@jobs.jobvite.com/syntheticco/job/jobvite001", bad: true},
		{name: "custom port", url: "https://jobs.jobvite.com:8443/syntheticco/job/jobvite001", bad: true},
		{name: "application route", url: "https://jobs.jobvite.com/syntheticco/job/jobvite001/apply", bad: true},
		{name: "encoded path", url: "https://jobs.jobvite.com/syntheticco/job/jobvite%2f001", bad: true},
		{name: "missing job ID", url: "https://jobs.jobvite.com/syntheticco/job", bad: true},
		{name: "unsafe site token", url: "https://jobs.jobvite.com/synthetic%2fco/job/jobvite001", bad: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			site, id, err := parseJobviteURL(tc.url)
			if tc.bad {
				if err == nil {
					t.Fatalf("accepted unsupported URL %q as %q/%q", tc.url, site, id)
				}
				return
			}
			if err != nil || site != tc.site || id != tc.id {
				t.Fatalf("parseJobviteURL(%q) = %q/%q, %v", tc.url, site, id, err)
			}
		})
	}
}

func TestExtractJobviteMetadataPreservesSourceAndCanonicalProjection(t *testing.T) {
	data := jobviteFixture(t)
	got, err := extractJobviteMetadata(data, "syntheticco", "jobvite001")
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical := "Role summary\nBuild the example platform with care.\nResponsibilities\n• Design reliable services.\n• Review changes.\nQualifications\nExperience with Go."
	if got.Title != "Synthetic Engineering Lead" || got.Company != "Example Company" || got.Description != wantCanonical {
		t.Fatalf("canonical metadata = %+v; description = %q", got, got.Description)
	}
	if got.SalaryText != "" || got.SalaryOrigin != "" {
		t.Fatalf("quoted, unvalidated salary values were accepted: %+v", got)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("Jobvite source unavailable: %+v", got.Source)
	}
	source := *got.Source.Source
	if source.Identity != (PostingIdentity{Provider: "jobvite", Board: "syntheticco", PostingID: "jobvite001"}) {
		t.Fatalf("Jobvite identity = %+v", source.Identity)
	}
	if !strings.Contains(source.DescriptionHTML, "<h2>Role summary</h2>") || source.DescriptionPlain != "" {
		t.Fatalf("rich source alternatives changed: %+v", source)
	}
	if source.Location != "Denver, Colorado, United States; Boston, Massachusetts, United States" ||
		source.WorkplaceType != "remote" || source.EmploymentType != "FULL_TIME" || source.SalarySummary != "" {
		t.Fatalf("Jobvite direct facts = %+v", source)
	}
	if source.CanonicalDescriptionSHA256 != CanonicalDescriptionDigest(got.Description) {
		t.Fatalf("canonical source digest = %q", source.CanonicalDescriptionSHA256)
	}
	contentDigest, err := source.ContentDigest()
	if err != nil || contentDigest != source.SourceContentSHA256 {
		t.Fatalf("source content digest = %q, %v", contentDigest, err)
	}
}

func TestJobviteOnlyAcceptsOneIdentityMatchedPostingRecord(t *testing.T) {
	base := string(jobviteFixture(t))
	tests := []struct {
		name string
		body string
	}{
		{name: "wrong identifier", body: strings.Replace(base, `"identifier": "jobvite001"`, `"identifier": "otherpost1"`, 1)},
		{name: "missing identifier", body: strings.Replace(base, `"identifier": "jobvite001",`+"\n", "", 1)},
		{name: "mismatched optional URL", body: strings.Replace(base, "/job/jobvite001?source=fixture", "/job/otherpost1?source=fixture", 1)},
		{name: "off-host optional URL", body: strings.Replace(base, "https://jobs.jobvite.com/syntheticco/job/jobvite001?source=fixture", "https://evil.example/syntheticco/job/jobvite001", 1)},
		{name: "missing title", body: strings.Replace(base, `"title": "Synthetic Engineering Lead",`+"\n", "", 1)},
		{name: "second posting record", body: strings.Replace(base, "</head>", `<script type="application/ld+json">{"@type":"JobPosting","identifier":"otherpost1","title":"Other role"}</script></head>`, 1)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extractJobviteMetadata([]byte(tc.body), "syntheticco", "jobvite001"); err == nil {
				t.Fatal("accepted missing, mismatched, or ambiguous posting evidence")
			}
		})
	}
}

func TestJobviteUsesExistingValidatedSalaryMapper(t *testing.T) {
	data := string(jobviteFixture(t))
	data = strings.Replace(data, `"minValue": "110000"`, `"minValue": 110000`, 1)
	data = strings.Replace(data, `"maxValue": "130000"`, `"maxValue": 130000`, 1)
	got, err := extractJobviteMetadata([]byte(data), "syntheticco", "jobvite001")
	if err != nil {
		t.Fatal(err)
	}
	if got.SalaryText != "USD 110000–130000 / YEAR" || got.SalaryOrigin != "employer" || got.Source.Source == nil || got.Source.Source.SalarySummary != got.SalaryText {
		t.Fatalf("valid structured salary was not mapped by the existing validator: %+v", got)
	}
}

func TestFetchJobviteUsesBoundedAnonymousDetailGETWithoutRedirects(t *testing.T) {
	fixture := jobviteFixture(t)
	requests := 0
	useJobviteTransport(t, jobviteRoundTripper(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "jobs.jobvite.com" ||
			request.URL.Path != "/syntheticco/job/jobvite001" || request.URL.RawQuery != "" ||
			request.Header.Get("Accept") != "text/html, application/xhtml+xml" {
			t.Errorf("unexpected Jobvite request: %s %s Accept=%q", request.Method, request.URL, request.Header.Get("Accept"))
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(fixture))), Header: make(http.Header), Request: request}, nil
	}))
	got, err := fetchJobvite(context.Background(), "syntheticco", "jobvite001")
	if err != nil || got.Title != "Synthetic Engineering Lead" || requests != 1 {
		t.Fatalf("fetch Jobvite = %+v, %v; requests=%d", got, err, requests)
	}
}

func TestFetchJobviteDoesNotFollowRedirectsOrTreatAccessErrorsAsClosure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		closed bool
	}{
		{name: "redirect", status: http.StatusFound},
		{name: "not found", status: http.StatusNotFound, closed: true},
		{name: "gone", status: http.StatusGone, closed: true},
		{name: "forbidden", status: http.StatusForbidden},
		{name: "rate limited", status: http.StatusTooManyRequests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			useJobviteTransport(t, jobviteRoundTripper(func(request *http.Request) (*http.Response, error) {
				requests++
				header := make(http.Header)
				if tc.status == http.StatusFound {
					header.Set("Location", "https://evil.example/other-job")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader("")), Header: header, Request: request}, nil
			}))
			_, err := fetchJobvite(context.Background(), "syntheticco", "jobvite001")
			if err == nil || errors.Is(err, ErrClosed) != tc.closed || requests != 1 {
				t.Fatalf("fetch status %d: err=%v closed=%v requests=%d", tc.status, err, errors.Is(err, ErrClosed), requests)
			}
		})
	}
}

func TestFetchJobviteRejectsOversizedHTML(t *testing.T) {
	useJobviteTransport(t, jobviteRoundTripper(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", jobviteMaxHTMLBytes+1))), Header: make(http.Header), Request: request}, nil
	}))
	if _, err := fetchJobvite(context.Background(), "syntheticco", "jobvite001"); err == nil || !strings.Contains(err.Error(), "exceeds 1 MiB") {
		t.Fatalf("oversized page result = %v", err)
	}
}

func TestJobviteRejectsInvalidOrUnboundedHTMLSource(t *testing.T) {
	if _, err := jobvitePostingFromHTML([]byte{0xff}); err == nil {
		t.Fatal("invalid UTF-8 HTML was accepted")
	}
	if _, err := jobvitePostingFromHTML([]byte(strings.Repeat("x", jobviteMaxHTMLBytes+1))); err == nil {
		t.Fatal("oversized HTML was accepted")
	}
}
