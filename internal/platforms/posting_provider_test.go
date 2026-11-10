package platforms

import (
	"testing"
)

// PostingProvider must recognize every supported ATS platform using the same
// identity logic as retrieval, without network use, and stay silent for
// anything else. The Opportunities list renders this Go-owned value.
func TestPostingProviderRecognizesSupportedPlatforms(t *testing.T) {
	for url, want := range map[string]string{
		"https://jobs.ashbyhq.com/Example/01234567-89ab-cdef-0123-456789abcdef": "ashby",
		"https://job-boards.greenhouse.io/example/jobs/123456":                 "greenhouse",
		"https://jobs.lever.co/example/0e4b8b1e-1234-4a5b-8c9d-0e1f2a3b4c5d": "lever",
		"https://example.recruitee.com/o/platform-engineer":                  "recruitee",
		"https://example.jobs.personio.de/job/12345":                         "personio",
		"https://example.teamtailor.com/jobs/12345-platform-engineer":         "teamtailor",
		"https://jobs.smartrecruiters.com/SampleCo/744000123456789":           "smartrecruiters",
		"https://apply.workable.com/example/j/0123456789ABCDEF":              "workable",
		"https://jobs.jobvite.com/example/job/0ab1c2d3e4f5":                  "jobvite",
		"https://example.jobs.personio.de.evil.test/job/12345":               "",
		"https://user@example.recruitee.com/o/platform-engineer":              "",
		"https://example.com/role":                                           "",
		"https://jobs.lever.co/example":                                         "",
		"not a url":                                                          "",
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
