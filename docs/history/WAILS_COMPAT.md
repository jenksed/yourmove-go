> **Historical reference:** Historical Wails compatibility notes. For current instructions, see [the quickstart](../TRY_IT.md) and [macOS release guide](../../release/macos/README.md).

# Wails v3 native WebView compatibility spike

This is **not** a packaged application. Run from the repository root so the
existing `templates/`, `static/`, and `jobs.db` paths remain unchanged. The
browser mode continues to use `go run .` with the same SQLite file.

## Mac prerequisites and commands

- Go 1.27.2 (per `go.mod`), macOS Xcode Command Line Tools, CGO enabled.
- Pin Wails v3.0.0-beta.32. Run `go mod tidy` once to resolve Wails and update
  `go.sum` on your machine; include `go.sum` in the eventual commit.

```sh
cd ~/Projects/yourmove-go
go mod tidy
go test ./...
go test -tags desktop ./...
go run -tags desktop .
```

## Manual acceptance checklist

1. Open `/jobs` in the Wails window. Check CSS and logo; follow an existing job to `/jobs/{id}` and back.
2. Add `https://example.com/wails-smoke` with title `Wails form test`. The source
   is intentionally unsupported, so no third-party fetch is required. Verify
   that POST returns to `/jobs`, creates exactly one job, and retains its title.
3. Open the saved job; edit company/title/priority and save. Confirm redirect
   to detail and fields persisted. The edit route exercises several form values.
4. Refresh the saved job (unsupported is expected), then refresh a real supported
   Lever/Ashby job and verify that returned metadata and source status update.
5. Close the window, rerun `go run -tags desktop .` from **the same directory**.
   The job and edits must remain. Browser mode `go run .` should see them too.
6. Use `go run .` and repeat add/edit/refresh through the ordinary browser.

If any step fails, preserve HTTP status, current URL, and terminal output;
**do not** rewrite all form handling into Wails bindings. POST bodies, 303s,
and origin handling are the compatibility boundaries being tested.

## Isolated blob-backed request-body probe

```sh
YOURMOVE_WAILS_PROBE=1 go run -tags desktop .
```

This opens a separate diagnostic page in the native window. Click **Run body
probes**. The URL-encoded POST should report `urlencoded_ok: true`. The
blob-backed multipart POST may report `blob_ok: false` or a parse error on
macOS due to WebKit's custom asset-scheme limitation. This has no production
upload route or changes to the normal job UI. Do not treat a multipart
failure as a failure of URL-encoded forms.

## Constraints

- Wails v3 is beta; the actual native build and runtime behavior must be
  qualified on macOS.
- No `.app` packaging, asset embedding, desktop data migration, browser automation,
  persistent logging, or new user-facing feature is in this slice.
- The current exact-origin protection stays in browser mode. Desktop Wails
  POSTs are permitted only when the Wails asset bridge marks the request and
  the WebView origin is exactly `wails://localhost`.

## macOS POST/303 compatibility

macOS WebKit's Wails custom scheme may display a blank window after an HTTP
303 response. Desktop mode now adapts successful form POST redirects with
safe internal destinations (`/jobs` and `/jobs/{id}`) into a 200 HTML page
that immediately navigates with a GET. Browser mode still returns normal
303 responses. A successful desktop POST logs `desktop navigation: POST ...`.
If the WebView still goes blank, verify the job actually persisted by running
`go run .` and opening the browser dashboard. The compatibility adapter does
not modify request bodies or persistence logic.

## G1 Application Preparation (development workflow)

Both `go run .` and `go run -tags desktop .` now use the same data directory:
`~/Library/Application Support/YourMove` on macOS. Set an absolute
`YOURMOVE_DATA_DIR` to override it. If that directory has no database yet,
a consistent SQLite snapshot of a repository-local `jobs.db` is imported.
The original is retained as a backup, not deleted. Check the migrated jobs
before creating any new records. Do not run both versions against different
existing data directories and assume they will automatically merge.

1. Run `go test ./...` and `go test -tags desktop ./...`.
2. Start desktop mode and open **Resume library**.
3. Use **Choose PDF or DOCX** to import via the macOS native picker.
   Browser mode uses an ordinary multipart upload instead.
4. Open a saved job. Assign the imported resume and save selection.
5. Open the employer page and submit *manually*. Only after that, check the
   confirmation box and click **Record completed application**.
6. Verify **Applied** and the resume snapshot in submission history.
7. Restart the app; confirm jobs, resume assignments, and history survive.

No automated application submission, resume matching, data export,
installer, embedded assets, or third-party telemetry is included.
Note: Wails blob-backed uploads remain unqualified; the native chooser
prevents depending on that path.
