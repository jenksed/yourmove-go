package chromium

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/yourmove-lite/internal/application/browser"
)

func TestTransfersCancelWhileSessionBusy(t *testing.T) {
	s := &Session{}
	s.mu.Lock()
	defer s.mu.Unlock()
	operations := map[string]func(context.Context) error{
		"contacts": func(ctx context.Context) error { _, err := s.FillContacts(ctx, browser.ContactValues{}); return err },
		"resume":   func(ctx context.Context) error { _, err := s.AttachResume(ctx, "/unreachable.pdf"); return err },
		"cover-file": func(ctx context.Context) error {
			_, err := s.AttachCoverLetter(ctx, "/unreachable.pdf", "")
			return err
		},
		"cover-text":  func(ctx context.Context) error { _, err := s.FillCoverLetter(ctx, "unused"); return err },
		"answer":      func(ctx context.Context) error { _, err := s.FillAnswer(ctx, browser.AnswerValue{}); return err },
		"observation": func(ctx context.Context) error { _, err := s.Observe(ctx); return err },
	}
	for name, run := range operations {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			defer cancel()
			if err := run(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("busy session did not honor cancellation: %v", err)
			}
		})
	}
}

func TestCDPReadLimitIsBoundedButAboveNormalFormResponses(t *testing.T) {
	if maxCDPMessageBytes <= 32768 || maxCDPMessageBytes > 8<<20 {
		t.Fatalf("unexpected CDP read limit: %d", maxCDPMessageBytes)
	}
}
func TestFileSurveyIdentityBeforeMutation(t *testing.T) {
	first := []fileInput{{Label: "Resume", Accept: ".pdf"}, {Label: "Cover letter", Accept: ".pdf"}}
	same := append([]fileInput(nil), first...)
	if !sameFileInputs(first, same) {
		t.Fatal("same controls differed")
	}
	same[0].Existing = true
	if sameFileInputs(first, same) {
		t.Fatal("changed attachment must invalidate survey")
	}
	if sameFileInputs(first, first[:1]) {
		t.Fatal("removed field must invalidate survey")
	}
}
func TestPreflightErrorPreservesCause(t *testing.T) {
	cause := errors.New("read failed")
	err := &PreflightError{Cause: cause}
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "preflight") {
		t.Fatal(err)
	}
}
