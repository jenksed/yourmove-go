package platforms

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	ashbyM0PostingID = "11111111-1111-4111-8111-111111111111"
	ashbyM0Fixture   = "testdata/job-intelligence/ashby-long-synthetic.json"
)

func readAshbyM0Fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(ashbyM0Fixture))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAshbyM0FixtureFreezesCurrentCanonicalAndSalaryProjection(t *testing.T) {
	data := readAshbyM0Fixture(t)
	const wantFixtureSHA256 = "008818ef01d7578e6f5b96a8c2ee0907d24ac63273d256bc0299019ee0842b0c"
	fixtureHash := sha256.Sum256(data)
	if got := hex.EncodeToString(fixtureHash[:]); got != wantFixtureSHA256 {
		t.Fatalf("fixture SHA-256 = %s, want %s", got, wantFixtureSHA256)
	}

	got, err := extractAshbyMetadata(data, "fable", ashbyM0PostingID)
	if err != nil {
		t.Fatal(err)
	}
	const wantCanonical = `Senior Platform Engineer

About the role
Fable builds dependable tools for teams that coordinate complex work. We are looking for a senior engineer who enjoys improving the foundations that product teams rely on every day.

You will work with a small platform group and partner closely with application engineers. The role combines thoughtful service design, practical operations, and steady delivery with reliability as a shared responsibility.

What you will do
• Design and maintain Go services that support customer-facing workflows.
• Improve observability and operational readiness:
  • Define useful service-level indicators and dashboards.
  • Write clear runbooks and take part in incident reviews.
  • Share patterns in the engineering handbook.
• Work with product and security partners to make reliability part of normal delivery.

What you will bring
You have experience operating production services and can explain the trade-offs behind your designs. You are comfortable reading unfamiliar code, writing tests, and helping a team make a safe change.

1. Several years building backend software in Go or a similar language.
2. Experience with relational data, service monitoring, and incident response.
3. Care for accessible, maintainable systems and collaborative code review.

We value different paths into engineering. You do not need to match every item to apply.

Location and workplace
This role is remote within the European Union. Team members meet online and gather in person a few times each year.

Compensation
Base salary: €120,000 – €150,000 per year, depending on role scope and experience.

Interview process
The process includes an introductory conversation, a practical systems discussion, and time to meet future teammates. We explain each stage before it begins and leave time for your questions.`
	if got.Title != "Senior Platform Engineer" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Description != wantCanonical {
		t.Fatalf("canonical description changed:\ngot:  %q\nwant: %q", got.Description, wantCanonical)
	}
	const wantCanonicalSHA256 = "35c9e859b907f7e9043ba282564fe4d83b8da62c53e6c75b9eb06ab5735458c6"
	canonicalHash := sha256.Sum256([]byte(got.Description))
	if gotHash := hex.EncodeToString(canonicalHash[:]); gotHash != wantCanonicalSHA256 {
		t.Errorf("canonical SHA-256 = %s, want %s", gotHash, wantCanonicalSHA256)
	}
	if got.SalaryText != "€120K - €150K" || got.SalaryOrigin != "employer" {
		t.Errorf("structured salary = (%q, %q), want (employer summary, employer)", got.SalaryText, got.SalaryOrigin)
	}

	if got.Source.Availability != PostingSourceAvailable || got.Source.Source == nil {
		t.Fatalf("rich source unavailable: %+v", got.Source)
	}
	var fixture struct {
		Jobs []struct {
			DescriptionHTML  string `json:"descriptionHtml"`
			DescriptionPlain string `json:"descriptionPlain"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	source := got.Source.Source
	if len(fixture.Jobs) != 1 || source.DescriptionHTML != fixture.Jobs[0].DescriptionHTML || source.DescriptionPlain != fixture.Jobs[0].DescriptionPlain {
		t.Fatal("selected Ashby HTML/plain alternatives were not retained exactly")
	}
	if source.Location != "Remote - European Union" || source.WorkplaceType != "Remote" || source.EmploymentType != "FullTime" {
		t.Fatalf("direct Ashby facts = (%q, %q, %q)", source.Location, source.WorkplaceType, source.EmploymentType)
	}
	if err := source.Validate("https://jobs.ashbyhq.com/fable/"+ashbyM0PostingID, got.Description); err != nil {
		t.Fatalf("selected source association/digests: %v", err)
	}
	t.Logf("fixture_bytes=%d canonical_bytes=%d", len(data), len(got.Description))
}

func TestAshbyM0HTMLFallbackUsesCurrentFlatteningAndSalaryRules(t *testing.T) {
	var board map[string]any
	if err := json.Unmarshal(readAshbyM0Fixture(t), &board); err != nil {
		t.Fatal(err)
	}
	jobs := board["jobs"].([]any)
	job := jobs[0].(map[string]any)
	job["descriptionPlain"] = "  \n\t "
	delete(job, "compensation")
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}

	got, err := extractAshbyMetadata(data, "fable", ashbyM0PostingID)
	if err != nil {
		t.Fatal(err)
	}
	const wantDescription = `Senior Platform Engineer
About the role
Fable builds dependable tools for teams that coordinate complex work. We are looking for a senior engineer who enjoys improving the foundations that product teams rely on every day.
You will work with a small platform group and partner closely with application engineers. The role combines thoughtful service design, practical operations, and steady delivery with reliability as a shared responsibility.
What you will do
• Design and maintain Go services that support customer-facing workflows.
• Improve observability and operational readiness:
• Define useful service-level indicators and dashboards.
• Write clear runbooks and take part in incident reviews.
• Share patterns in the engineering handbook.
• Work with product and security partners to make reliability part of normal delivery.
What you will bring
You have experience operating production services and can explain the trade-offs behind your designs. You are comfortable reading unfamiliar code, writing tests, and helping a team make a safe change.
• Several years building backend software in Go or a similar language.
• Experience with relational data, service monitoring, and incident response.
• Care for accessible, maintainable systems and collaborative code review.
We value different paths into engineering. You do not need to match every item to apply.
Location and workplace
This role is remote within the European Union. Team members meet online and gather in person a few times each year.
Compensation
Base salary: €120,000 – €150,000 per year, depending on role scope and experience.
Interview process
The process includes an introductory conversation, a practical systems discussion, and time to meet future teammates. We explain each stage before it begins and leave time for your questions.`
	if got.Description != wantDescription {
		t.Fatalf("HTML fallback description changed:\ngot:  %q\nwant: %q", got.Description, wantDescription)
	}
	if got.SalaryText != "Base salary: €120,000 – €150,000 per year, depending on role scope and experience." || got.SalaryOrigin != "description" {
		t.Fatalf("description salary = (%q, %q)", got.SalaryText, got.SalaryOrigin)
	}
}

func TestAshbyM0URLSyntaxBoundaries(t *testing.T) {
	valid := "https://jobs.ashbyhq.com/fable/" + ashbyM0PostingID
	tests := []struct {
		name    string
		url     string
		board   string
		id      string
		wantErr bool
	}{
		{name: "saved posting URL with query and fragment", url: valid + "?source=careers#description", board: "fable", id: ashbyM0PostingID},
		{name: "alternate board is syntactically valid", url: "https://jobs.ashbyhq.com/other-board/" + ashbyM0PostingID, board: "other-board", id: ashbyM0PostingID},
		{name: "HTTP scheme", url: strings.Replace(valid, "https:", "http:", 1), wantErr: true},
		{name: "lookalike host", url: "https://jobs.ashbyhq.com.attacker.example/fable/" + ashbyM0PostingID, wantErr: true},
		{name: "explicit default port", url: "https://jobs.ashbyhq.com:443/fable/" + ashbyM0PostingID, wantErr: true},
		{name: "credentials", url: "https://user:pass@jobs.ashbyhq.com/fable/" + ashbyM0PostingID, wantErr: true},
		{name: "extra destination path", url: valid + "/application", wantErr: true},
		{name: "encoded path separator", url: "https://jobs.ashbyhq.com/fable%2Fother/" + ashbyM0PostingID, wantErr: true},
		{name: "invalid posting ID", url: "https://jobs.ashbyhq.com/fable/not-a-uuid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board, id, err := parseAshbyURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseAshbyURL(%q) error = %v, wantErr=%v", tt.url, err, tt.wantErr)
			}
			if err == nil && (board != tt.board || id != tt.id) {
				t.Fatalf("parseAshbyURL(%q) = (%q, %q), want (%q, %q)", tt.url, board, id, tt.board, tt.id)
			}
		})
	}
}

func TestAshbySelectedRecordRequiresQualifiedExactIdentity(t *testing.T) {
	otherID := "22222222-2222-4222-8222-222222222222"
	requestedURL := "https://jobs.ashbyhq.com/fable/" + ashbyM0PostingID
	otherBoardURL := "https://jobs.ashbyhq.com/other-board/" + ashbyM0PostingID

	response := func(id, jobURL, title string) []byte {
		t.Helper()
		posting := struct {
			ID          string `json:"id,omitempty"`
			JobURL      string `json:"jobUrl,omitempty"`
			Title       string `json:"title"`
			Description string `json:"descriptionPlain"`
		}{ID: id, JobURL: jobURL, Title: title, Description: "Synthetic description for " + title}
		board := struct {
			Jobs []any `json:"jobs"`
		}{Jobs: []any{posting}}
		data, err := json.Marshal(board)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	tests := []struct {
		name       string
		id         string
		jobURL     string
		wantTitle  string
		wantErr    bool
		wantClosed bool
	}{
		{name: "ID-only record lacks full source association", id: ashbyM0PostingID, wantErr: true},
		{name: "missing ID with matching jobUrl is positive", jobURL: requestedURL, wantTitle: "URL fallback"},
		{name: "matching ID and qualified jobUrl", id: ashbyM0PostingID, jobURL: requestedURL, wantTitle: "ID plus URL"},
		{name: "HTTP jobUrl associated with requested posting is invalid", jobURL: "http://jobs.ashbyhq.com/fable/" + ashbyM0PostingID, wantErr: true},
		{name: "wrong-board same ID is rejected", id: ashbyM0PostingID, jobURL: otherBoardURL, wantErr: true},
		{name: "wrong-board same UUID without ID is rejected", jobURL: otherBoardURL, wantErr: true},
		{name: "contradictory ID and matching jobUrl is rejected", id: otherID, jobURL: requestedURL, wantErr: true},
		{name: "foreign jobUrl with matching ID is rejected", id: ashbyM0PostingID, jobURL: "https://untrusted.example/other-posting", wantErr: true},
		{name: "wrong same-board posting URL with matching ID is rejected", id: ashbyM0PostingID, jobURL: "https://jobs.ashbyhq.com/fable/" + otherID, wantErr: true},
		{name: "missing ID and different posting suffix is not found", jobURL: "https://jobs.ashbyhq.com/other-board/" + otherID, wantErr: true, wantClosed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := response(tt.id, tt.jobURL, tt.wantTitle)
			got, err := extractAshbyMetadata(data, "fable", ashbyM0PostingID)
			if tt.wantErr {
				if err == nil {
					t.Fatal("unexpected selected posting")
				}
				if errors.Is(err, ErrClosed) != tt.wantClosed {
					t.Fatalf("not-found error closed classification = %v, want %v: %v", errors.Is(err, ErrClosed), tt.wantClosed, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", got.Title, tt.wantTitle)
			}
			if tt.id == "" && got.Source.Source == nil {
				t.Fatal("qualified missing-ID jobUrl selected without an associated source envelope")
			}
		})
	}
}

func TestAshbyDuplicateMatchingRecordsAreRejectedAsAmbiguous(t *testing.T) {
	data := []byte(`{"jobs":[
		{"id":"11111111-1111-4111-8111-111111111111","jobUrl":"https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111","title":"First duplicate","descriptionPlain":"First record"},
		{"id":"11111111-1111-4111-8111-111111111111","jobUrl":"https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111","title":"Second duplicate","descriptionPlain":"Second record"}
	]}`)
	_, err := extractAshbyMetadata(data, "fable", ashbyM0PostingID)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate association error = %v, want ambiguity", err)
	}
}

func TestAshbyM0MalformedWireResponsesAreDecodeErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "malformed JSON", data: []byte(`{"jobs":[`)},
		{name: "jobs has the wrong type", data: []byte(`{"jobs":{}}`)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractAshbyMetadata(tt.data, "fable", ashbyM0PostingID)
			if err == nil {
				t.Fatal("malformed response unexpectedly decoded")
			}
			if errors.Is(err, ErrClosed) {
				t.Fatalf("decode error was classified as closure: %v", err)
			}
			if !strings.Contains(err.Error(), "decode Ashby response") {
				t.Fatalf("unexpected malformed-response error: %v", err)
			}
		})
	}
}
