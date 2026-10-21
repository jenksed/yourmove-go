package browser

import (
	"net/url"
	"regexp"
	"strings"
)

var postingToken = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var postingUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var greenhouseID = regexp.MustCompile(`^[1-9][0-9]*$`)

// PostingPlatform accepts only supported, complete posting identities. A board
// index, sign-in page, custom employer host, or partial ID stays on the manual path.
func PostingPlatform(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawPath != "" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || !postingToken.MatchString(parts[0]) {
		return ""
	}
	switch u.Host {
	case "jobs.ashbyhq.com":
		if len(parts) == 2 && postingUUID.MatchString(parts[1]) {
			return "ashby"
		}
	case "job-boards.greenhouse.io", "boards.greenhouse.io":
		if len(parts) == 3 && parts[1] == "jobs" && greenhouseID.MatchString(parts[2]) {
			return "greenhouse"
		}
	case "jobs.lever.co", "jobs.eu.lever.co":
		if len(parts) == 2 && postingUUID.MatchString(parts[1]) {
			return "lever"
		}
	}
	return ""
}

// OwnsPostingURL keeps every observation and transfer on the exact saved origin
// and posting. Greenhouse board/error redirects and neighboring jobs never match.
func OwnsPostingURL(saved, observed string) bool {
	platform := PostingPlatform(saved)
	if platform == "" {
		return false
	}
	s, _ := url.Parse(saved)
	o, err := url.Parse(observed)
	if err != nil || o.Scheme != s.Scheme || o.Host != s.Host || o.User != nil || o.RawPath != "" {
		return false
	}
	base := strings.TrimSuffix(s.Path, "/")
	path := strings.TrimSuffix(o.Path, "/")
	if path == base {
		return true
	}
	switch platform {
	case "ashby":
		return strings.HasPrefix(path, base+"/")
	case "lever":
		return path == base+"/apply"
	}
	return false
}
