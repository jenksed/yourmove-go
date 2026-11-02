package ashby

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// ApplicationDestination returns a same-posting, hosted Ashby application URL.
// The public board's documented applyUrl takes precedence. An unavailable,
// incomplete or unexpected API result falls back to Ashby's /application
// route for this exact posting; never follow an unrelated/third-party URL.
// No applicant data is sent by this lookup.
func (i *Inspector) ApplicationDestination(ctx context.Context, postingURL string) (string, error) {
	board, id, err := posting(postingURL)
	if err != nil {
		return "", err
	}
	original, _ := url.Parse(postingURL)
	fallback := *original
	fallback.Path = "/" + board + "/" + id + "/application"
	fallback.RawPath = ""
	fallback.Fragment = ""
	fallback.RawFragment = ""

	// Do not block opening the native browser on an unavailable public API.
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := i.Inspect(lookupCtx, postingURL)
	if err != nil || !matchesApplicationDestination(board, id, result.ApplicationURL) {
		return fallback.String(), nil
	}
	apply, _ := url.Parse(result.ApplicationURL)
	// Preserve legitimate referral attribution in the original posting link.
	// Never copy arbitrary query parameters or credentials to a new URL.
	query := apply.Query()
	originQuery := original.Query()
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term"} {
		if query.Get(key) == "" && originQuery.Get(key) != "" {
			query.Set(key, originQuery.Get(key))
		}
	}
	apply.RawQuery = query.Encode()
	return apply.String(), nil
}

// Require a route that definitely belongs to this exact Ashby posting. The
// public API may contain custom application links, which require separate
// operator review instead of automatic résumé delivery.
func matchesApplicationDestination(board, id, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "jobs.ashbyhq.com" || u.Port() != "" || u.User != nil || u.Fragment != "" || u.RawPath != "" || u.Opaque != "" {
		return false
	}
	base := "/" + board + "/" + id
	path := strings.TrimSuffix(u.Path, "/")
	return path == base+"/application" || path == base+"/apply"
}
