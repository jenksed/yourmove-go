# Application platforms: current scope and next integrations

## Current behavior

The managed browser uses the existing Ashby flow for hosted Greenhouse and Lever postings as well. This is an extension of the current session, observation, and transfer commands, not a second automation engine.

- Ashby: `jobs.ashbyhq.com/<board>/<uuid>`; existing API-derived application destination.
- Greenhouse: `job-boards.greenhouse.io/<board>/jobs/<numeric-id>` and the older `boards.greenhouse.io` host; open the saved posting's embedded application.
- Lever: `jobs.lever.co/<board>/<uuid>` and `jobs.eu.lever.co`; open the same posting's `/apply` route, preserving query attribution.
- Other hosts and incomplete posting URLs: manual application, with a primary Open application action. Download the selected resume and saved cover-letter PDF from Prepare. Final submission and recording remain explicit user actions.

Only uniquely identified native controls are filled. Existing values/files, custom dropdowns, authentication, CAPTCHA, sensitive questions, and cross-origin embedded forms remain manual. A successful file selection does not establish employer acceptance. Account or tenant-specific variations still need live read-only qualification; local fixtures do not prove compatibility with every hosted form.

### Lever qualification detail

The [Spreedly Support Engineer application](https://jobs.lever.co/spreedly/3a6264ff-2a29-41d4-95fa-fa52b86e9c69/apply) was inspected through its public HTML and upload scripts. Its ordinary resume control is `#resume-upload-input[name=resume]`, nested inside the visible upload link and a `.application-question` label. Its optional Apply with LinkedIn widget is a separate `.awli-application-row` / `.awli-button-container`; exclude those containers from observations and contact/document targets. Do not click social-auth buttons or attempt OAuth. Lever may still load its own LinkedIn script when the page opens.

The local Chromium fixture now covers that nested upload shape, a conflicting resume input inside the social widget, one direct resume change event, and zero LinkedIn clicks or submissions. This is not evidence of a successful real Spreedly upload. A “Couldn't auto-read resume” message concerns Lever's parsing step; do not equate it with missing file selection or retry an upload automatically. Inspect the named attachment and the application's own outcome. This particular public form did not expose a cover-letter upload control when inspected; do not invent a destination for a saved letter.

## Code to start from

| Concern | Existing location |
| --- | --- |
| Supported posting shapes and exact destination ownership | `internal/application/browser/posting.go` |
| First application destination and tab selection | `internal/application/browser/chromium/chromium.go` |
| Read-only structural observation | `internal/application/browser/inspect.js`, `observation.go` |
| Unique document purpose, format, existing file and integrity checks | `internal/application/browser/chromium/attachment.go` |
| Qualified contact fields and preservation of manual entry | `internal/application/browser/chromium/contact.go` |
| Saved-answer and cover-letter transfer | `internal/application/browser/chromium/answer.go`, `cover_letter.go` |
| Eligibility, explicit scope, and independent outcomes | `application_handlers.go`, `application_continuation.go` |
| Provider-aware answer persistence and employer-scoped reuse | `application_answers.go`, `reusable_answers.go` |
| Candidate/role/company filenames without modifying source bytes | `attachment_names.go`, `attachment_downloads.go` |
| Closed-posting evidence | `internal/platforms/`, `fetchPostingDetails`, `applyPostingRefresh` |

Use `graft ask` and `graft callers` for current spans before editing. Do not broaden the existing allowlist with a generic "same host means same job" rule.

## Workday next

Start with one user-provided tenant and a saved posting, then record the actual posting, application, sign-in, and step URLs using read-only inspection. Determine how tenant, career site, locale, and requisition ID identify the same opportunity before adding an ownership rule. Do not assume a hostname suffix or a requisition-looking substring proves identity.

Questions to resolve with evidence:

1. Does Apply stay on the tenant origin and retain an exact requisition identity? Which redirects need an explicit, narrowly validated transition?
2. Which steps require sign-in or an account? Leave those steps to the user and resume only after proving the active page still belongs to this job.
3. Which controls are native, and which are custom or dependent controls? Inspect actual labels and machine identifiers before extending the contact resolver. Do not use positional guesses.
4. Does selecting a resume cause parsing, replace contact values, or advance a step? Keep contact, resume, and answer outcomes independent and verify after rerenders.
5. How does the page distinguish a removed posting from authentication, a temporary API failure, or an unavailable tenant? Require affirmative evidence before closing a job.

Keep the manual path available while qualifying one tenant. Add a narrow adapter only when observed route differences require one; do not add a general workflow engine speculatively.

## iCIMS next

Start with one real employer portal and saved job. Inspect the exact portal/job identity, Apply destination, any account transition, and any embedded frame boundary. Custom employer domains must be explicitly validated against the saved opportunity; do not allow unrelated portals just because they share a vendor.

Questions to resolve with evidence:

1. Which parts of the posting and application URLs are stable job identifiers, and which refer to an account, session, or portal?
2. Is the actual form in the top page or an iframe? The current collector does not inspect or mutate controls inside cross-origin frames. Add frame-aware ownership and tests before considering that supported.
3. How are resume versus cover-letter controls identified? An ambiguous Attach control must stay manual.
4. Which application steps require user navigation, profile creation, or consent? Do not carry upload authorization into another job, restart, or account flow.
5. What affirmative evidence indicates closure? A sign-in screen or list of other jobs alone is not sufficient evidence.

## Verification before enabling either platform

- Exact posting identity tests: own steps accepted; other jobs, tenants, account pages, error boards, scheme changes, encoded path tricks, and lookalike hosts rejected.
- Run the existing consent/stale-scope/independent-failure tests for the new platform. Saved answers must use its actual provider and exact destination; reusable answers must remain employer-scoped.
- Local real-Chromium fixtures: contacts, full versus separate names, hidden upload inputs, distinct resume and letter, existing manual values/files, delayed/rerendered controls, and zero form submissions.
- Read-only checks on real pages to establish selector and route evidence. Any real applicant-material transfer requires the user's explicit in-app authorization.
- Browser close/reopen must discard prior transfer authority and remove staged files. No automatic replay after uncertain selection.
- Check the manual Prepare path, named downloads, Wails POST redirect behavior, and the 390px layout.

Useful commands:

```sh
go test ./...
YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification ./internal/application/browser/chromium -run TestHostedPlatformForms -v
YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification ./internal/application/browser/chromium -run TestControlledApplicationHandoff -v
graft build
```

The new hosted-platform fixtures exercise local forms and transfers. They do not contact an employer or submit applications. Native Wails visual review and live tenant qualification should be recorded separately from these test results.
