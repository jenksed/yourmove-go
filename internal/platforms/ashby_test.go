package platforms

import (
	"errors"
	"strings"
	"testing"
)

const ashbyTestPostingID = "11111111-1111-4111-8111-111111111111"

func ashbyTestBoardJSON(fields string) []byte {
	return []byte(`{"jobs":[{"id":"` + ashbyTestPostingID + `","jobUrl":"https://jobs.ashbyhq.com/fixture/` + ashbyTestPostingID + `","title":"Platform Engineer",` + fields + `}]}`)
}

func TestParseAshbyURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		board   string
		id      string
		wantErr bool
	}{
		{
			name:  "valid Render posting",
			url:   "https://jobs.ashbyhq.com/render/a3f88090-5e37-43a7-b392-04b02056db6d",
			board: "render",
			id:    "a3f88090-5e37-43a7-b392-04b02056db6d",
		},
		{
			name:    "unsupported host",
			url:     "https://example.com/render/123",
			wantErr: true,
		},
		{
			name:    "missing posting ID",
			url:     "https://jobs.ashbyhq.com/render",
			wantErr: true,
		},
		{
			name:    "insecure URL",
			url:     "http://jobs.ashbyhq.com/render/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, id, err := parseAshbyURL(tt.url)

			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if board != tt.board || id != tt.id {
				t.Errorf(
					"got (%q, %q), want (%q, %q)",
					board, id, tt.board, tt.id,
				)
			}
		})
	}
}

func TestExtractAshbyMetadata(t *testing.T) {
	data := []byte(`{
		"jobs": [
			{
				"id": "other-job",
				"title": "Other Position",
				"descriptionPlain": "Wrong description"
			},
			{
				"id": "a3f88090-5e37-43a7-b392-04b02056db6d",
				"jobUrl": "https://jobs.ashbyhq.com/render/a3f88090-5e37-43a7-b392-04b02056db6d",
				"title": "Software Engineer, Dev Velocity (all levels)",
				"descriptionPlain": "Build developer tooling and automation.",
                "compensation": {"scrapeableCompensationSalarySummary": "$100K - $140K"}
			}
		]
	}`)

	metadata, err := extractAshbyMetadata(
		data,
		"render",
		"a3f88090-5e37-43a7-b392-04b02056db6d",
	)
	if err != nil {
		t.Fatal(err)
	}

	if metadata.Title != "Software Engineer, Dev Velocity (all levels)" {
		t.Errorf("unexpected title: %q", metadata.Title)
	}

	if metadata.SalaryText != "$100K - $140K" {
		t.Errorf("salary summary = %q", metadata.SalaryText)
	}
	if metadata.Description != "Build developer tooling and automation." {
		t.Errorf("unexpected description: %q", metadata.Description)
	}
}

func TestAshbyHTMLDescriptionFallback(t *testing.T) {
	data := []byte(`{"jobs":[{"id":"c7f26e3d-7345-44a6-8e5a-3599679bfe9e","jobUrl":"https://jobs.ashbyhq.com/openai/c7f26e3d-7345-44a6-8e5a-3599679bfe9e","title":"Support Operations Lead","descriptionHtml":"<h2>About the role</h2><p>Lead operations &amp; automation.</p>","compensation":{"scrapeableCompensationSalarySummary":"$279K - $310K"}}]}`)
	got, err := extractAshbyMetadata(data, "openai", "c7f26e3d-7345-44a6-8e5a-3599679bfe9e")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Support Operations Lead" || got.Description != "About the role\nLead operations & automation." || got.SalaryText != "$279K - $310K" {
		t.Fatalf("Ashby fallback: %+v", got)
	}
}

func TestParseAshbyURLRejectsTruncatedUUID(t *testing.T) {
	if _, _, err := parseAshbyURL("https://jobs.ashbyhq.com/render/a3f88090-5e37-43a7-b392-04b02056d"); err == nil {
		t.Fatal("truncated UUID accepted")
	}
}

func TestAshbyCanMatchDocumentedJobURLWithoutID(t *testing.T) {
	data := []byte(`{"jobs":[{"jobUrl":"https://jobs.ashbyhq.com/openai/c7f26e3d-7345-44a6-8e5a-3599679bfe9e","title":"Support Operations Lead","descriptionPlain":"Team description"}]}`)
	got, err := extractAshbyMetadata(data, "openai", "c7f26e3d-7345-44a6-8e5a-3599679bfe9e")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Support Operations Lead" {
		t.Fatalf("posting not found via jobUrl: %+v", got)
	}
}

func TestAshbyPreservesSelectedAlternativesAndDirectFacts(t *testing.T) {
	data := ashbyTestBoardJSON(`"descriptionPlain":" Canonical paragraph ","descriptionHtml":"<h2>Role</h2><p>Rich source</p>","location":"  Berlin, DE  ","workplaceType":"Hybrid","employmentType":"Contract","compensation":{"scrapeableCompensationSalarySummary":"€120K - €150K"}`)
	got, err := extractAshbyMetadata(data, "fixture", ashbyTestPostingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Canonical paragraph" || got.SalaryText != "€120K - €150K" || got.SalaryOrigin != "employer" {
		t.Fatalf("canonical/salary projection changed: %+v", got)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("selected rich source unavailable: %+v", got.Source)
	}
	source := got.Source.Source
	if source.DescriptionPlain != " Canonical paragraph " || source.DescriptionHTML != "<h2>Role</h2><p>Rich source</p>" {
		t.Fatalf("alternatives were not preserved exactly: plain=%q html=%q", source.DescriptionPlain, source.DescriptionHTML)
	}
	if source.Location != "Berlin, DE" || source.WorkplaceType != "Hybrid" || source.EmploymentType != "Contract" || source.SalarySummary != "€120K - €150K" {
		t.Fatalf("direct source facts = (%q, %q, %q, %q)", source.Location, source.WorkplaceType, source.EmploymentType, source.SalarySummary)
	}
	if err := source.Validate("https://jobs.ashbyhq.com/fixture/"+ashbyTestPostingID, got.Description); err != nil {
		t.Fatalf("strict source validation: %v", err)
	}
	if err := source.ValidateIntegrity("https://jobs.ashbyhq.com/other/" + ashbyTestPostingID); !errors.Is(err, ErrPostingSourceAssociation) {
		t.Fatalf("wrong board validation error = %v", err)
	}
	if err := source.Validate("https://jobs.ashbyhq.com/fixture/"+ashbyTestPostingID, got.Description+" edited"); !errors.Is(err, ErrPostingSourceDigest) {
		t.Fatalf("canonical mismatch validation error = %v", err)
	}
	changed := *source
	changed.DescriptionHTML += "<p>new source</p>"
	if err := changed.ValidateIntegrity("https://jobs.ashbyhq.com/fixture/" + ashbyTestPostingID); !errors.Is(err, ErrPostingSourceDigest) {
		t.Fatalf("changed source digest validation error = %v", err)
	}
	changedHTML := strings.Replace(string(data), "Rich source", "different rich source", 1)
	updated, err := extractAshbyMetadata([]byte(changedHTML), "fixture", ashbyTestPostingID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != got.Description || updated.Source.Source.SourceContentSHA256 == source.SourceContentSHA256 {
		t.Fatalf("HTML-only source update changed canonical or kept source digest: canonical=(%q, %q) digest=(%s, %s)", got.Description, updated.Description, source.SourceContentSHA256, updated.Source.Source.SourceContentSHA256)
	}
}

func TestAshbyMapsKnownAndUnknownDirectEnumsWithoutRemoteInference(t *testing.T) {
	for _, tc := range []struct {
		name, workplace, employment string
	}{
		{"remote", "Remote", "FullTime"},
		{"hybrid", "Hybrid", "PartTime"},
		{"onsite", "OnSite", "Temporary"},
		{"unknown wording", "Distributed-ish", "Seasonal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := ashbyTestBoardJSON(`"descriptionPlain":"Role details","workplaceType":"` + tc.workplace + `","employmentType":"` + tc.employment + `","isRemote":true`)
			got, err := extractAshbyMetadata(data, "fixture", ashbyTestPostingID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Source.Source.WorkplaceType != tc.workplace || got.Source.Source.EmploymentType != tc.employment {
				t.Fatalf("direct enum mapping = (%q, %q)", got.Source.Source.WorkplaceType, got.Source.Source.EmploymentType)
			}
		})
	}
	got, err := extractAshbyMetadata(ashbyTestBoardJSON(`"descriptionPlain":"Role details","isRemote":true`), "fixture", ashbyTestPostingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Source.WorkplaceType != "" {
		t.Fatalf("broad isRemote flag became a workplace assertion: %q", got.Source.Source.WorkplaceType)
	}
}

func TestAshbyMalformedOptionalFactsDoNotDiscardCanonicalPosting(t *testing.T) {
	data := ashbyTestBoardJSON(`"descriptionPlain":"Salary: $90k–$120k per year","descriptionHtml":{"not":"a string"},"location":42,"workplaceType":[],"employmentType":null,"compensation":{"scrapeableCompensationSalarySummary":false}`)
	got, err := extractAshbyMetadata(data, "fixture", ashbyTestPostingID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Salary: $90k–$120k per year" || got.SalaryText != got.Description || got.SalaryOrigin != "description" {
		t.Fatalf("malformed optional values changed canonical fallback: %+v", got)
	}
	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("plain source was lost after malformed optional values: %+v", got.Source)
	}
	if got.Source.Source.DescriptionHTML != "" || got.Source.Source.Location != "" || got.Source.Source.WorkplaceType != "" || got.Source.Source.EmploymentType != "" {
		t.Fatalf("invalid optional values were not omitted: %+v", got.Source.Source)
	}
}

func TestAshbyInvalidUTF8OrNULRichAlternativeDoesNotFailUsablePlainText(t *testing.T) {
	prefix := `{"jobs":[{"id":"` + ashbyTestPostingID + `","jobUrl":"https://jobs.ashbyhq.com/fixture/` + ashbyTestPostingID + `","title":"Platform Engineer","descriptionHtml":"`
	suffix := `","descriptionPlain":"Readable canonical text"}]}`
	invalidUTF8 := append([]byte(prefix), 0xff)
	invalidUTF8 = append(invalidUTF8, suffix...)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"invalid UTF-8 HTML", invalidUTF8},
		{"NUL HTML", []byte(prefix + `\u0000` + suffix)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractAshbyMetadata(tc.data, "fixture", ashbyTestPostingID)
			if err != nil {
				t.Fatalf("usable plain canonical text failed retrieval: %v", err)
			}
			if got.Description != "Readable canonical text" {
				t.Fatalf("canonical text = %q", got.Description)
			}
			if tc.name == "invalid UTF-8 HTML" {
				if got.Source.Availability != PostingSourceAvailable || got.Source.Source.DescriptionHTML != "" {
					t.Fatalf("invalid UTF-8 alternative was not omitted: %+v", got.Source)
				}
			} else if got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceInvalid {
				t.Fatalf("NUL source state = %+v", got.Source)
			}
		})
	}
}

func TestAshbySourceBoundsAndSuccessfulCanonicalFallback(t *testing.T) {
	tooLongFact := strings.Repeat("x", 513)
	got, err := extractAshbyMetadata(ashbyTestBoardJSON(`"descriptionPlain":"Role details","location":"`+tooLongFact+`","workplaceType":"Remote"`), "fixture", ashbyTestPostingID)
	if err != nil || got.Source.Source.Location != "" || got.Source.Source.WorkplaceType != "Remote" {
		t.Fatalf("oversized optional fact handling: source=%+v err=%v", got.Source, err)
	}

	oversizedDescription := strings.Repeat("x", MaxPostingSourceInputBytes+1)
	got, err = extractAshbyMetadata(ashbyTestBoardJSON(`"descriptionPlain":"`+oversizedDescription+`","descriptionHtml":"<p>rich</p>"`), "fixture", ashbyTestPostingID)
	if err != nil {
		t.Fatalf("oversized rich source failed canonical retrieval: %v", err)
	}
	if got.Description != oversizedDescription || got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceTooLarge {
		t.Fatalf("oversized source fallback = canonical bytes %d, source %+v", len(got.Description), got.Source)
	}

	got, err = extractAshbyMetadata(ashbyTestBoardJSON(`"location":"Berlin","workplaceType":"Remote"`), "fixture", ashbyTestPostingID)
	if err != nil || got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("facts-only retrieval was not retained: metadata=%+v err=%v", got, err)
	}
	if got.Description != "" || got.Source.Source.Location != "Berlin" || got.Source.Source.WorkplaceType != "Remote" {
		t.Fatalf("facts-only source lost data or invented a description: metadata=%+v source=%+v", got, got.Source.Source)
	}
	if err := got.Source.Source.Validate("https://jobs.ashbyhq.com/fixture/"+ashbyTestPostingID, got.Description); err != nil {
		t.Fatalf("facts-only source failed identity/digest validation: %v", err)
	}

	got, err = extractAshbyMetadata(ashbyTestBoardJSON(`"descriptionPlain":""`), "fixture", ashbyTestPostingID)
	if err != nil || got.Source.Availability != PostingSourceUnavailable || got.Source.Reason != PostingSourceAbsent {
		t.Fatalf("empty successful retrieval: metadata=%+v err=%v", got, err)
	}
}
