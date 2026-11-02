package playwright

import (
	"context"
	"net/url"
	"strings"

	"github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/providers/ashby"
)

// applicationLanding resolves the exact employer application route for the
// first browser launch, mirroring CDP launch behavior: Ashby's public
// posting API supplies the applyUrl, Lever appends /apply, and all other
// platforms open the saved posting URL.
func applicationLanding(ctx context.Context, posting string) (string, error) {
	if browser.PostingPlatform(posting) == "ashby" {
		landing, err := ashby.New(nil).ApplicationDestination(ctx, posting)
		if err != nil {
			return "", err
		}
		return landing, nil
	}
	if browser.PostingPlatform(posting) == "lever" {
		u, _ := url.Parse(posting)
		u.Path = strings.TrimSuffix(u.Path, "/") + "/apply"
		return u.String(), nil
	}
	return posting, nil
}

func (s *Session) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func safeObservedAddress(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "unknown address"
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}
