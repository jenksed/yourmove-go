package app

import (
	"context"
	"testing"
	"time"

	"github.com/jenksed/yourmove-go/internal/application/browser"
)

type steppedObservationSession struct {
	ready        chan struct{}
	observations chan browser.Observation
}

func (s *steppedObservationSession) Observe(ctx context.Context) (browser.Observation, error) {
	select {
	case s.ready <- struct{}{}:
	case <-ctx.Done():
		return browser.Observation{}, ctx.Err()
	}
	select {
	case observation := <-s.observations:
		return observation, nil
	case <-ctx.Done():
		return browser.Observation{}, ctx.Err()
	}
}

func (*steppedObservationSession) Close() error { return nil }

func TestWatcherRequiresConsecutiveMatchingObservations(t *testing.T) {
	session := &steppedObservationSession{
		ready:        make(chan struct{}),
		observations: make(chan browser.Observation),
	}
	host := sessionHost{
		pollEvery: time.Millisecond,
		factory: func(context.Context, string) (readOnlySession, error) {
			return session, nil
		},
	}
	if err := host.start(context.Background(), 8, testAshbyPosting, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(host.closeAll)
	saved := make(chan string, 8)
	host.watch(8, testAshbyPosting, func(o browser.Observation) (int64, error) {
		saved <- o.Controls[0].ID
		return 1, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	waitReady := func() {
		t.Helper()
		select {
		case <-session.ready:
		case <-ctx.Done():
			t.Fatal("watcher did not request the next observation:", ctx.Err())
		}
	}
	waitReady()
	for i, step := range []struct{ observation, wantSave string }{
		{"A", ""}, {"A", "A"},
		{"B", ""}, {"A", ""},
		{"B", ""}, {"B", "B"}, {"B", ""},
	} {
		observation := browser.Observation{
			URL: testAshbyPosting + "/application", Forms: 1,
			Controls: []browser.Control{{ID: step.observation, Label: step.observation, Kind: "text"}},
		}
		select {
		case session.observations <- observation:
		case <-ctx.Done():
			t.Fatal("watcher did not accept observation:", ctx.Err())
		}
		// Entering the next Observe proves the prior iteration finished,
		// including its save callback. No observation can advance until we send it.
		waitReady()
		got := ""
		select {
		case got = <-saved:
		default:
		}
		if got != step.wantSave {
			t.Fatalf("observation %d (%s): saved %q, want %q", i+1, step.observation, got, step.wantSave)
		}
		select {
		case extra := <-saved:
			t.Fatalf("observation %d: unexpected additional save %q", i+1, extra)
		default:
		}
	}
}
