package app

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/jenksed/yourmove-go/internal/application/browser/chromium"
)

type delayedResumeBrowser struct {
	*fakeBrowser
	entered, release chan struct{}
	result           chromium.AttachmentResult
	err              error
}

func (b *delayedResumeBrowser) AttachResume(context.Context, string) (chromium.AttachmentResult, error) {
	close(b.entered)
	<-b.release
	return b.result, b.err
}
func TestResumeResultCannotReachReplacementSession(t *testing.T) {
	for _, outcome := range []string{"selected", "waiting", "preflight", "uncertain"} {
		t.Run(outcome, func(t *testing.T) {
			old := &delayedResumeBrowser{fakeBrowser: &fakeBrowser{url: testAshbyPosting}, entered: make(chan struct{}), release: make(chan struct{})}
			switch outcome {
			case "selected":
				old.result = chromium.AttachmentResult{Selected: true, Verified: true}
			case "waiting":
				old.err = chromium.ErrNoResumeInput
			case "preflight":
				old.err = &chromium.PreflightError{Cause: errors.New("read")}
			default:
				old.err = errors.New("unknown")
			}
			h := sessionHost{factory: func(context.Context, string) (readOnlySession, error) { return old, nil }}
			if err := h.start(context.Background(), 1, testAshbyPosting, ""); err != nil {
				t.Fatal(err)
			}
			if err := h.armResume(1, t.TempDir(), "old.pdf", "old.pdf", nil); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() { h.tryResume(context.Background(), 1); close(done) }()
			<-old.entered
			if err := h.close(1); err != nil {
				t.Fatal(err)
			}
			h.factory = func(context.Context, string) (readOnlySession, error) { return &preflightFakeBrowser{}, nil }
			if err := h.start(context.Background(), 1, testAshbyPosting, ""); err != nil {
				t.Fatal(err)
			}
			defer h.closeAll()
			if err := h.armResume(1, t.TempDir(), "new.pdf", "new.pdf", nil); err != nil {
				t.Fatal(err)
			}
			before := h.state(1)
			close(old.release)
			<-done
			after := h.state(1)
			if after.ResumeStatus != before.ResumeStatus || after.ResumeMessage != before.ResumeMessage || h.handoffs[1].preflightFailures != 0 {
				t.Fatalf("late %s result changed replacement: before=%+v after=%+v", outcome, before, after)
			}
		})
	}
}

func TestResumeHandoffRejectsExistingBrowserForEarlierDestination(t *testing.T) {
	a, _ := testWorkspace(t)
	if err := a.saveJob(Job{URL: testAshbyPosting, Host: "jobs.ashbyhq.com"}); err != nil {
		t.Fatal(err)
	}
	resume, err := a.importResume("resume.pdf", strings.NewReader("%PDF-1.4\nfixture\n%%EOF"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.assignResume(1, resume.ID); err != nil {
		t.Fatal(err)
	}
	fake := &preflightFakeBrowser{}
	a.browsers.factory = func(context.Context, string) (readOnlySession, error) { return fake, nil }
	if err := a.browsers.start(context.Background(), 1, "https://jobs.ashbyhq.com/earlier/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ""); err != nil {
		t.Fatal(err)
	}
	defer a.browsers.closeAll()
	rec := postApp(newMux(a), "/jobs/1/handoff", url.Values{"destination": {testAshbyPosting}, "resume_id": {strconv.FormatInt(resume.ID, 10)}, "resume_hash": {resume.SHA256}}, "", "")
	if rec.Code != 409 || a.browsers.state(1).ResumeStatus != "" || fake.calls != 0 {
		t.Fatal("earlier browser inherited transfer", rec.Code)
	}
}
