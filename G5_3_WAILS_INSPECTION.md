# G5.3 — Wails-first inspection reliability

## User workflow

1. Open an active saved Ashby job and select **Inspect application**.
2. Press **Open application inspector** once. An isolated, visible Chrome/Chromium window opens to the saved Ashby posting. You navigate that window yourself.
3. **Open Apply** (or the application page) in Chromium. YourMove checks the current page approximately every 2.2 seconds, requires two matching structural inspections before saving a new observation, and keeps earlier snapshots.
4. Return to YourMove to review observed fields. The inspection page checks read-only status every 2.5 seconds while visible. When a different snapshot is saved, it refreshes the page. If this status fetch fails in Wails, use **Capture now**. No form values, candidate facts, or file bytes are read or transmitted.
5. Navigate to another application step or reveal a conditional question in Chromium. YourMove records a distinct observation if the **form structure** changes; simply typing text does not record additional snapshots.
6. Press **Finish inspection and close browser** when done. This closes that temporary browser process. Existing saved observations remain available, including after app restart.

Browser and Wails UI have the same inspection workflow; WebView is never used to load the employer application. The job posting's normal **Open employer** action continues opening the system browser.

## Corrections relative to G5.2

- The G5.2 inspection template included a `<section>` and nested `<form>` elements inside the public inspection `<button>`. Browsers, including macOS WebKit, may silently repair this invalid structure. The entire inspection page now uses independent valid forms, predictable single-purpose buttons and a single primary inspection action.
- **Important:** the G5.2 desktop redirect adapter already recognizes `/jobs/{id}/inspection`. This pass adds regression assertions rather than representing it as a new route fix. Browser mode keeps HTTP 303; native Wails keeps the pre-existing 200-and-safe-GET adapter.
- A browser target's CDP websocket URL may use `localhost` instead of `127.0.0.1`. Only those two exact loopback hosts and the expected port are allowed, and the connection URL is normalized to `127.0.0.1`. Same-posting application tabs take precedence over the job overview; unrelated targets are rejected.
- A per-job Go watcher automatically captures stable form-structure changes. It is canceled before closing a session, and job deletion closes an associated session.
- No SQLite schema change, no new Go dependency, no new ATS write or upload capability, no imported TypeScript frontend.

## Qualification

1. Start from a clean, committed G5.2 checkout. Backup `~/Library/Application Support/YourMove` while the app is stopped.
2. Apply the `yourmove-g5-3-wails-inspection-reliability.patch` patch with `git apply --check` first.
3. Run `go test ./...`, `go test -tags desktop ./...`, and `go test -race ./...` on macOS.
4. Launch `go run -tags desktop .`. Verify the Inspection page renders both when no public API result exists and after public discovery has been saved.
5. Open an Ashby inspection. Ensure the original Wails window stays intact and the dedicated Chromium window opens. Return to YourMove: the status should say it's watching. If Chrome/Chromium is not installed, expect a helpful bounded failure, not an empty WebView.
6. In Chromium, visit the application and expose a conditional field (manually). Return to YourMove. Two stable checks (normally several seconds) should yield one saved observation; a second changed step should yield one additional observation. Do not press actual employer submission controls during this inspection.
7. Verify **Capture now**, **Finish inspection and close browser**, browser-mode route redirects, prior history, candidate profile, document selection, and manually recorded submission.
8. Restart YourMove. History should survive; the inspection session should be closed. A different Ashby job should not be attributed to the first one.
9. If Wails's custom scheme blocks automatic `fetch` status polling, the UI explicitly directs you to use **Capture now**. Record the native console or terminal error; do not infer success from the synthetic tests below.

## Evidence limitations

- The isolated Go 1.23 test harness uses local stubs for dependencies because this container cannot download the project's Go 1.27.2 toolchain. It verifies source type-checking and pure state/model tests, **not** real SQLite or macOS Wails.
- Headless Chromium's CDP port and read-only evaluation were exercised locally. This container blocked the test HTTP page as `chrome-error://chromewebdata/`; therefore successful extraction from a real employer page is **not qualified** here.
- macOS native Wails behavior, genuine Ashby multi-step navigation, popup ownership, and real attachment acceptance must still be checked on the machine running YourMove.
- This is structural form observation only. It never fills, uploads, advances steps, or submits, and cannot see controls in uninspected frames or later pages.

## Browser discovery follow-up

The inspector now searches standard system app paths, `~/Applications`, the
Playwright browser cache (`~/Library/Caches/ms-playwright` on macOS), and an
absolute custom `PLAYWRIGHT_BROWSERS_PATH`. Only a full interactive Chromium
binary is selected; Playwright's headless-only shell is excluded. An explicit
`YOURMOVE_CHROMIUM` executable path takes precedence. The browser session still
uses its own temporary profile and has no employer-form write capability.

To diagnose an unusual installation:

```sh
find "$HOME/Library/Caches/ms-playwright" -type f \
  \( -name 'Google Chrome for Testing' -o -name 'Chromium' \) 2>/dev/null
```
