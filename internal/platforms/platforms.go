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

// PostingProvider identifies the ATS platform for a saved posting URL using
// the identity logic as retrieval, without any network use. It reports
// only the provider key ("ashby", "greenhouse", "lever", "recruitee",
// "personio", "teamtailor", "smartrecruiters", "workable", "jobvite",
// "workday", "icims") or "" when the platform cannot be identified.
// A fully qualified posting identity wins; otherwise vendor-operated
// posting hosts still brand the source (postingBrand) so saved links from
// recognizable boards carry their platform even when the link itself is
// not a complete posting. Corporate marketing hosts never brand: they
// share a brand name but are not posting infrastructure. Retrieval,
// browser capture, and Fill keep using the strict identity checks;
// branding mints no Fill authority and implies no posting support.
// React must render this Go-owned value, never its own heuristics.
func PostingProvider(rawURL string) string {
	if parsed, err := url.Parse(rawURL); err == nil {
		if _, err := ICIMSPostingIdentity(rawURL); err == nil {
			return "icims"
		}
		if _, err := PersonioPostingIdentity(rawURL); err == nil {
			return "personio"
		}
		if _, err := TeamtailorPostingIdentity(rawURL); err == nil {
			return "teamtailor"
		}
		if _, err := RecruiteePostingIdentity(rawURL); err == nil {
			return "recruitee"
		}
		host := strings.ToLower(parsed.Hostname())
		switch host {
		case "jobs.smartrecruiters.com":
			if _, _, err := parseSmartRecruitersURL(rawURL); err == nil {
				return "smartrecruiters"
			}
		case "apply.workable.com":
			if _, _, err := parseWorkableURL(rawURL); err == nil {
				return "workable"
			}
		case "jobs.jobvite.com":
			if _, _, err := parseJobviteURL(rawURL); err == nil {
				return "jobvite"
			}
		}
		if workdayJobsHost(host) || workdayMyWorkdaySiteHost(host) {
			if _, err := WorkdayPostingIdentity(rawURL); err == nil {
				return "workday"
			}
		}
	}
	if u, err := url.Parse(rawURL); err == nil && u.Hostname() == "jobs.ashbyhq.com" {
		if _, _, err := parseAshbyURL(rawURL); err == nil {
			return "ashby"
		}
	}
	if isAshbyCorporateCareersURL(rawURL) {
		if _, err := parseAshbyCorporateURL(rawURL); err == nil {
			return "ashby"
		}
	}
	if _, _, err := parseGreenhouseURL(rawURL); err == nil {
		return "greenhouse"
	}
	if _, _, _, err := parseLeverURL(rawURL); err == nil {
		return "lever"
	}
	return postingBrand(rawURL)
}

// postingBrand recognizes an ATS provider from sufficiently trustworthy
// host evidence alone, without requiring a valid posting ID. Every matched
// host is vendor-operated posting infrastructure: an exact public board
// host, a single tenant label under a fixed vendor suffix, or a Workday
// tenant pattern. Bare vendor domains, attacker-controlled suffix games,
// and corporate marketing hosts (notably www.ashbyhq.com) never match.
// Display-only: it never alters the strict posting-identity checks used by
// retrieval, browser capture, and Fill.
func postingBrand(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" {
		return ""
	}
	switch host := strings.ToLower(u.Hostname()); host {
	case "jobs.ashbyhq.com":
		return "ashby"
	case "job-boards.greenhouse.io", "boards.greenhouse.io":
		return "greenhouse"
	case "jobs.lever.co", "jobs.eu.lever.co":
		return "lever"
	case "jobs.smartrecruiters.com":
		return "smartrecruiters"
	case "apply.workable.com":
		return "workable"
	case "jobs.jobvite.com":
		return "jobvite"
	default:
		if workdayJobsHost(host) || workdayMyWorkdaySiteHost(host) {
			return "workday"
		}
	}
	if _, err := vendorPostingURL(raw, ".recruitee.com"); err == nil {
		return "recruitee"
	}
	if _, err := vendorPostingURL(raw, ".teamtailor.com"); err == nil {
		return "teamtailor"
	}
	if _, err := vendorPostingURL(raw, ".icims.com"); err == nil {
		return "icims"
	}
	if _, err := vendorPostingURL(raw, ".jobs.personio.de", ".jobs.personio.com"); err == nil {
		return "personio"
	}
	return ""
}

// PostingProviderLabel is the applicant-facing source name for a provider
// key from PostingProvider. It distinguishes the ATS platform from the
// employer identity and never implies automatic capture or Fill support.
func PostingProviderLabel(provider string) string {
	switch provider {
	case "ashby":
		return "Ashby"
	case "greenhouse":
		return "Greenhouse"
	case "lever":
		return "Lever"
	case "recruitee":
		return "Recruitee"
	case "personio":
		return "Personio"
	case "teamtailor":
		return "Teamtailor"
	case "smartrecruiters":
		return "SmartRecruiters"
	case "workable":
		return "Workable"
	case "jobvite":
		return "Jobvite"
	case "workday":
		return "Workday"
	case "icims":
		return "iCIMS"
	default:
		return ""
	}
}

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

	// Ashby's own corporate careers page links postings with an ashby_jid
	// parameter. Resolve those against Ashby's verified public board; other
	// hosts carrying an ashby_jid parameter stay unsupported.
	if isAshbyCorporateCareersURL(jobURL) {
		postingID, err := parseAshbyCorporateURL(jobURL)
		if err != nil {
			return Metadata{}, fmt.Errorf("Ashby URL: %w", err)
		}
		return fetchAshby(ctx, ashbyCorporateBoard, postingID)
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
