package discovery

import (
	"context"
	"errors"
	"example.com/yourmove-lite/internal/application/schema"
	"testing"
)

type probe func(context.Context, string) (schema.Result, error)

func (p probe) Inspect(ctx context.Context, s string) (schema.Result, error) { return p(ctx, s) }
func TestRegistryDoesNotVisitUntrustedHosts(t *testing.T) {
	calls := 0
	f := probe(func(_ context.Context, _ string) (schema.Result, error) {
		calls++
		return schema.Result{Platform: "fixture"}, nil
	})
	r := Registry{Greenhouse: f, Ashby: f}
	for _, url := range []string{"https://evil.test/", "file:///tmp/test", "https://jobs.ashbyhq.com.evil.test/foo", "https://user@jobs.ashbyhq.com/render/abc"} {
		if _, err := r.Inspect(context.Background(), url); err == nil {
			t.Errorf("unexpected support %s", url)
		}
	}
	if calls != 0 {
		t.Fatalf("called provider for unsafe host %d", calls)
	}
	_, err := r.Inspect(context.Background(), "https://example.com/job")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
}
