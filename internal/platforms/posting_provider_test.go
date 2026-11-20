package platforms

import (
	"testing"
)

// PostingProvider must recognize every supported ATS platform using the same
// identity logic as retrieval, without network use. Fully qualified
// postings resolve exactly; vendor-operated board hosts additionally brand
// the source without a posting ID. Corporate marketing hosts, bare vendor
// domains, and anything else stay silent. The Opportunities list renders
// this Go-owned value.
func TestPostingProviderRecognizesSupportedPlatforms(t *testing.T) {
	for url, want := range map[string]string{
		"https://jobs.ashbyhq.com/Example/01234567-89ab-cdef-0123-456789abcdef": "ashby",
		"https://job-boards.greenhouse.io/example/jobs/123456":                  "greenhouse",
		"https://jobs.lever.co/example/0e4b8b1e-1234-4a5b-8c9d-0e1f2a3b4c5d":    "lever",
		"https://example.recruitee.com/o/platform-engineer":                     "recruitee",
		"https://example.jobs.personio.de/job/12345":                            "personio",
		"https://example.teamtailor.com/jobs/12345-platform-engineer":           "teamtailor",
		"https://jobs.smartrecruiters.com/SampleCo/744000123456789":             "smartrecruiters",
		"https://apply.workable.com/example/j/0123456789ABCDEF":                 "workable",
		"https://jobs.jobvite.com/example/job/0ab1c2d3e4f5":                     "jobvite",
		"https://example.jobs.personio.de.evil.test/job/12345":                  "",
		"https://user@example.recruitee.com/o/platform-engineer":                "",
		"https://example.com/role":                                              "",
		// Source branding: vendor posting infrastructure without a posting ID.
		"https://jobs.lever.co/example":             "lever",
		"https://Jobs.Lever.Co/example":             "lever",
		"https://jobs.eu.lever.co/example":          "lever",
		"https://job-boards.greenhouse.io/example":  "greenhouse",
		"https://boards.greenhouse.io/example":      "greenhouse",
		"https://jobs.ashbyhq.com/example":          "ashby",
		"https://jobs.smartrecruiters.com/SampleCo": "smartrecruiters",
		"https://apply.workable.com/example":        "workable",
		"https://jobs.jobvite.com/example":          "jobvite",
		"https://example.recruitee.com":             "recruitee",
		"https://example.teamtailor.com":            "teamtailor",
		"https://example.icims.com":                 "icims",
		"https://example.jobs.personio.com":         "personio",
		"https://example.wd3.myworkdayjobs.com":     "workday",
		"https://wd5.myworkdaysite.com":             "workday",
		// Corporate marketing hosts share a brand name but are not posting
		// infrastructure; bare vendor domains and lookalikes never brand.
		"https://www.ashbyhq.com/careers":      "",
		"https://lever.co":                     "",
		"https://icims.com":                    "",
		"https://recruitee.com":                "",
		"https://jobs.lever.co.evil.test/acme": "",
		"https://evillever.co/":                "",
		"http://jobs.lever.co/example":         "",
		"not a url":                            "",
	} {
		if got := PostingProvider(url); got != want {
			t.Fatalf("PostingProvider(%q)=%q want %q", url, got, want)
		}
		if want != "" && PostingProviderLabel(want) == "" {
			t.Fatalf("missing accessible label for %q", want)
		}
	}
	if PostingProviderLabel("unknown") != "" {
		t.Fatal("unknown provider needs no platform label")
	}
}
