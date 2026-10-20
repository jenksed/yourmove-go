# G4 — Document Library, Role Families and Local Resume Matching

## What changed

- **Posting checks** is now in **Utilities**, with the existing `/enrichment` route, worker and progress state unchanged. The explicit check controls and Wails POST compatibility remain intact.
- **Documents** replaces the Resume Library nav label. Existing PDF/DOCX imports, private storage, SHA-256 integrity checks and downloads still work as before.
- **Documents → Manage role families** lets you create, edit, delete and preview local title-based matching rules.
- Link each resume to exactly one family. Multiple PDFs/DOCX files can share a family. If there are multiple, set exactly one **Primary resume** so automatic matching has a deterministic document to recommend.
- Suggestions appear in Opportunities and a job's Prepare tab. **Use suggested resume** explicitly assigns that file to the job. A selected resume is never overwritten or reselected in the background. Confirmed applications retain their original document snapshot.
- **Eight example role families** are opt-in, editable and idempotent. They cover General Customer Service, Implementation, Technical Account Management, Technical Support Engineering, Application Support, Linux & Hosting Support, Web & Hosting Operations, and Customer Success. Their rules are examples, not universal classifications.

## How matching works (no model or remote service)

Each family has three lists, separated by newlines or commas:

1. **Title phrases** (required): at least one exact literal phrase must appear in the saved posting title, with word boundaries after punctuation normalization. A `go` phrase matches “Go Developer” but not “Django Developer”.
2. **Supporting description phrases**: each adds limited evidence (maximum three bonuses), never creates a match on its own. They cannot override stronger title specificity.
3. **Excluded title phrases**: any one makes that family ineligible for that title; exclusions are deliberately title-only so incidental text in an employer's long description cannot block a match.

The longest/more specific matching title phrase leads. Equally scoring families produce **ambiguous — review manually**, not an arbitrary suggestion. If the winning family has no resume, the UI explains that; with multiple resumes and no primary, it asks you to choose one. With exactly one resume for a family, it becomes the recommendation without a primary flag.

Matching is presentation-only until a person clicks **Use suggested resume** or saves a manual choice. Neither changing rules nor changing the primary resume mutates jobs or historical submissions. Existing matching only uses title and description already saved locally; unextracted or untitled jobs cannot be confidently matched. Scores do **not** measure candidate eligibility, qualifications, or likelihood of hiring.

## Detailed example

Install the example families, import two different technical-support resumes and a Linux resume, assign both support resumes to **Technical Support Engineering**, and make one the primary. Assign the Linux resume to **Linux & Hosting Support**.

- **Senior Technical Support Engineer** with “REST APIs, logs and Python” suggests your primary technical-support resume and explains `technical support engineer` as the title evidence.
- **Linux Systems Administrator** with “NGINX, DNS and MySQL” suggests your Linux resume.
- **Customer Success Manager** matches Customer Success, **not** Technical Account Management.
- A posting titled **Support Specialist** will not be forced into a family unless you explicitly add a suitable title phrase.

Users whose work spans other areas can create unlimited different families and adjust literal phrases without touching Go code or installing an AI model.

## Employer postings in a real browser

Wails macOS does not consistently honor `target="_blank"` from the custom-asset WebView. Desktop now opens every posting in the user's **default system browser** through Wails `application.Get().Browser.OpenURL()`. The WebView stays on the same YourMove job workspace; the familiar POST/303 adapter handles the return navigation. The external URL is loaded only from the saved job ID, checked as `http(s)` without credentials, and cannot be substituted by a browser form field. The opener is not exposed in browser-only mode. Browser-mode links remain ordinary `<a target="_blank" rel="noopener noreferrer">` anchors. This is an OS browser launch, **not an embedded ATS browser or automated preparation**, and does not repair employer-side closed/expired URLs.

For manual macOS qualification, open a posting from both Opportunities and the job workspace, confirm Safari/Chrome/your default browser gets the exact URL (including important query parameters) and YourMove stays usable. Also check that error responses do not create submissions, jobs, or resume changes. `posting_open_test.go` verifies desktop-only routing, origin rejection, and the native GET return response.

## Architecture and migration

- One additive `role_families` table holds editable routing rules. Existing `resumes` gains nullable `role_family_id` and `is_primary` (default false). Both additions are idempotent; existing resume identities and job records remain unchanged.
- `role_families.go` owns rule normalization, validation, persistence and pure deterministic matching; `family_handlers.go` owns bounded form parsing. No new dependencies, no new HTTP mutation style, no changes to `jobs.selected_resume_id` or submissions.
- Browser mode uses real `303` form redirects, Wails converts only known local 303 targets to safe GET navigation (including `/families`).
- This iteration does not extract PDF/DOCX text, generate resumes, fill employer forms, or send candidate information externally.

## Verification on macOS

```sh
cd ~/Projects/yourmove-go
go test ./...
go test -tags desktop ./...
go test -race ./...
go run -tags desktop .
```

1. **Utilities → Posting checks**: check that start/pause/resume still work and the route is no longer a main workspace sidebar item.
2. **Documents → Manage role families → Add eight example families**. Repeat the action; verify no duplicated families or overwritten edits.
3. Import/assign two support resume variants and a Linux resume, mark the correct primary; test the two concrete titles above with the preview form.
4. Open a matching saved job, see the explanation, click **Use suggested resume**, then verify that changing rules doesn't overwrite that selection.
5. Simulate ambiguity using two families with identical title phrases and description rules; verify no suggestion is made. Verify an untitled job remains unmatched.
6. Record a test submission, change primary resumes and role rules, and verify the historical resume name/hash and current manual selection remain intact.
7. Reload browser and native Wails modes and verify data survives without the earlier blank-screen POST redirect issue.

**Limitations:** Native desktop interactions, real SQLite migration, and complete tests require Go 1.27.2 on the Mac. This environment can type-check the Go code against stubbed external packages, render templates, and run pure matcher unit tests, but cannot run the actual SQLite/Wails integration here.
