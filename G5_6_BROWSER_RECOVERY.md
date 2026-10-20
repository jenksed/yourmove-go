# G5.6 — Lost application tab recovery (Wails)

## User problem

When the person closed the Ashby application tab or its Chromium window,
YourMove retained a stale in-memory session. "Recheck this page" returned a raw
HTTP 409 (`no matching Ashby application tab`), without a useful recovery path.
This was a failure of YourMove's session lifecycle, not a resume-selection issue.

## Changed behavior

- `/json/list` no longer treats a closed browser / zero open pages like a foreign
  page such as a login destination. They produce distinct typed errors.
- The background watcher waits for two failed reads before retiring a session,
  allowing brief target changes during navigation. A manual Recheck can retire a
  demonstrably closed session immediately.
- When a tab/window is gone, YourMove closes the orphaned Chromium process,
  releases the temporary staged resume bytes, and shows an **Application browser
  ended** recovery block on the normal inspection page.
- The previously selected job resume and saved field observations remain intact.
  A fresh browser never inherits the authorization to upload the previous file.
- **Open saved posting** starts a read-only browser; **Start a new authorized
  resume handoff** requires fresh, explicit consent and the same hash check.
- When another page remains open (e.g. employer authentication), YourMove does
  not kill the window. It prominently offers **Reopen application** with a
  confirmation, alongside the option to return to the Ashby posting manually.
- Recheck errors return to the usable inspection page instead of a dead-end
  text response. Chromium launch failures also appear on the recovery page.
- The native Wails WebView reloads its controls when the watched browser ends.

## Limits and safety

Reopening a closed employer tab cannot restore browser-only inputs or guarantee
that an upload succeeded at Ashby. An attachment with unknown acceptance is
never automatically replayed. Application submission remains user-controlled.

The Go tests can be run locally on the Mac with:

```sh
go test ./...
go test -tags desktop ./...
go test -race ./...
go run -tags desktop .
```

Acceptance: start a handoff, close the Chromium application tab, wait for the
inspection page to recognize it, then reopen and verify the original selected
resume is intact and has not been reuploaded without new confirmation. Repeat
while on an unrelated authentication tab: it should remain open rather than
being forcibly closed. Browser and Wails native runtime checks are required;
unit tests alone do not qualify real Ashby uploads.
