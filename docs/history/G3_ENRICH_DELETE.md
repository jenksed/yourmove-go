# G3 — Controlled source checks and safe deletion

## Product behavior

- Open **Posting checks** from Opportunities or the sidebar. Explicitly start a maximum 30-job batch of unchecked active postings; a separate action retries up to 30 failed checks.
- At most two source requests run concurrently; each is given a 12-second context budget, and existing ATS HTTP clients have their own bounded timeouts. The page refreshes every three seconds while running. No checks run until a person starts a batch.
- Pause stops assignment of new jobs but allows in-flight requests to finish; Resume continues where the process left off. A new batch cannot start while one is running or paused.
- On app restart, in-memory run progress is gone. **Committed per-job source statuses** survive in SQLite; restart a pending/failed batch to process remaining eligible jobs. In-flight interrupted requests can be retried if no result was committed. Completed `unsupported` records are excluded from automatic retry; individual Refresh from posting remains available.
- Source results use the same atomic refresh code as the individual job route. Manual title/company/description/salary edits remain protected. A failed lookup preserves prior metadata. Neither retrieved nor 404 evidence automatically changes application stage or asserts an employer is still hiring.
- Remove unwanted or test jobs from **Opportunities → Delete selected jobs** by checking rows and typing `DELETE`. The bulk action is limited to 50, requires a checkbox confirmation and is atomic. Jobs with submissions cannot be bulk-deleted. Hidden filtered jobs are automatically unchecked in JS mode.
- Every job has **Overview → Delete this job permanently**, including jobs with recorded submissions. For a job with confirmed submissions, the explicit phrase is `DELETE APPLICATION HISTORY`, accompanied by a second consent checkbox. Deletion removes this job's local submission history; **resume files and their shared registry records remain untouched**. There is no recycle bin or undo; back up before deleting real application history.
- Deletion cannot resurrect a job through a queued enrichment update; a removed ID is recorded as `removed` in the run rather than reinserted.

## Architecture

- `enrichment.go`: a bounded two-worker in-memory coordinator with synchronized snapshots, explicit start/pause/resume, deterministic pure eligibility selection and injectable fetcher; `enrichment_handlers.go` handles HTTP and templates.
- `deletion.go`: validated, transactional deletion policy. `deletion_handlers.go` contains the HTTP confirmation routes. Submission and deletion share the existing intake mutex to avoid racing.
- `templates/enrichment.html`: server-rendered progress; no WebSocket, event bus, framework, or background OS service.
- Wails keeps the existing POST-to-safe-GET navigation adapter, extended only with `/enrichment`.
- No new dependencies, no schema migration, no web tracking, no automatic submission.

## Qualification

```sh
cd ~/Projects/yourmove-go
go test ./...
go test -tags desktop ./...
go test -race ./...
go run -tags desktop .
```

1. Start Posting checks against imported jobs, pause, refresh progress, resume; confirm retrieved/unsupported/failed counts. Repeat to process the next 30 when more jobs remain.
2. Save a manual title, recheck the posting, verify title remains unchanged. Browser mode must show the same result.
3. Select two temporary example.com jobs and bulk-delete them. Reimport the same URLs and verify they are accepted as new. Their resumes are still in the library.
4. Open a job with confirmed submission history and verify bulk deletion refuses it; only delete a real historical record if you deliberately want it gone and understand the loss.
5. Verify native Wails navigation never shows a white page after start/pause/resume/delete actions.

### Limitations

- Wails v3 beta and macOS WebKit need real runtime acceptance checks; HTTP tests cannot certify native navigation.
- This is a source-enrichment queue, **not** an ATS form-filling queue. Only supported posting adapters can extract metadata. No browser rendering or password-protected website access was added.
- Job/deletion history is not recoverable without a separate database backup; deleting job records is permanent.
