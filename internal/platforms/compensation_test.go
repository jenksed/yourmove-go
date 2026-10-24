package platforms

import (
	"strings"
	"testing"
)

func TestDescriptionBlankLinePolicy(t *testing.T) {
	for _, c := range []struct{ blanks, want int }{{0, 0}, {1, 1}, {2, 1}, {3, 2}, {4, 2}, {12, 2}} {
		raw := "First\n" + strings.Repeat(" \t\n", c.blanks) + "Second"
		want := "First" + strings.Repeat("\n", c.want+1) + "Second"
		if got := NormalizeDescription(raw); got != want {
			t.Errorf("%d blank lines: got %q want %q", c.blanks, got, want)
		}
	}
	if got := NormalizeDescription("\r\nFirst\r\n \t\r\n\r\n  • Keep indentation\r\n\r\n"); got != "First\n\n  • Keep indentation" {
		t.Fatalf("line endings/indentation: %q", got)
	}
}

func TestDescriptionSalaryExcerpts(t *testing.T) {
	for _, c := range []struct{ name, text, want string }{
		{"annual", "The base salary range is $90,000–$120,000 per year.", "The base salary range is $90,000–$120,000 per year."},
		{"hourly", "Hourly rate: $24.50 - $29.75 per hour", "Hourly rate: $24.50 - $29.75 per hour"},
		{"currency", "Salary: GBP 70,000–90,000 annually", "Salary: GBP 70,000–90,000 annually"},
		{"heading", "Compensation\n\n$100k - $140k\nper year", "Compensation\n$100k - $140k\nper year"},
		{"period unknown", "Base salary: $90k - $120k", "Base salary: $90k - $120k"},
		{"regional", "US salary: $90k–$120k\nCanada salary: CAD 100k–130k", "US salary: $90k–$120k\nCanada salary: CAD 100k–130k"},
		{"sentence", "We build tools. The salary is $100,000 per year. Remote work is supported.", "The salary is $100,000 per year"},
		{"benefit", "Benefits include $2,000 per year for learning", ""},
		{"conference registration cost", "Conference registration costs $3,000 per year", ""},
		{"annual service cost", "Annual service costs $2,000 per year", ""},
		{"benefits sentence before explicit salary", "Benefits include health insurance. Base salary: $100,000 per year", "Base salary: $100,000 per year"},
		{"compensation benefit", "Compensation includes a $500 wellness stipend per year", ""},
		{"salary includes allowance", "Salary includes a $1,000 equipment allowance.", ""},
		{"salary and relocation budget", "Base salary is $100,000 per year. Relocation budget: $8,000.", "Base salary is $100,000 per year"},
		{"salary and bonus in same sentence", "Salary is $100,000 per year, plus a $10,000 signing bonus.", ""},
		{"salary phrase with reimbursement", "Salary includes reimbursement up to $2,000 per year.", ""},
		{"salary phrase with equity", "Salary includes $20,000 in equity per year.", ""},
		{"salary phrase with stipend", "Salary includes a $1,000 monthly stipend.", ""},
		{"benefit heading", "Benefits\n$2,000 per year", ""},
		{"context bleed", "Salary is competitive.\nReferral reward $1000 per year", ""},
		{"bonus", "Signing bonus: $5,000 per year", ""},
		{"equity", "Equity worth $20,000 per year", ""},
		{"revenue", "Our revenue exceeded $100m per year", ""},
		{"numbers only", "Salary range 90000–120000", ""},
		{"unrelated range", "Manage projects worth $90k–$120k", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := SalaryFromDescription(c.text); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestStructuredSalaryWinsAndDescriptionRemainsSourceText(t *testing.T) {
	raw := "Work\n\n\n\n\nSalary: $90k–$120k per year"
	got := completeMetadata(Metadata{Description: raw, SalaryText: "USD 100000–130000 / year"})
	if got.Description != raw || got.SalaryText != "USD 100000–130000 / year" || got.SalaryOrigin != "employer" {
		t.Fatalf("structured/source precedence: %+v", got)
	}
	got = completeMetadata(Metadata{Description: raw})
	if got.SalaryOrigin != "description" || got.SalaryText != "Salary: $90k–$120k per year" || got.Description != raw {
		t.Fatalf("description fallback: %+v", got)
	}
}

func TestJSONLDSalaryIsOptionalAndPreservesUnits(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{`{"currency":"USD","value":{"minValue":90000,"maxValue":120000,"unitText":"YEAR"}}`, "USD 90000–120000 / YEAR"},
		{`{"currency":"USD","value":{"value":28.5,"unitText":"HOUR"}}`, "USD 28.5 / HOUR"},
		{`{"currency":"USD","value":{"minValue":90000,"maxValue":120000}}`, ""},
		{`{"currency":"USD","value":{"minValue":120000,"maxValue":90000,"unitText":"YEAR"}}`, ""},
		{`{"currency":"USD","value":90000}`, ""},
	} {
		if got := salaryFromJSONLD([]byte(c.raw)); got != c.want {
			t.Errorf("%s: %q want %q", c.raw, got, c.want)
		}
	}
}

func TestProviderDescriptionFallbackAndStructuredPrecedence(t *testing.T) {
	got, err := extractAshbyMetadata([]byte(`{"jobs":[{"id":"fixture","title":"Support Engineer","descriptionPlain":"Salary: $90k–$120k per year"}]}`), "fixture")
	if err != nil || got.SalaryText != "Salary: $90k–$120k per year" || got.SalaryOrigin != "description" {
		t.Fatalf("Ashby fallback: %+v %v", got, err)
	}
	page := `<script type="application/ld+json">{"@type":"JobPosting","title":"Engineer","description":"<p>Salary: $90k–$120k per year</p>","baseSalary":{"currency":"USD","value":{"minValue":100000,"maxValue":130000,"unitText":"YEAR"}}}</script>`
	got, err = extractJobMetadata(strings.NewReader(page))
	if err != nil || got.SalaryText != "USD 100000–130000 / YEAR" || got.SalaryOrigin != "employer" {
		t.Fatalf("JSON-LD precedence: %+v %v", got, err)
	}
	page = strings.Replace(page, `{"currency":"USD","value":{"minValue":100000,"maxValue":130000,"unitText":"YEAR"}}`, `{"currency":"USD","value":123}`, 1)
	got, err = extractJobMetadata(strings.NewReader(page))
	if err != nil || got.Title != "Engineer" || got.SalaryOrigin != "description" {
		t.Fatalf("malformed optional salary lost posting/fallback: %+v %v", got, err)
	}
}
