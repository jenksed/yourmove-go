# G1 — Resume library and manual application history

The original browser and Wails entry points still share the same Go router, templates, SQLite, and source extraction.

## Storage and migration

By default on macOS, both modes use `~/Library/Application Support/YourMove/jobs.db` and `~/Library/Application Support/YourMove/resumes/`. For isolated testing use `YOURMOVE_DATA_DIR=/absolute/path` in *both* modes.

On first launch when that target database doesn't exist, a legacy `./jobs.db` is copied using SQLite `VACUUM INTO` (includes committed WAL changes) to a staged file and renamed into place. The original database is never removed or modified. A pre-existing destination database is never overwritten. The original remains as a recovery copy; inspect it before deleting. This migration depends on starting from the repository directory where the original jobs.db is located. No automatic merge is attempted when destination and legacy both exist.

Imported files have random on-disk names and are stored with restrictive permissions. Identical bytes are deduplicated by SHA-256. Records hold a display filename, extension, size, timestamp and hash. No PDF/DOCX content is sent to an external service. There is no resume text extraction in G1.

## Use

1. Run `go run .` (browser), or `go run -tags desktop .` on macOS.
2. Visit **Resume library**. Browser mode uses an ordinary multipart HTML form; Wails desktop uses a native macOS picker to avoid the custom-scheme blob upload issue.
3. Import PDF or DOCX (up to 10 MiB). The content signature/ZIP structure must match the extension.
4. Open a saved job; select a resume and save the assignment. The selection persists after restart.
5. Apply through the employer website, then separately check the confirmation box in YourMove and press **Record completed application**.
6. The action saves an immutable resume name/hash snapshot, timestamp and notes; it then sets the job stage to Applied. Resubmitting an identical form token is idempotent; a distinct later confirmation creates another history item.

No employer submission is performed by this feature, and it never infers submission success from browsing. A job can have a manually confirmed submission even if no resume was selected.

## Verification

```sh
cd ~/Projects/yourmove-go
go test ./...
go test -tags desktop ./...
go run -tags desktop .
```

A native user must validate the Wails file picker, download behavior, and form navigation because headless HTTP tests do not exercise WebKit.

**Security limitations:** local browser mode binds only to loopback; there is no multiuser authentication. Avoid accessing the browser-mode port from untrusted browser pages or other local users. We reject cross-site POSTs and cross-site resume downloads, but this is not yet a hardened multiuser document service.
