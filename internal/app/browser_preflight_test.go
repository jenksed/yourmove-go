package app

import (
	"context"
	"errors"
	"testing"

	browser "github.com/jenksed/yourmove-go/internal/application/browser"
	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

type preflightFakeBrowser struct {
	calls          int
	failPreflight  bool
	failAfterWrite bool
	existing       bool
	unverified     bool
}

func (b *preflightFakeBrowser) Observe(context.Context) (browser.Observation, error) {
	return browser.Observation{}, nil
}
func (b *preflightFakeBrowser) Close() error { return nil }
func (b *preflightFakeBrowser) AttachResume(context.Context, string) (chromium.AttachmentResult, error) {
	b.calls++
	if b.existing {
		return chromium.AttachmentResult{}, chromium.ErrExistingDocument
	}
	if b.unverified {
		return chromium.AttachmentResult{Selected: true}, nil
	}
	if b.failPreflight {
		return chromium.AttachmentResult{}, &chromium.PreflightError{Cause: errors.New("transient CDP read error")}
	}
	if b.failAfterWrite {
		return chromium.AttachmentResult{}, errors.New("attachment outcome unknown")
	}
	return chromium.AttachmentResult{Selected: true, Verified: true, Detail: "local input matches"}, nil
}

func TestResumePreflightRetriesOnlyBeforeFileSelection(t *testing.T) {
	fake := &preflightFakeBrowser{failPreflight: true}
	h := sessionHost{factory: func(context.Context, string) (readOnlySession, error) { return fake, nil }}
	if err := h.start(context.Background(), 12, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	defer h.closeAll()
	if err := h.armResume(12, t.TempDir(), "/tmp/mock.pdf", "mock.pdf", nil); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		h.tryResume(context.Background(), 12)
		state := h.state(12)
		if i < 3 && state.ResumeStatus != "waiting" {
			t.Fatalf("failed preflight %d should retry: %+v", i, state)
		}
		if i == 3 && (state.ResumeStatus != "attention" || !state.ResumeRetryable) {
			t.Fatalf("preflight should offer retry: %+v", state)
		}
	}
	h.tryResume(context.Background(), 12)
	if fake.calls != 3 {
		t.Fatalf("unbounded retries: %d", fake.calls)
	}
	if err := h.retryResumePreflight(12); err != nil {
		t.Fatal(err)
	}
	fake.failPreflight = false
	h.tryResume(context.Background(), 12)
	state := h.state(12)
	if state.ResumeStatus != "selected" || state.ResumeRetryable || fake.calls != 4 {
		t.Fatalf("successful retry: %+v calls=%d", state, fake.calls)
	}
	if err := h.retryResumePreflight(12); err == nil {
		t.Fatal("selected file must never be replayed")
	}
}
func TestAmbiguousAttachmentOutcomeNeverRetransmits(t *testing.T) {
	fake := &preflightFakeBrowser{failAfterWrite: true}
	h := sessionHost{factory: func(context.Context, string) (readOnlySession, error) { return fake, nil }}
	if err := h.start(context.Background(), 14, testAshbyPosting); err != nil {
		t.Fatal(err)
	}
	defer h.closeAll()
	if err := h.armResume(14, t.TempDir(), "/tmp/mock.pdf", "mock.pdf", nil); err != nil {
		t.Fatal(err)
	}
	h.tryResume(context.Background(), 14)
	h.tryResume(context.Background(), 14)
	state := h.state(14)
	if fake.calls != 1 || state.ResumeRetryable || state.ResumeStatus != "attention" {
		t.Fatalf("unsafe replay: %+v calls=%d", state, fake.calls)
	}
	if err := h.retryResumePreflight(14); err == nil {
		t.Fatal("unsafe retry permitted")
	}
}

func TestResumeExistingAndUnverifiedAreNotReplayed(t *testing.T) {
	for _, tc := range []struct {
		name string
		fake *preflightFakeBrowser
		want string
	}{
		{"existing", &preflightFakeBrowser{existing: true}, "present"},
		{"unverified", &preflightFakeBrowser{unverified: true}, "attention"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := sessionHost{factory: func(context.Context, string) (readOnlySession, error) { return tc.fake, nil }}
			if err := h.start(context.Background(), 1, testAshbyPosting); err != nil {
				t.Fatal(err)
			}
			defer h.closeAll()
			if err := h.armResume(1, t.TempDir(), "/tmp/mock.pdf", "mock.pdf", nil); err != nil {
				t.Fatal(err)
			}
			h.tryResume(context.Background(), 1)
			h.tryResume(context.Background(), 1)
			state := h.state(1)
			if state.ResumeStatus != tc.want || state.ResumeRetryable || tc.fake.calls != 1 {
				t.Fatalf("state=%+v calls=%d", state, tc.fake.calls)
			}
		})
	}
}
