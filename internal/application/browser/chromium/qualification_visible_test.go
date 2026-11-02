//go:build qualification

package chromium

import (
	"context"
	"testing"
)

func TestVisibleFixtureRejectsUnboundDestination(t *testing.T) {
	for _, destination := range []string{
		"https://jobs.ashbyhq.com/example/job",
		"http://localhost:1234/synthetic",
		"http://127.0.0.1:1234/synthetic/other",
		"http://127.0.0.1:1234/synthetic?redirect=elsewhere",
	} {
		if _, err := LaunchVisibleFixture(context.Background(), destination); err == nil {
			t.Fatalf("accepted unbound fixture %q", destination)
		}
	}
}
