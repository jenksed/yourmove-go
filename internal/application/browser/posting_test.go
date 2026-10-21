package browser

import "testing"

func TestSupportedPostingOwnership(t *testing.T) {
	for _, tc := range []struct{ posting, platform, application string }{
		{"https://job-boards.greenhouse.io/cordance/jobs/5423762008", "greenhouse", "#application"},
		{"https://boards.greenhouse.io/cordance/jobs/5423762008", "greenhouse", "?gh_src=email#app"},
		{"https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "lever", "/apply?lever-source=email"},
		{"https://jobs.eu.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "lever", "/apply"},
	} {
		t.Run(tc.posting, func(t *testing.T) {
			if PostingPlatform(tc.posting) != tc.platform || !OwnsPostingURL(tc.posting, tc.posting+tc.application) {
				t.Fatal("supported application rejected")
			}
			for _, foreign := range []string{
				"https://job-boards.greenhouse.io/cordance?error=true",
				"https://job-boards.greenhouse.io/cordance/jobs/99999",
				"https://jobs.lever.co/acme/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb/apply",
				"https://jobs.lever.co/acme/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/other",
				"https://evil.test/application",
			} {
				if OwnsPostingURL(tc.posting, foreign) {
					t.Fatalf("accepted unrelated destination %s", foreign)
				}
			}
			result, err := AsResult(tc.posting, Observation{URL: tc.posting + tc.application})
			if err != nil || result.Platform != tc.platform {
				t.Fatalf("wrong observation provider: %+v %v", result, err)
			}
		})
	}
	for _, invalid := range []string{
		"https://job-boards.greenhouse.io/cordance?error=true",
		"https://job-boards.greenhouse.io/cordance/jobs/0",
		"https://job-boards.greenhouse.io.evil.test/cordance/jobs/5423762008",
		"http://job-boards.greenhouse.io/cordance/jobs/5423762008",
		"https://job-boards.greenhouse.io:443/cordance/jobs/5423762008",
		"https://user@job-boards.greenhouse.io/cordance/jobs/5423762008",
		"https://jobs.lever.co/acme/truncated",
		"https://jobs.ashbyhq.com/acme/12345678-1234-1234-1234-truncated",
	} {
		if PostingPlatform(invalid) != "" || OwnsPostingURL(invalid, invalid) {
			t.Fatalf("invalid posting supported: %s", invalid)
		}
	}
}
