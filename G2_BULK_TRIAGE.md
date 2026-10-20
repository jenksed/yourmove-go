# G2 — Bulk Capture and Next Action

## What changes for the user

- Paste up to 50 posting URLs at once under **Add opportunity → Import multiple job URLs**.
- Get a per-line result for added, already saved, and invalid entries. Retry safely: duplicates will be skipped.
- Every valid URL is saved locally with source status `not_checked` and no fabricated title or salary. Employer metadata is fetched only when you explicitly refresh that job. Keep meaningful query parameters, since some ATS posting IDs live there.
- The Opportunities view highlights one suggested next move. Explicit high/normal/low priority leads; readiness and selected resume break ties; recent jobs are preferred after that. Completed and closed jobs are excluded. This is a deterministic presentation aid, not an AI score or an employer availability claim.
- Single-job Add Opportunity now also skips normalized duplicates without touching manual edits, resume selections, or submission history.

## Architecture

- `intake.go`: URL normalization, duplicate indexing, transactional batch import. One transaction per batch, all-or-nothing if SQLite fails, while malformed URLs are isolated and reported. Application-owned mutex serializes simultaneous local intake. No new dependencies or data migrations.
- `triage.go`: pure next-job selection based on existing `Job` fields. No new database status or inference.
- `intake_handlers.go`: bounded POST parser and HTML result rendering; no external network operation. The existing browser/Wails route and CSRF protection apply unchanged.
- `jobs.go`: one shared `insertJob` function for `sql.DB` and `sql.Tx`. Existing `saveJob` remains available for tests and legacy callers.
- `templates/import_results.html`: server-rendered results with contextual links. Wails gets a normal 200 response, avoiding the custom-scheme 303 compatibility problem.

## Quality bar and measurable acceptance

1. Import 20 job URLs, including repeated and invalid lines, in a single action. Show exact added/duplicate/invalid counts; only unique valid jobs are added.
2. Repeat the same batch. There must be zero new jobs.
3. Edit a saved job and re-import its URL with different casing, a fragment, or a trailing slash. The original title, tracking state, resume, and application history must remain unchanged.
4. Batch intake performs zero third-party HTTP fetches. New records explicitly show unverified metadata rather than invented titles.
5. The next-action recommendation selects the highest-priority active job and explains the immediate local action. No submitted or closed job is recommended.
6. A simulated SQLite insert failure leaves zero rows from that batch. Concurrent identical intake requests create one record.
7. Existing `go test ./...` and `go test -tags desktop ./...` pass, and browser/Wails import results are rendered safely (including escaped invalid text). Native Wails runs still require hands-on qualification on a Mac.

## Running locally

```sh
go test ./...
go test -tags desktop ./...
go run -tags desktop .
```

Run `go run .` separately for normal browser mode. Both entry points still use the established YourMove application data directory.

## Non-goals

No Playwright, no auto-fill, no artificial fit scoring, no database migrations, no resume rewriting, no background metadata crawling, and no automatic employer submission. Existing G1 resume import and Wails navigation compatibility are untouched.

## Recommended following iteration

Add an explicit, bounded public-metadata enrichment queue with visible progress and failures, ideally reusing the `internal/platforms` extractors. **Do not** let a batch of URLs silently trigger many external requests or conflate a retrieved posting with an open application.
