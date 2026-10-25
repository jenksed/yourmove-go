package platforms

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	smartRecruitersTestCompany = "sampleco"
	smartRecruitersTestID      = "744000123456789"
)

type smartRecruitersTestRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip smartRecruitersTestRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func smartRecruitersFixtureBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "job-intelligence", "smartrecruiters-posting-synthetic.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func smartRecruitersFixturePosting(t *testing.T) smartRecruitersPosting {
	t.Helper()
	var posting smartRecruitersPosting
	if err := json.Unmarshal(smartRecruitersFixtureBytes(t), &posting); err != nil {
		t.Fatal(err)
	}
	return posting
}

func smartRecruitersMarshalPosting(t *testing.T, posting smartRecruitersPosting) []byte {
	t.Helper()
	data, err := json.Marshal(posting)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParseSmartRecruitersURL(t *testing.T) {
	tests := []struct {
		name      string
		rawURL    string
		company   string
		postingID string
		wantError bool
	}{
		{
			name:      "numeric posting ID",
			rawURL:    "https://jobs.smartrecruiters.com/SampleCo/744000123456789",
			company:   "SampleCo",
			postingID: "744000123456789",
		},
		{
			name:      "numeric ID with title suffix and tracking query",
			rawURL:    "https://jobs.smartrecruiters.com/sampleco/744000123456789-senior-api-engineer?trid=tracking#job-description",
			company:   "sampleco",
			postingID: "744000123456789",
		},
		{name: "HTTP rejected", rawURL: "http://jobs.smartrecruiters.com/sampleco/123", wantError: true},
		{name: "lookalike host rejected", rawURL: "https://jobs.smartrecruiters.com.evil.test/sampleco/123", wantError: true},
		{name: "explicit port rejected", rawURL: "https://jobs.smartrecruiters.com:443/sampleco/123", wantError: true},
		{name: "userinfo rejected", rawURL: "https://user@jobs.smartrecruiters.com/sampleco/123", wantError: true},
		{name: "encoded path rejected", rawURL: "https://jobs.smartrecruiters.com/sampleco/123%2Fother", wantError: true},
		{name: "extra path rejected", rawURL: "https://jobs.smartrecruiters.com/sampleco/123/job", wantError: true},
		{name: "trailing slash rejected", rawURL: "https://jobs.smartrecruiters.com/sampleco/123/", wantError: true},
		{name: "zero ID rejected", rawURL: "https://jobs.smartrecruiters.com/sampleco/0", wantError: true},
		{name: "non-numeric ID rejected", rawURL: "https://jobs.smartrecruiters.com/sampleco/job-123", wantError: true},
		{name: "UUID URL form unsupported", rawURL: "https://jobs.smartrecruiters.com/sampleco/d8bb6f62-8934-4fed-a532-8d8c968b7c12", wantError: true},
		{name: "empty suffix rejected", rawURL: "https://jobs.smartrecruiters.com/sampleco/123-", wantError: true},
		{name: "one-click application route rejected", rawURL: "https://jobs.smartrecruiters.com/oneclick-ui/company/sampleco/publication/d8bb6f62-8934-4fed-a532-8d8c968b7c12", wantError: true},
		{name: "career page rejected", rawURL: "https://careers.smartrecruiters.com/sampleco", wantError: true},
		{name: "legacy host rejected", rawURL: "https://www.smartrecruiters.com/sampleco/123-job", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			company, postingID, err := parseSmartRecruitersURL(test.rawURL)
			if test.wantError {
				if err == nil {
					t.Fatalf("parseSmartRecruitersURL(%q) unexpectedly succeeded", test.rawURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSmartRecruitersURL(%q): %v", test.rawURL, err)
			}
			if company != test.company || postingID != test.postingID {
				t.Fatalf("parseSmartRecruitersURL(%q) = %q/%q, want %q/%q", test.rawURL, company, postingID, test.company, test.postingID)
			}
		})
	}
}

func TestExtractSmartRecruitersMetadataPreservesSectionsAndDirectFacts(t *testing.T) {
	got, err := extractSmartRecruitersMetadata(smartRecruitersFixtureBytes(t), smartRecruitersTestCompany, smartRecruitersTestID)
	if err != nil {
		t.Fatal(err)
	}
	const wantDescription = "Company Description\n" +
		"We build tools & services.\n" +
		"Job Description\n" +
		"Own the API.\n" +
		"• Ship safely\n" +
		"• Keep latency low\n" +
		"Qualifications\n" +
		"Three years of Go.\n" +
		"Additional Information\n" +
		"Remote within Canada."
	if got.Title != "Senior API Engineer" || got.Company != "Sample Company" {
		t.Fatalf("title/company = %q/%q", got.Title, got.Company)
	}
	if got.Description != wantDescription {
		t.Fatalf("canonical description bytes differ:\n got: %q\nwant: %q", got.Description, wantDescription)
	}
	if got.SalaryText != "CAD 100000–125000 / yearly" || got.SalaryOrigin != "employer" {
		t.Fatalf("salary = %q (%q)", got.SalaryText, got.SalaryOrigin)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("source availability = %+v", got.Source)
	}
	source := got.Source.Source
	if source.Identity != (PostingIdentity{Provider: "smartrecruiters", Board: smartRecruitersTestCompany, PostingID: smartRecruitersTestID}) {
		t.Fatalf("source identity = %+v", source.Identity)
	}
	if source.DescriptionPlain != wantDescription || source.CanonicalDescriptionSHA256 != CanonicalDescriptionDigest(wantDescription) {
		t.Fatalf("source plain/canonical association changed: plain=%q digest=%q", source.DescriptionPlain, source.CanonicalDescriptionSHA256)
	}
	if source.Location != "Montreal, QC, Canada" || source.WorkplaceType != "Remote" || source.EmploymentType != "Full-time" || source.SalarySummary != got.SalaryText {
		t.Fatalf("direct source facts = %+v", source)
	}
	positions := []int{
		strings.Index(source.DescriptionHTML, "Company Description"),
		strings.Index(source.DescriptionHTML, "Job Description"),
		strings.Index(source.DescriptionHTML, "Qualifications"),
		strings.Index(source.DescriptionHTML, "Additional Information"),
	}
	for i, position := range positions {
		if position < 0 || i > 0 && position <= positions[i-1] {
			t.Fatalf("job-ad section order = %v", positions)
		}
	}
	if strings.Contains(source.DescriptionHTML, "video.example.invalid") || strings.Contains(source.DescriptionHTML, "oga=true") {
		t.Fatal("video or application URL leaked into the description source")
	}
	digest, err := source.ContentDigest()
	if err != nil || digest != source.SourceContentSHA256 {
		t.Fatalf("source digest = %q, %v; sealed %q", digest, err, source.SourceContentSHA256)
	}
}

func TestExtractSmartRecruitersMetadataRejectsMismatchedAndUnpublishedResponses(t *testing.T) {
	base := smartRecruitersFixturePosting(t)
	active := false
	tests := []struct {
		name       string
		mutate     func(*smartRecruitersPosting)
		wantClosed bool
	}{
		{name: "company mismatch", mutate: func(p *smartRecruitersPosting) { p.Company.Identifier = "otherco" }},
		{name: "posting ID mismatch", mutate: func(p *smartRecruitersPosting) { p.ID = "744000000000001" }},
		{name: "posting URL mismatch", mutate: func(p *smartRecruitersPosting) {
			p.PostingURL = "https://jobs.smartrecruiters.com/otherco/744000123456789"
		}},
		{name: "malformed response UUID", mutate: func(p *smartRecruitersPosting) { p.UUID = "not-a-uuid" }},
		{name: "inactive posting", mutate: func(p *smartRecruitersPosting) { p.Active = &active }, wantClosed: true},
		{name: "missing active state", mutate: func(p *smartRecruitersPosting) { p.Active = nil }},
		{name: "internal posting", mutate: func(p *smartRecruitersPosting) { p.Visibility = "INTERNAL" }},
		{name: "missing public visibility", mutate: func(p *smartRecruitersPosting) { p.Visibility = "" }},
		{name: "missing company name", mutate: func(p *smartRecruitersPosting) { p.Company.Name = " " }},
		{name: "missing title", mutate: func(p *smartRecruitersPosting) { p.Name = " " }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			posting := base
			test.mutate(&posting)
			_, err := extractSmartRecruitersMetadata(smartRecruitersMarshalPosting(t, posting), smartRecruitersTestCompany, smartRecruitersTestID)
			if err == nil {
				t.Fatal("invalid or unpublished posting unexpectedly succeeded")
			}
			if errors.Is(err, ErrClosed) != test.wantClosed {
				t.Fatalf("closed disposition = %v for error %v", errors.Is(err, ErrClosed), err)
			}
		})
	}
}

func TestSmartRecruitersEmptySectionsAndWorkplaceFactsAreConservative(t *testing.T) {
	posting := smartRecruitersFixturePosting(t)
	posting.JobAd.Sections.Qualifications.Text = ""
	falseValue := false
	posting.Location.Remote = &falseValue
	posting.Location.Hybrid = &falseValue
	got, err := extractSmartRecruitersMetadata(smartRecruitersMarshalPosting(t, posting), smartRecruitersTestCompany, smartRecruitersTestID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Description, "Qualifications") || strings.Contains(got.Source.Source.DescriptionHTML, "Qualifications") {
		t.Fatal("empty qualifications section was rendered")
	}
	if got.Source.Source.WorkplaceType != "" {
		t.Fatalf("false remote/hybrid flags implied a workplace type %q", got.Source.Source.WorkplaceType)
	}

	trueValue := true
	posting.Location.Remote = &trueValue
	posting.Location.Hybrid = &trueValue
	got, err = extractSmartRecruitersMetadata(smartRecruitersMarshalPosting(t, posting), smartRecruitersTestCompany, smartRecruitersTestID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Source.WorkplaceType != "" {
		t.Fatalf("contradictory remote/hybrid flags implied a workplace type %q", got.Source.Source.WorkplaceType)
	}
}

func TestSmartRecruitersSalarySummaryRequiresCompleteSaneRange(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{name: "complete yearly range", json: `{"min":75000,"max":90000,"currency":"USD","period":"YEARLY"}`, want: "USD 75000–90000 / yearly"},
		{name: "missing upper bound", json: `{"min":75000,"currency":"USD","period":"YEARLY"}`},
		{name: "reversed range", json: `{"min":90000,"max":75000,"currency":"USD","period":"YEARLY"}`},
		{name: "unsupported period", json: `{"min":75000,"max":90000,"currency":"USD","period":"PER_YEAR"}`},
		{name: "invalid currency", json: `{"min":75000,"max":90000,"currency":"US","period":"YEARLY"}`},
		{name: "malformed value", json: `{"min":"75000","max":90000,"currency":"USD","period":"YEARLY"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := smartRecruitersSalarySummary(json.RawMessage(test.json))
			if got != test.want {
				t.Fatalf("salary summary = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSmartRecruitersOversizeSourceAndJSONAreUnavailableWithoutTruncation(t *testing.T) {
	posting := smartRecruitersFixturePosting(t)
	posting.JobAd.Sections.JobDescription.Text = strings.Repeat("x", MaxPostingSourceInputBytes+1)
	got, err := extractSmartRecruitersMetadata(smartRecruitersMarshalPosting(t, posting), smartRecruitersTestCompany, smartRecruitersTestID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceTooLarge || got.Source.Source != nil || got.Description != "" {
		t.Fatalf("oversized section was truncated or accepted: metadata description bytes=%d source=%+v", len(got.Description), got.Source)
	}

	posting = smartRecruitersFixturePosting(t)
	posting.JobAd.Sections.JobDescription.Text = "<p>" + strings.Repeat(`"`, 400000) + "</p>"
	got, err = extractSmartRecruitersMetadata(smartRecruitersMarshalPosting(t, posting), smartRecruitersTestCompany, smartRecruitersTestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Description) == 0 || len(got.Description) > MaxPostingSourceInputBytes ||
		got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceTooLarge || got.Source.Source != nil {
		t.Fatalf("encoded source JSON size was not bounded: description bytes=%d source=%+v", len(got.Description), got.Source)
	}
}

func TestExtractSmartRecruitersMetadataRejectsInvalidSectionText(t *testing.T) {
	posting := smartRecruitersFixturePosting(t)
	posting.JobAd.Sections.JobDescription.Text = "<p>Readable prefix</p>\x00<script>unsafe fallback</script>"
	_, err := extractSmartRecruitersMetadata(smartRecruitersMarshalPosting(t, posting), smartRecruitersTestCompany, smartRecruitersTestID)
	if err == nil || !strings.Contains(err.Error(), "invalid text") {
		t.Fatalf("invalid section text error = %v", err)
	}
}

func TestFetchSmartRecruitersUsesAnonymousFixedDetailEndpoint(t *testing.T) {
	fixture := string(smartRecruitersFixtureBytes(t))
	var calls int
	client := postingClient()
	client.Transport = smartRecruitersTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != smartRecruitersAPIHost ||
			request.URL.Path != "/v1/companies/sampleco/postings/"+smartRecruitersTestID || request.Body != nil {
			t.Errorf("unexpected detail request: method=%s URL=%s body=%v", request.Method, request.URL, request.Body)
		}
		if request.Header.Get("Accept") != "application/json" || request.Header.Get("Authorization") != "" || request.Header.Get("X-SmartToken") != "" {
			t.Errorf("request headers include credentials or wrong accept value: %#v", request.Header)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(fixture)),
			Request:    request,
		}, nil
	})
	if client.Timeout != 10*time.Second {
		t.Fatalf("posting client timeout = %s, want 10s", client.Timeout)
	}
	if err := client.CheckRedirect(&http.Request{}, nil); err != http.ErrUseLastResponse {
		t.Fatalf("posting client redirect policy = %v, want ErrUseLastResponse", err)
	}
	got, err := fetchSmartRecruiters(context.Background(), client, smartRecruitersTestCompany, smartRecruitersTestID)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || got.Source.Availability != PostingSourceAvailable || got.Title != "Senior API Engineer" {
		t.Fatalf("fetch calls=%d metadata=%+v", calls, got)
	}
}

func TestFetchSmartRecruitersDoesNotFollowRedirects(t *testing.T) {
	var calls int
	client := postingClient()
	client.Transport = smartRecruitersTestRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		header := make(http.Header)
		header.Set("Location", "https://attacker.example.invalid/posting")
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Request:    request,
		}, nil
	})
	_, err := fetchSmartRecruiters(context.Background(), client, smartRecruitersTestCompany, smartRecruitersTestID)
	if err == nil || calls != 1 {
		t.Fatalf("redirect handling error=%v calls=%d", err, calls)
	}
}

func TestFetchSmartRecruitersDistinguishesClosedErrorsAndBoundsResponse(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := postingClient()
			client.Transport = smartRecruitersTestRoundTripper(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
			})
			_, err := fetchSmartRecruiters(context.Background(), client, smartRecruitersTestCompany, smartRecruitersTestID)
			if !errors.Is(err, ErrClosed) {
				t.Fatalf("HTTP %d error = %v, want ErrClosed", status, err)
			}
		})
	}

	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := postingClient()
			client.Transport = smartRecruitersTestRoundTripper(func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
			})
			_, err := fetchSmartRecruiters(context.Background(), client, smartRecruitersTestCompany, smartRecruitersTestID)
			if err == nil || errors.Is(err, ErrClosed) {
				t.Fatalf("HTTP %d error = %v, want transient retrieval error", status, err)
			}
		})
	}

	t.Run("response cap", func(t *testing.T) {
		client := postingClient()
		oversized := strings.Repeat("x", maxPostingResponse+1)
		client.Transport = smartRecruitersTestRoundTripper(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(oversized)), Request: request}, nil
		})
		_, err := fetchSmartRecruiters(context.Background(), client, smartRecruitersTestCompany, smartRecruitersTestID)
		if err == nil || !strings.Contains(err.Error(), "exceeds 4 MiB") {
			t.Fatalf("oversized response error = %v", err)
		}
	})
}
