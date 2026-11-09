# Application platforms: capability reference and next integrations

**Authoritative platform-support reference as of `2baa899`** (release branch `v0.2/release-0.2.0`).
Verify against source before extending support; dated prose elsewhere is historical.

**Core rule: distinguish support by operation, not by a blanket supported/unsupported label.**
Job-intake support (recognizing a URL and fetching posting metadata) is **not**
equivalent to managed capture or automated Fill support. A platform can be fully
supported for intake while remaining manual-only for browser operations.

Status grades used below:

- **Implemented + tested** — covered by Go unit tests and, where browser effects
  are involved, controlled Chromium fixtures. Not a claim of universal employer-site
  compatibility or native Wails acceptance.
- **Implemented, not natively verified** — the code path exists and is exercised in
  tests/fixtures, but end-to-end native Wails GUI acceptance on that OS is missing.
- **Unsupported** — the code refuses the operation with an explicit manual fallback.
- **Planned / under investigation** — Workday and iCIMS only; narrow, evidence-gated
  next steps are recorded below. No other platform is committed.

## Capability matrix

| Operation | Ashby | Greenhouse | Lever | Other intake-supported providers¹ | Other / incomplete URLs |
| --- | --- | --- | --- | --- | --- |
| Job URL recognition + intake metadata (`internal/platforms.FetchJob`) | Implemented + tested (`jobs.ashbyhq.com/<board>/<uuid>`, plus corporate-careers `ashby_jid` links resolved against the verified public board) | Implemented + tested (`job-boards.greenhouse.io/<board>/jobs/<numeric-id>` and `boards.greenhouse.io`) | Implemented + tested (`jobs.lever.co/<board>/<uuid>` and `jobs.eu.lever.co`) | Implemented + tested for metadata only: iCIMS, Personio, Teamtailor, Recruitee, SmartRecruiters, Workable, Jobvite, Workday identity repair, Paylocity (`2000recruiting.paylocity.com`) | Unsupported (`ErrUnsupported` or a URL-specific error); never silently treated as closed |
| Public / early question discovery (`internal/application/discovery`, read-only preview) | Early questions via read-only rendered preview only (`previewEarlyQuestions`, `chromium.Preview`, source `preview`). The documented public posting API exposes **no** application questions | Implemented + tested via the public boards API (`?questions=true`; compliance/demographic content marked sensitive, conditional fields unverified) | Unsupported (no public or preview discovery) | Unsupported | Unsupported |
| Managed browser inspection (`browser.PostingPlatform`, `OwnsPostingURL`) | Implemented + tested: exact `https` board/UUID identity; same-posting descendants allowed | Implemented + tested: exact board/job identity; the **same posting path only** (board/error redirects and neighboring jobs never match) | Implemented + tested: exact board/UUID identity; the same posting's `/apply` route allowed, query attribution preserved | Unsupported | Unsupported |
| Field / question capture (read-only observation → `saveInspection`) | Implemented + tested | Implemented + tested | Implemented + tested | Unsupported: managed capture disabled up front | Unsupported: managed capture disabled up front |
| Candidate-profile Fill (contact resolver) | Implemented, not natively verified (controlled Chromium only) | Implemented, not natively verified (same shared resolver flow as Ashby) | Implemented, not natively verified (same shared resolver flow; social-widget controls excluded) | Unsupported | Unsupported |
| Saved-answer Fill | Implemented, not natively verified (controlled Chromium only) | Implemented, not natively verified | Implemented, not natively verified | Unsupported | Unsupported |
| Document transfer (resume / cover letter) | Implemented, not natively verified (controlled Chromium only) | Implemented, not natively verified | Implemented, not natively verified (nested upload shape covered by fixture; see Lever note below) | Unsupported | Unsupported |
| Manual fallback | Always available | Always available | Always available | Manual path: download selected resume and saved letter, open the employer site, complete and submit personally | Manual path with explicit unsupported-posting guidance |
| Final employer submission | Manual only (no automation on any platform) | Manual only | Manual only | Manual only | Manual only |

¹ Intake-only providers have posting-metadata retrieval (`internal/platforms/*`) and
nothing else: no public question discovery, no managed inspection, no Fill. Workday
posting-identity intake was repaired for previously failed records; that repair does
not extend to managed capture or Fill.

The managed browser uses the existing Ashby session, observation, and transfer flow
for hosted Greenhouse and Lever postings as well. This is an extension of the
current commands, not a second automation engine.

### Eligibility and ownership rules enforced by the code

- **Exact posting identity.** `PostingPlatform` accepts only supported, complete
  posting identities: `https`, no credentials, no explicit ports, no encoded-path
  tricks. Board indexes, sign-in pages, custom employer domains, partial IDs, `http`
  URLs, and already-deep application paths stay on the manual path.
- **Exact ownership.** `OwnsPostingURL` keeps every observation and transfer on the
  exact saved origin and posting. Ashby allows same-posting descendants, Lever allows
  the same posting's `/apply`, and Greenhouse requires the same posting path.
- **Capture gating.** `prepareQuestions` returns no live inventory unless the saved
  posting owns itself; browser start/observe routes reject unsupported postings
  (`422`) with guidance to prepare manually. The Questions and continuation APIs
  expose `ownsPosting` so React disables Capture up front instead of failing on
  click (`2baa899`); preserved writing stays legible.
- **Fill gating.** Every Fill revalidates the exact saved destination, field
  identity, wording, kind, options, sensitivity, revision, and live DOM target, and
  preserves existing or actively edited values. A captured question is **not**
  assumed to be automatically fillable. See
  [`../architecture/APPLICATION_FIELD_CLASSIFICATION.md`](../architecture/APPLICATION_FIELD_CLASSIFICATION.md)
  for the authoritative field-role and authorization rules.
- **Submission boundary.** `POST /jobs/{id}/submit` records the applicant's manual
  submission declaration locally. Nothing submits to an employer.

### Control-level limitations (all managed platforms)

Only uniquely identified native controls are filled. Existing values/files, custom
dropdowns and other structured controls, authentication, CAPTCHA, sensitive
questions, and cross-origin embedded forms remain manual. Observation stays
read-only and provider-neutral: accessible custom controls keep their real
question label, choices already visible in the DOM are preserved as read-only
reference (never collected by opening or clicking a widget), and recognizable
controls without a stable identity receive a relationship-scoped display
identity that confers no transfer authority. A successful file selection does
not establish employer acceptance. Account or tenant-specific variations still
need live read-only qualification; local fixtures do not prove compatibility
with every hosted form.

Known investigation example: the live Render posting rotates Ashby's per-form
UUID namespace, and its “How did you hear about Render?” question arrives as an
option-less Ashby combobox without a stable control identity. The question is
represented under its real label with manual-completion guidance while staying
outside automated-Fill scope. Option-less comboboxes, `custom_combobox`,
`listbox`, and `unsupported_ambiguous` controls are never automated-Fill targets.

### Lever qualification detail

The [Spreedly Support Engineer application](https://jobs.lever.co/spreedly/3a6264ff-2a29-41d4-95fa-fa52b86e9c69/apply) was inspected through its public HTML and upload scripts. Its ordinary resume control is `#resume-upload-input[name=resume]`, nested inside the visible upload link and a `.application-question` label. Its optional Apply with LinkedIn widget is a separate `.awli-application-row` / `.awli-button-container`; exclude those containers from observations and contact/document targets. Do not click social-auth buttons or attempt OAuth. Lever may still load its own LinkedIn script when the page opens.

The local Chromium fixture now covers that nested upload shape, a conflicting resume input inside the social widget, one direct resume change event, and zero LinkedIn clicks or submissions. This is not evidence of a successful real Spreedly upload. A “Couldn't auto-read resume” message concerns Lever's parsing step; do not equate it with missing file selection or retry an upload automatically. Inspect the named attachment and the application's own outcome. This particular public form did not expose a cover-letter upload control when inspected; do not invent a destination for a saved letter.

## Code to start from

| Concern | Existing location |
| --- | --- |
| Supported posting shapes and exact destination ownership | `internal/application/browser/posting.go` |
| Field-role decisions (profile / question / document / employer-only / unsupported) | `internal/app/application_field_classification.go`; authoritative reference in `docs/architecture/APPLICATION_FIELD_CLASSIFICATION.md` |
| Questions inventory, `ownsPosting`, and save authority | `internal/app/workspace_writing_api.go`, `internal/app/application_answers.go`, `internal/app/early_application_answers.go` |
| Transfer eligibility and explicit Fill authorization | `internal/app/workspace_continuation_api.go`, `internal/app/application_answer_transfer.go`, `internal/app/browser_session.go` |
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
- Route every newly captured control through the existing authoritative classifier (`classifyApplicationField`); do not duplicate field-role rules in React or another Go subsystem, and do not classify profile fields by value presence or provenance.
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
