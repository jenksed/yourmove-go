package platforms

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// Explicitly opt-in public read-only qualification; uses no applicant material,
// credentials, application form navigation, or browser mutation.
func TestPublicFeedLiveSourceQualification(t *testing.T) {
	if os.Getenv("YOURMOVE_QUALIFY_PUBLIC_FEEDS") != "1" {
		t.Skip("public-source qualification is opt-in")
	}
	for _, tt := range []struct{ url, provider, title string }{
		{"https://personio.jobs.personio.de/job/1834171?language=en", "personio", "Staff Software Engineer"},
		{"https://knime.jobs.personio.de/job/2696582?language=en", "personio", ""},
		{"https://career.teamtailor.com/jobs/8118064-uk-account-executive-smb", "teamtailor", "UK Account Executive"},
		{"https://hiire.teamtailor.com/jobs/8099372-senior-backend-engineer", "teamtailor", "Senior Backend Engineer"},
		{"https://make.recruitee.com/o/backend-engineer-freelance-1", "recruitee", "Backend Engineer"},
		{"https://rewire.recruitee.com/o/head-of-recruiting-people-development-mwd", "recruitee", "Head of Recruiting"},
		{"https://careers-origamirisk.icims.com/jobs/4645/software-engineer%2C-modernization/job", "icims", "Software Engineer"},
		{"https://careers-i3-corps.icims.com/jobs/5272/software-engineer/job", "icims", "Software Engineer"},
	} {
		t.Run(tt.provider+"/"+tt.url, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			m, err := FetchJob(ctx, tt.url)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(m.Title, tt.title) || strings.TrimSpace(m.Description) == "" {
				t.Fatal("source lacks expected title or content")
			}
			if m.Source.Source == nil || m.Source.Source.Identity.Provider != tt.provider {
				t.Fatal("missing provider source")
			}
			if err := m.Source.Validate(tt.url, m.Description); err != nil {
				t.Fatal(err)
			}
			t.Logf("public source title=%q canonical_bytes=%d source_sha256=%s", m.Title, len(m.Description), m.Source.Source.SourceContentSHA256)
		})
	}
}
