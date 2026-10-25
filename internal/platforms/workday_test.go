package platforms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestWorkdayPostingIdentity(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want PostingIdentity
	}{
		{
			name: "myworkdayjobs with locale and query",
			url:  "https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle-WA/Platform-Engineer_R-12345?locations=west",
			want: PostingIdentity{
				Provider: "workday", Board: "acme.wd5.myworkdayjobs.com/acme/External",
				Region: "en-US", PostingID: "Seattle-WA/Platform-Engineer_R-12345",
			},
		},
		{
			name: "myworkdayjobs without locale",
			url:  "https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite/job/US-CA-Santa-Clara/Developer_Tools_R-9988",
			want: PostingIdentity{
				Provider: "workday", Board: "nvidia.wd5.myworkdayjobs.com/nvidia/NVIDIAExternalCareerSite",
				PostingID: "US-CA-Santa-Clara/Developer_Tools_R-9988",
			},
		},
		{
			name: "myworkdayjobs single posting segment",
			url:  "https://abbott.wd5.myworkdayjobs.com/en-US/abbottcareers/job/Application-Specialist_31158698-1",
			want: PostingIdentity{
				Provider: "workday", Board: "abbott.wd5.myworkdayjobs.com/abbott/abbottcareers",
				Region: "en-US", PostingID: "Application-Specialist_31158698-1",
			},
		},
		{
			name: "myworkdaysite with locale",
			url:  "https://wd5.myworkdaysite.com/fr-FR/recruiting/uw/UWHires/job/Seattle/Platform-Engineer_REQ-12345",
			want: PostingIdentity{
				Provider: "workday", Board: "wd5.myworkdaysite.com/uw/UWHires",
				Region: "fr-FR", PostingID: "Seattle/Platform-Engineer_REQ-12345",
			},
		},
		{
			name: "myworkdaysite without locale",
			url:  "https://wd1.myworkdaysite.com/recruiting/uw/UWHires/job/Payroll-Operations_REQ-456",
			want: PostingIdentity{
				Provider: "workday", Board: "wd1.myworkdaysite.com/uw/UWHires",
				PostingID: "Payroll-Operations_REQ-456",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := WorkdayPostingIdentity(test.url)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("identity = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestParseWorkdayURLRejectsUnqualifiedTargets(t *testing.T) {
	urls := []string{
		"http://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform_R-1",
		"https://acme.wd5.myworkdayjobs.com:443/en-US/External/job/Seattle/Platform_R-1",
		"https://user@acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform_R-1",
		"https://acme.wd5.myworkdayjobs.com.evil.invalid/en-US/External/job/Seattle/Platform_R-1",
		"https://evil.acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform_R-1",
		"https://acme_wd.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform_R-1",
		"https://acme.wd5.myworkdayjobs.com/en-US/External/jobs",
		"https://acme.wd5.myworkdayjobs.com/en-US/External/details/Platform_R-1",
		"https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform/extra",
		"https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle%2FWest/Platform_R-1",
		"https://acme.wd5.myworkdayjobs.com/en-US/External/job/../Platform_R-1",
		"https://wd5.myworkdaysite.com/en-US/recruiting/uw/UWHires/jobs",
		"https://wd5.myworkdaysite.com/en-US/recruiting/uw/UWHires/job/Seattle/Platform_R-1/extra",
		"https://myworkdaysite.com/en-US/recruiting/uw/UWHires/job/Seattle/Platform_R-1",
		"https://acme.wd5.myworkdayjobs.com/english-us/External/job/Seattle/Platform_R-1",
	}
	for _, rawURL := range urls {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := parseWorkdayURL(rawURL); err == nil {
				t.Fatalf("accepted unsupported URL %q", rawURL)
			}
		})
	}
}

func TestExtractWorkdayMetadataPreservesStructuredSource(t *testing.T) {
	data := workdaySyntheticDetail(t)
	posting, err := parseWorkdayURL("https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle-WA/Platform-Engineer_R-12345?locations=west")
	if err != nil {
		t.Fatal(err)
	}
	got, err := extractWorkdayMetadata(data, posting)
	if err != nil {
		t.Fatal(err)
	}

	wantDescription := "Role overview\nBuild safe systems.\nWhat you will do\n• Design APIs.\n• Review code.\nQualifications\nExperience shipping Go services."
	if got.Title != "Platform Engineer" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Description != wantDescription {
		t.Errorf("canonical description bytes = %q, want %q", got.Description, wantDescription)
	}
	if got.Company != "Fixture Systems" || got.SalaryText != "" {
		t.Errorf("provider inferred an unsupported fact: company=%q salary=%q", got.Company, got.SalaryText)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("source unavailable: %+v", got.Source)
	}
	source := got.Source.Source
	if source.Identity != posting.identity() {
		t.Errorf("source identity = %+v, want %+v", source.Identity, posting.identity())
	}
	if source.DescriptionHTML != workdaySyntheticHTML {
		t.Errorf("rich HTML was changed: %q", source.DescriptionHTML)
	}
	if source.DescriptionPlain != "" {
		t.Errorf("invented plain source alternative: %q", source.DescriptionPlain)
	}
	if source.Location != "Seattle, WA; Portland, OR" || source.WorkplaceType != "Hybrid" || source.EmploymentType != "Full time" {
		t.Errorf("direct facts = location %q, workplace %q, employment %q", source.Location, source.WorkplaceType, source.EmploymentType)
	}
	if source.SalarySummary != "" {
		t.Errorf("invented salary: %q", source.SalarySummary)
	}
	if source.CanonicalDescriptionSHA256 != CanonicalDescriptionDigest(wantDescription) {
		t.Errorf("canonical digest = %q", source.CanonicalDescriptionSHA256)
	}
}

const workdaySyntheticHTML = "<section><h2>Role overview</h2><p>Build safe systems.</p></section><section><h2>What you will do</h2><ul><li>Design APIs.</li><li>Review code.</li></ul></section><section><h2>Qualifications</h2><p>Experience shipping Go services.</p></section>"

func TestFetchWorkdayUsesExactDetailGETForBothHosts(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantHost     string
		wantEndpoint string
	}{
		{
			name:         "myworkdayjobs",
			url:          "https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle-WA/Platform-Engineer_R-12345?locations=west",
			wantHost:     "acme.wd5.myworkdayjobs.com",
			wantEndpoint: "/wday/cxs/acme/External/job/Seattle-WA/Platform-Engineer_R-12345",
		},
		{
			name:         "myworkdaysite",
			url:          "https://wd5.myworkdaysite.com/en-US/recruiting/uw/UWHires/job/Seattle/Platform-Engineer_R-12345",
			wantHost:     "wd5.myworkdaysite.com",
			wantEndpoint: "/wday/cxs/uw/UWHires/job/Seattle/Platform-Engineer_R-12345",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			client := &http.Client{Transport: workdayRoundTripper(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", req.Method)
				}
				if req.URL.Scheme != "https" || req.URL.Host != test.wantHost || req.URL.Path != test.wantEndpoint {
					t.Errorf("request target = %s, want https://%s%s", req.URL, test.wantHost, test.wantEndpoint)
				}
				if req.URL.RawQuery != "" || req.Header.Get("Accept") != "application/json" {
					t.Errorf("request carried unvalidated query or omitted JSON Accept: query=%q accept=%q", req.URL.RawQuery, req.Header.Get("Accept"))
				}
				return workdayHTTPResponse(req, http.StatusOK, workdaySyntheticDetail(t), nil), nil
			})}
			got, err := fetchWorkdayWithClient(context.Background(), test.url, client)
			if err != nil {
				t.Fatal(err)
			}
			if requests != 1 {
				t.Fatalf("request count = %d, want exactly one detail GET", requests)
			}
			if got.Title != "Platform Engineer" || got.Source.Availability != PostingSourceAvailable {
				t.Fatalf("metadata = %+v", got)
			}
		})
	}
}

func TestWorkdayReturnedPostingIdentityMustMatchURL(t *testing.T) {
	posting, err := parseWorkdayURL("https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform-Engineer_R-12345")
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Replace(workdaySyntheticDetail(t), []byte("Platform-Engineer_R-12345"), []byte("Different-Posting_R-12345"), 1)
	if _, err := extractWorkdayMetadata(data, posting); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("mismatched posting identity error = %v", err)
	}

	missingID := []byte(`{"jobPostingInfo":{"title":"Platform Engineer","jobDescription":"<p>Details</p>"}}`)
	if _, err := extractWorkdayMetadata(missingID, posting); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("missing posting identity error = %v", err)
	}
}

func TestWorkdayMalformedOptionalFactsDoNotDiscardPosting(t *testing.T) {
	posting, err := parseWorkdayURL("https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform-Engineer_R-12345")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"jobPostingInfo":{"title":"Platform Engineer","jobPostingId":"Platform-Engineer_R-12345","jobDescription":"<p>Build safe systems.</p>","location":{"descriptor":"not-a-direct-string"},"additionalLocations":{"unexpected":true},"remoteType":false,"timeType":3}}`)
	got, err := extractWorkdayMetadata(data, posting)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Platform Engineer" || got.Description != "Build safe systems." || got.Source.Availability != PostingSourceAvailable {
		t.Fatalf("valid canonical posting was discarded: %+v", got)
	}
	if got.Source.Source.Location != "" || got.Source.Source.WorkplaceType != "" || got.Source.Source.EmploymentType != "" {
		t.Fatalf("malformed optional facts were retained: %+v", got.Source.Source)
	}
}

func TestWorkdaySourceLimitsAndAbsentContent(t *testing.T) {
	posting, err := parseWorkdayURL("https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform-Engineer_R-12345")
	if err != nil {
		t.Fatal(err)
	}
	tooLarge := strings.Repeat("x", MaxPostingSourceInputBytes+1)
	data, err := json.Marshal(workdayDetailResponse{
		HiringOrganization: json.RawMessage(`{"name":"Fixture Systems"}`),
		JobPostingInfo: &workdayPostingInfo{
			Title: workdayOptionalString("Platform Engineer"), JobPostingID: workdayOptionalString("Platform-Engineer_R-12345"),
			JobDescription: workdayOptionalString(tooLarge),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := extractWorkdayMetadata(data, posting)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Platform Engineer" || got.Company != "Fixture Systems" || got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceTooLarge || got.Description != "" {
		t.Fatalf("oversized source handling: title=%q company=%q desc bytes=%d source=%+v", got.Title, got.Company, len(got.Description), got.Source)
	}

	empty, err := extractWorkdayMetadata([]byte(`{"jobPostingInfo":{"title":"Platform Engineer","jobPostingId":"Platform-Engineer_R-12345"}}`), posting)
	if err != nil || empty.Source.Availability != PostingSourceUnavailable || empty.Source.Reason != PostingSourceAbsent {
		t.Fatalf("empty source handling: metadata=%+v err=%v", empty, err)
	}
}

func TestWorkdayResponseErrorsAndBounds(t *testing.T) {
	rawURL := "https://acme.wd5.myworkdayjobs.com/en-US/External/job/Seattle/Platform-Engineer_R-12345"
	for _, status := range []int{http.StatusNotFound, http.StatusGone, http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := &http.Client{Transport: workdayRoundTripper(func(req *http.Request) (*http.Response, error) {
				return workdayHTTPResponse(req, status, []byte("blocked or unavailable"), nil), nil
			})}
			_, err := fetchWorkdayWithClient(context.Background(), rawURL, client)
			if err == nil {
				t.Fatal("expected status error")
			}
			closed := errors.Is(err, ErrClosed)
			if closed != (status == http.StatusNotFound || status == http.StatusGone) {
				t.Fatalf("ErrClosed = %t for HTTP %d: %v", closed, status, err)
			}
		})
	}

	t.Run("oversized response", func(t *testing.T) {
		body := bytes.Repeat([]byte("x"), maxPostingResponse+1)
		client := &http.Client{Transport: workdayRoundTripper(func(req *http.Request) (*http.Response, error) {
			return workdayHTTPResponse(req, http.StatusOK, body, nil), nil
		})}
		_, err := fetchWorkdayWithClient(context.Background(), rawURL, client)
		if err == nil || !strings.Contains(err.Error(), "exceeds 4 MiB") {
			t.Fatalf("oversized response error = %v", err)
		}
	})

	t.Run("redirect refused", func(t *testing.T) {
		requests := 0
		client := postingClient()
		client.Transport = workdayRoundTripper(func(req *http.Request) (*http.Response, error) {
			requests++
			return workdayHTTPResponse(req, http.StatusFound, nil, http.Header{"Location": []string{"https://attacker.invalid/apply"}}), nil
		})
		_, err := fetchWorkdayWithClient(context.Background(), rawURL, client)
		if err == nil || requests != 1 {
			t.Fatalf("redirect result err=%v requests=%d", err, requests)
		}
	})
}

func workdaySyntheticDetail(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/job-intelligence/workday-detail-synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type workdayRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip workdayRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return roundTrip(req)
}

func workdayHTTPResponse(req *http.Request, status int, body []byte, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
}
