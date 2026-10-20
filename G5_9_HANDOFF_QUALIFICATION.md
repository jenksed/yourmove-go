# G5.9 — Controlled Ashby handoff qualification and focused Wails status

## What changes

The primary application inspection page now has **one user-facing status** built from the session's authoritative browser and resume state: Ready to prepare, Waiting for resume field, Checking resume control, Resume selected (verify in Ashby), Resume needs attention, or Application browser closed. Form history is optional troubleshooting information, not a competing progress display. Status polling refreshes the controls on an actionable state transition; normal form activity stays in Chromium.

No new dependencies, database migration, question bank, general-purpose field filling, programmatic Next click, or final-submit click. Selected-job resume authorization, exact hash/bytes, same-posting target ownership, and no automatic reupload after an uncertain outcome are unchanged.

## Synthetic end-to-end qualification

From the repository root after installing G5.8 and G5.9:

```sh
# Normal regressions
go test ./...
go test -tags desktop ./...
go test -race ./...

# Full opt-in controlled browser path (no employer traffic)
YOURMOVE_QUALIFY_BROWSER=1 go test ./internal/application/browser/chromium -run '^TestControlledApplicationHandoff$' -v -count=1 -timeout=60s
```

The test **runs real Chrome/Chromium and the production Go DevTools adapter**, but its application and upload endpoint are served solely by a temporary loopback HTTP server. The test uses a synthetic PDF, never the user's stored candidate data. A test-only posting ownership predicate permits that loopback fixture; production retains Ashby's strict same-posting checks.

The synthetic site has a job-description URL and distinct application URL. The browser must open the application, not the JD. The first application step has no file input; the test explicitly navigates to a second step to simulate the operator (production Go does not click Continue). The second step hydrates late, with separate resume, cover letter, and parser inputs. The real Go `AttachResume` must select the synthetic PDF only into the resume input, causing exactly one multipart upload of exactly the expected bytes/name. A repeat selection must be refused, and the test asserts neither cover-letter upload nor form submission occurs.

If the optional qualification is explicitly requested and no usable Chromium executable is available, the test fails instead of silently skipping.

## What the qualification does NOT establish

A controlled Ashby-shaped site does not prove live Ashby acceptance, the correctness of undocumented real Ashby internal upload endpoints, or Wails WebView behavior on macOS. It checks our Go browser handoff boundary. On macOS, run desktop mode with G5.9, authorize an existing selected resume on one real posting, and verify Ashby's own attachment confirmation without submitting. That live check requires explicit in-product authorization and must not be run by CI.

## Failure policy

- A resume control absent on the current step means **waiting**, not a permanent failure.
- A pre-attachment error can be retried a bounded number of times; show a retry control if the bound is exhausted.
- An error at or after the file-selection command is **uncertain**, and never silently retried.
- A file selected locally is not employer acceptance. The applicant reviews it in Chromium.
- A closed tab offers recovery, but cannot claim restored answers or attachments.
- Submission is always performed by the applicant.
