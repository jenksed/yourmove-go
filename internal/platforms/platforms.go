package platforms

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrUnsupported = errors.New("unsupported job platform")

// ErrClosed means the source confirms that the saved posting is no longer available.
var ErrClosed = errors.New("job closed; no longer hiring")

type Metadata struct {
	Title       string
	Company     string
	Description string
	// SalaryText is structured employer pay or an explicit description excerpt.
	SalaryText   string
	SalaryOrigin string // employer (structured) or description (literal excerpt)
	// Source carries selected employer posting evidence when the provider can
	// preserve it. A zero value from existing callers means a successful
	// canonical-only result; retrieval failures continue to use the error return.
	Source PostingSourceResult
}

func FetchJob(ctx context.Context, jobURL string) (Metadata, error) {
	if isGreenhouseErrorBoard(jobURL) {
		return Metadata{}, ErrClosed
	}
	if parsed, err := url.Parse(jobURL); err == nil {
		host := strings.ToLower(parsed.Hostname())
		if _, err := ICIMSPostingIdentity(jobURL); err == nil {
			return fetchICIMS(ctx, postingClient(), jobURL)
		}
		if _, err := PersonioPostingIdentity(jobURL); err == nil {
			return fetchPersonio(ctx, postingClient(), jobURL)
		}
		if _, err := TeamtailorPostingIdentity(jobURL); err == nil {
			return fetchTeamtailor(ctx, postingClient(), jobURL)
		}
		if _, err := RecruiteePostingIdentity(jobURL); err == nil {
			return fetchRecruitee(ctx, postingClient(), jobURL)
		}

		switch host {
		case "jobs.smartrecruiters.com":
			company, postingID, parseErr := parseSmartRecruitersURL(jobURL)
			if parseErr != nil {
				return Metadata{}, fmt.Errorf("SmartRecruiters URL: %w", parseErr)
			}
			return fetchSmartRecruiters(ctx, postingClient(), company, postingID)
		case "apply.workable.com":
			account, shortcode, parseErr := parseWorkableURL(jobURL)
			if parseErr != nil {
				return Metadata{}, fmt.Errorf("Workable URL: %w", parseErr)
			}
			return fetchWorkable(ctx, postingClient(), account, shortcode)
		case "jobs.jobvite.com":
			site, postingID, parseErr := parseJobviteURL(jobURL)
			if parseErr != nil {
				return Metadata{}, fmt.Errorf("Jobvite URL: %w", parseErr)
			}
			return fetchJobvite(ctx, site, postingID)
		}
		if workdayJobsHost(host) || workdayMyWorkdaySiteHost(host) {
			if _, err := WorkdayPostingIdentity(jobURL); err != nil {
				return Metadata{}, fmt.Errorf("Workday URL: %w", err)
			}
			return fetchWorkday(ctx, jobURL)
		}
	}
	// An incomplete Ashby UUID is an invalid source URL, not an
	// unsupported platform and not proof a posting has closed.
	if u, err := url.Parse(jobURL); err == nil && u.Hostname() == "jobs.ashbyhq.com" {
		board, postingID, err := parseAshbyURL(jobURL)
		if err != nil {
			return Metadata{}, fmt.Errorf("Ashby URL: %w", err)
		}
		return fetchAshby(ctx, board, postingID)
	}

	if board, jobID, err := parseGreenhouseURL(jobURL); err == nil {
		return fetchGreenhouse(ctx, postingClient(), board, jobID)
	}
	if site, postingID, apiHost, err := parseLeverURL(jobURL); err == nil {
		return fetchLever(ctx, postingClient(), site, postingID, apiHost)
	}
	if supportedMetadataURL(jobURL) {
		return fetchJobMetadata(ctx, jobURL)
	}

	return Metadata{}, ErrUnsupported
}
