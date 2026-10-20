# G5.2 — Managed Ashby form observation

## Scope

This iteration adds a **manually navigated, read-only Chromium inspector** to the Go application. It deliberately does not implement autofill, Next/Continue clicks, resume attachment, candidate-data transmission, authentication handling, or employer submission.

### Workflow

1. Import a real Ashby posting `https://jobs.ashbyhq.com/{board}/{uuid}`.
2. Open its **Application Inspection** workspace. Public posting metadata remains separately accessible.
3. Click **Open inspection browser**. A visible, isolated Chromium window opens to exactly the saved posting URL. It does not share your default browser cookies or login session. Only one inspection browser can run at a time.
4. In the **managed** window, use the site's own controls to navigate from the posting to the application. Log in or complete verification manually if you choose. Be careful: this is a real employer page, and any button you press in it may have effects. YourMove **does not** click it for you.
5. Return to YourMove and click **Reinspect current step**. Only visible structural information is collected: field label, input type, required status, option labels, form count, action captions, the current same-posting URL, and the number/hosts of embedded frames. No form values, passwords, tokens, cookies, file bytes, screenshots, browser storage, or requests are captured.
6. Change a conditional answer or move to the next page yourself, then recheck again. Distinct observations are saved in SQLite. Choose any previous snapshot to inspect it. The last public API observation remains separate from the rendered browser snapshots.
7. Close the inspection browser. Saving browser snapshots does not keep the browser session alive after restarting YourMove.
8. **Utilities → Discovered requirements** groups public and browser findings by separate provenance; it does not assert answer equivalence.

### Configuration

- Chrome or Chromium must be installed on the computer running Go. Chrome is located on macOS under `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, or found using PATH. Set `YOURMOVE_CHROMIUM` to an absolute executable path if needed.
- YourMove launches Chromium with a fresh owner-only temporary profile and a random loopback CDP debugging port. It closes the browser and removes the profile when the inspection is ended or the app shuts down normally. An abnormal OS crash may leave a temporary directory or process behind.
- `YOURMOVE_CHROME_NO_SANDBOX=1` is for isolated CI environments with Chromium sandbox problems. **Never set it for normal production use.**
- Browser control uses `github.com/coder/websocket` v1.8.14, already present as a Wails transitive dependency in G5. It is promoted to an explicit dependency; no Node runtime or Playwright is introduced.
- Both Wails and `go run .` drive the same locally managed Chromium process; the Wails WebView remains YourMove's interface, not the form-rendering environment.

### Architecture and trust boundaries

- `internal/application/browser`: browser-neutral, immutable observation types, exact Ashby posting ownership checking, a fixed read-only DOM script and conversion to `schema.Result`.
- `internal/application/browser/chromium`: CDP process/target/session control. Only the fixed `Runtime.evaluate` observation command is transmitted; no arbitrary user-supplied JS is executed, and no click/fill/upload commands exist.
- `browser_handlers.go`: small single-session host and HTTP routes (`/jobs/{id}/browser/{start|observe|stop}`). Browser factory is injectable for synthetic tests.
- `application_inspections.go`: reuses G5's inspection tables, adding separate per-source queries and persisted historical browser-snapshot browsing, with no schema migration.
- `requirements_handlers.go`: latest inspection is selected per **job and source**, so an incomplete rendered snapshot cannot hide Greenhouse public questions or prior published requirements.

A browser observation is **partial** by definition. Chromium may load additional fields later, conditional controls may be hidden, custom controls may not have native `required` attributes, and cross-origin iframes are reported but not inspected. A step containing a `Continue` control is never classified as safe to auto-click; its form method and destination are displayed as conservative evidence for future qualification. We do not automatically follow popup windows outside the saved Ashby posting identity.

### Regression and manual acceptance

```sh
cd ~/Projects/yourmove-go
git apply --check "$HOME/Downloads/yourmove-g5-2-ashby-browser-inspection.patch"
git apply "$HOME/Downloads/yourmove-g5-2-ashby-browser-inspection.patch"
go test ./...
go test -tags desktop ./...
go test -race ./...
go run -tags desktop .
```

For safe qualification use `YOURMOVE_DATA_DIR` pointing to a separate directory and a controlled Ashby-shaped synthetic application in an isolated test environment. Direct real Ashby pages may change or block automation; do not interpret missing controls as no requirements.

1. Inspect an existing Ashby posting using the G5 public API action; see an explicitly incomplete public schema.
2. Start Chromium from the same job's Inspection page. The browser should show the correct job while YourMove stays visible.
3. In the managed browser, navigate to the application form manually, then recheck. The inspection must show observed fields, required flags, dropdown options and action captions, **without values or prefilled text**.
4. If a manually selected answer reveals a conditional question, recheck; inspect both distinct saved observations. Repeat after same-tab navigation or same-posting popup navigation.
5. Navigate the browser to a different site or posting. Recheck must reject and store nothing.
6. Close the browser and restart the app; previously saved observations remain, browser state does not.
7. Verify G5 public Greenhouse inspection, candidate profile, documents, manual submission, and browser/Wails redirects remain unchanged.

### Verification limits

Pure observation/model and provider tests can run in a constrained Linux container. Full database tests require the project's Go version and SQLite dependencies; macOS Wails behavior requires a native run. Synthetic DOM observation can be qualified in Chromium without sending any data to an employer. Live Ashby multipage behavior and popup qualification are **not guaranteed across all employer implementations**. No live candidate information or documents should be transmitted during this milestone.

### Next milestone

G5.3: build an explicit per-job preparation checkpoint and selected-document manifest, then qualify read/write boundaries with a **local synthetic Ashby form** before any live candidate-data or resume transmission. Do not add a generic question bank until the accumulated observations show which employer questions are genuinely reusable.
