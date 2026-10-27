package browser

import (
	"errors"
	"strings"
)

// LocalizeScript rewrites engine-named UI copy inside shared page scripts
// for non-Chromium engines. The Chromium label short-circuits so the
// release-path CDP driver keeps byte-identical sources.
func LocalizeScript(script, label string) string {
	if label == "" || label == "Chromium" {
		return script
	}
	return strings.ReplaceAll(script, "Chromium", label)
}

// AttachmentResult is the browser-neutral outcome of one authorized document
// or text transfer. Selected means the file/value is present in the live
// employer control; Verified means it was read back and matches the approved
// material. Neither proves employer acceptance or submission.
type AttachmentResult struct {
	Selected bool
	Verified bool
	Detail   string
}

// PreflightError is emitted only before a file selection reaches the employer
// page. This makes bounded retries safe without risking duplicate uploads.
// All errors after that point remain ambiguous and require human inspection.
type PreflightError struct{ Cause error }

func (e *PreflightError) Error() string { return "document form preflight: " + e.Cause.Error() }
func (e *PreflightError) Unwrap() error { return e.Cause }

// Session-lifecycle sentinels. Every managed engine (CDP Chromium today,
// Playwright engines next) reports these so session ownership, recovery, and
// UI copy stay engine-neutral.
var (
	ErrNoPageTargets      = errors.New("the managed browser has no open pages")
	ErrBrowserUnavailable = errors.New("the managed browser is no longer reachable")
	ErrNoMatchingTab      = errors.New("saved application is not the current browser page")
)

// Document-selection sentinels. Engines share these so authorization,
// retry, and reporting rules do not depend on the driver.
var (
	ErrNoResumeInput        = errors.New("resume input not present on this application step")
	ErrAmbiguousResumeInput = errors.New("resume upload control cannot be identified safely; attach the file manually in the employer browser")
	ErrNoCoverInput         = errors.New("cover-letter file input not present on this application step")
	ErrAmbiguousCoverInput  = errors.New("cover-letter upload control cannot be identified safely")
	ErrDocumentFormat       = errors.New("employer document input does not accept the selected document format")
	ErrExistingDocument     = errors.New("a document is already attached in the employer form; review it manually instead of overwriting")
)
