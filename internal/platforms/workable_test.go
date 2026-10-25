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
	"time"
)

type workableRoundTripper func(*http.Request) (*http.Response, error)

func (fn workableRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func workableFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/job-intelligence/workable-widget-synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func workableTestClient(fn workableRoundTripper) *http.Client {
	return &http.Client{Transport: fn}
}

func workableTestResponse(r *http.Request, status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
		Request:    r,
	}
}

func TestParseWorkableURL(t *testing.T) {
	for _, raw := range []string{
		"https://apply.workable.com/quantic-1/j/095FF7455A",
		"https://apply.workable.com/quantic-1/j/095FF7455A/?source=career-page#overview",
	} {
		account, shortcode, err := parseWorkableURL(raw)
		if err != nil || account != "quantic-1" || shortcode != "095FF7455A" {
			t.Errorf("parseWorkableURL(%q) = %q/%q, %v", raw, account, shortcode, err)
		}
	}

	for _, raw := range []string{
		"http://apply.workable.com/quantic-1/j/095FF7455A",
		"https://apply.workable.com:443/quantic-1/j/095FF7455A",
		"https://user@apply.workable.com/quantic-1/j/095FF7455A",
		"https://careers.example.com/quantic-1/j/095FF7455A",
		"https://apply.workable.com/quantic-1",
		"https://apply.workable.com/quantic-1/j/095FF7455A/apply/",
		"https://apply.workable.com/j/095FF7455A",
		"https://apply.workable.com/quantic-1//095FF7455A",
		"https://apply.workable.com/quantic-1/j/%2Fprivate",
		"https://apply.workable.com/quantic.1/j/095FF7455A",
	} {
		if account, shortcode, err := parseWorkableURL(raw); err == nil {
			t.Errorf("parseWorkableURL(%q) unexpectedly accepted %q/%q", raw, account, shortcode)
		}
	}
}

func TestFetchWorkableSelectsExactShortcodeAndPreservesRichSource(t *testing.T) {
	fixture := workableFixture(t)
	client := workableTestClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != workablePublicPostingHost ||
			r.URL.Path != "/api/v1/widget/accounts/northstar" || r.URL.RawQuery != "details=true" {
			t.Errorf("unexpected Workable request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Errorf("unexpected Workable request headers: %#v", r.Header)
		}
		if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 10*time.Second {
			t.Errorf("request deadline = %v, present=%v; want a deadline within 10 seconds", deadline, ok)
		}
		return workableTestResponse(r, http.StatusOK, fixture), nil
	})

	got, err := fetchWorkable(context.Background(), client, "northstar", "9AB12CD345")
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical := "Help keep Northstar's platform reliable and secure.\nResponsibilities\n• Improve service health checks.\n• Review incident patterns.\nQualifications\nExperience operating production services.\nBenefits\nFlexible leave and learning time."
	if got.Title != "Senior Site Reliability Engineer" || got.Company != "Northstar Systems" || got.Description != wantCanonical {
		t.Fatalf("unexpected selected Workable metadata: %+v", got)
	}
	if got.SalaryText != "" || got.SalaryOrigin != "" {
		t.Fatalf("Workable salary was inferred: %+v", got)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("Workable source unavailable: %+v", got.Source)
	}
	source := got.Source.Source
	wantHTML := "<p>Help keep Northstar's platform reliable and secure.</p><h2>Responsibilities</h2><ul><li>Improve service health checks.</li><li>Review incident patterns.</li></ul><h2>Qualifications</h2><p>Experience operating production services.</p><h2>Benefits</h2><p>Flexible leave and learning time.</p>"
	if source.Identity != (PostingIdentity{Provider: "workable", Board: "northstar", PostingID: "9AB12CD345"}) ||
		source.DescriptionHTML != wantHTML || source.DescriptionPlain != "" ||
		source.Location != "Detroit, Michigan, United States" || source.WorkplaceType != "remote" ||
		source.EmploymentType != "Full-time" || source.SalarySummary != "" {
		t.Fatalf("unexpected retained Workable source: %+v", source)
	}
	if source.CanonicalDescriptionSHA256 != CanonicalDescriptionDigest(wantCanonical) {
		t.Fatalf("canonical digest does not match selected Workable description: %+v", source)
	}
	if digest, err := source.ContentDigest(); err != nil || digest != source.SourceContentSHA256 {
		t.Fatalf("Workable source content digest = %q, %v; stored %q", digest, err, source.SourceContentSHA256)
	}
}

func TestWorkableMissingAndDuplicateShortcodesAreErrors(t *testing.T) {
	fixture := workableFixture(t)
	if _, err := extractWorkableMetadata(fixture, "northstar", "NOTFOUND123"); err == nil || errors.Is(err, ErrClosed) {
		t.Fatalf("missing shortcode error = %v; want an unresolved lookup, not closure", err)
	}

	var account workableAccount
	if err := json.Unmarshal(fixture, &account); err != nil {
		t.Fatal(err)
	}
	account.Jobs = append(account.Jobs, account.Jobs[1])
	duplicate, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := extractWorkableMetadata(duplicate, "northstar", "9AB12CD345"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate shortcode error = %v; want ambiguous identity", err)
	}
}

func TestWorkableWorkplaceFactsAreConservative(t *testing.T) {
	remote := true
	notRemote := false
	for _, tc := range []struct {
		name string
		job  workablePosting
		want string
	}{
		{name: "explicit workplace type", job: workablePosting{WorkplaceType: "hybrid", Telecommuting: &notRemote}, want: "hybrid"},
		{name: "telecommuting true", job: workablePosting{Telecommuting: &remote}, want: "remote"},
		{name: "telecommuting false does not imply on-site", job: workablePosting{Telecommuting: &notRemote}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := workableWorkplaceType(tc.job); got != tc.want {
				t.Fatalf("workableWorkplaceType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFetchWorkableDoesNotFollowRedirects(t *testing.T) {
	calls := 0
	client := workableTestClient(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://127.0.0.1/private"}},
			Body:       io.NopCloser(strings.NewReader("redirect")),
			Request:    r,
		}, nil
	})
	_, err := fetchWorkable(context.Background(), client, "northstar", "9AB12CD345")
	if err == nil || calls != 1 {
		t.Fatalf("redirect result err=%v calls=%d; want one rejected response", err, calls)
	}
}

func TestFetchWorkableStatusAndMalformedJSON(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone, http.StatusForbidden, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := workableTestClient(func(r *http.Request) (*http.Response, error) {
				return workableTestResponse(r, status, []byte(`{"message":"unavailable"}`)), nil
			})
			_, err := fetchWorkable(context.Background(), client, "northstar", "9AB12CD345")
			if err == nil || errors.Is(err, ErrClosed) {
				t.Fatalf("HTTP %d error = %v; want retrieval failure, not posting closure", status, err)
			}
		})
	}

	client := workableTestClient(func(r *http.Request) (*http.Response, error) {
		return workableTestResponse(r, http.StatusOK, []byte(`{"name":"Northstar","jobs":[`)), nil
	})
	if _, err := fetchWorkable(context.Background(), client, "northstar", "9AB12CD345"); err == nil {
		t.Fatal("malformed Workable JSON was accepted")
	}
}

func TestFetchWorkableResponseLimit(t *testing.T) {
	tooLarge := bytes.Repeat([]byte("x"), maxPostingResponse+1)
	client := workableTestClient(func(r *http.Request) (*http.Response, error) {
		return workableTestResponse(r, http.StatusOK, tooLarge), nil
	})
	if _, err := fetchWorkable(context.Background(), client, "northstar", "9AB12CD345"); err == nil || !strings.Contains(err.Error(), "4 MiB") {
		t.Fatalf("oversized response error = %v", err)
	}
}

func TestWorkableSourceLimitSkipsCanonicalParsing(t *testing.T) {
	var account workableAccount
	if err := json.Unmarshal(workableFixture(t), &account); err != nil {
		t.Fatal(err)
	}
	account.Jobs[1].Description = strings.Repeat("x", MaxPostingSourceInputBytes+1)
	data, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	got, err := extractWorkableMetadata(data, "northstar", "9AB12CD345")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Senior Site Reliability Engineer" || got.Company != "Northstar Systems" {
		t.Fatalf("oversized source lost selected posting title/company: %+v", got)
	}
	if got.Description != "" {
		t.Fatalf("oversized canonical description was parsed: got %d bytes", len(got.Description))
	}
	if got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceTooLarge || got.Source.Source != nil {
		t.Fatalf("oversized source state = %+v; want bounded unavailable source", got.Source)
	}
}

func TestWorkableRejectsOversizedPersistedTitleAndCompany(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*workableAccount)
		want   string
	}{
		{
			name: "title",
			mutate: func(account *workableAccount) {
				account.Jobs[1].Title = strings.Repeat("T", 513)
			},
			want: "title",
		},
		{
			name: "company",
			mutate: func(account *workableAccount) {
				account.Name = strings.Repeat("C", 513)
			},
			want: "company name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var account workableAccount
			if err := json.Unmarshal(workableFixture(t), &account); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&account)
			data, err := json.Marshal(account)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := extractWorkableMetadata(data, "northstar", "9AB12CD345"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("oversized Workable %s metadata = %+v, %v; want bounded-field error", tc.name, got, err)
			}
		})
	}
}
