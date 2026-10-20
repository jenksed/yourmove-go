package chromium

import (
	"errors"
	"strings"
	"testing"
)

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
