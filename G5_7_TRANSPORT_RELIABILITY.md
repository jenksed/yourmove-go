# G5.7 — Browser transport and resume handoff reliability

## Confirmed defects

- `github.com/coder/websocket` defaults to 32 KiB per incoming message. Real CDP pages produce larger responses; error `read limited at 32769 bytes` is a transport limit, not a broken resume.
- `DOM.getDocument` requested depth `-1`, transferring the entire Ashby page tree even though only the document ID was needed for `DOM.querySelectorAll`.
- Any upload error previously moved the handoff into `attention` with a permanent no-retry warning, even when the file-input command had never been issued.

## Changes

- A shared local-only DevTools dialer sets an explicit 2 MiB **finite** read limit on every connection: read-only observation and file attachment. Oversized frames still fail safely.
- File-input discovery obtains a shallow DOM root (`depth=1`) and resolves the chosen input using `Runtime.evaluate` object identity plus `DOM.requestNode` instead of transferring the entire tree. A real Chromium CDP fixture verified that a shallow `DOM.querySelectorAll` does **not** work; the object-identity method does.
- The file-input survey is checked again immediately before selecting the file. If it changed, no file-selection command is sent.
- A typed `PreflightError` marks failures that definitively happened before file selection. Those get at most three bounded automatic attempts; then the UI offers `Retry resume check (no file sent yet)`.
- Any error at or after `DOM.setFileInputFiles` is **not** retried automatically, because the employer-side outcome could be unknown. Already selected attachments remain subject to manual employer-acceptance confirmation.
- No new packages outside the existing websocket dependency, no database migration, no added candidate/Question Bank work.

## Validation

Run `go test ./...`, `go test -tags desktop ./...`, `go test -race ./...`, then launch Wails and test a real Ashby form. Verify the correct file label and employer acceptance on the actual live form. On forms where file attachment is ambiguous, continue manually in the same browser without restarting. This patch alone does **not** prove live Ashby acceptance.
