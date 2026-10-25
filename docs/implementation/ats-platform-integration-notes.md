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

## Posting-source onboarding (provisional; JI-015a)

This section concerns reading a posting from a public ATS source and preserving its content for YourMove's Overview. It is separate from the browser/application scope above. Keep the existing concrete `FetchJob` dispatch and provider-owned decoding in `internal/platforms`; this checklist is a review aid, not a universal provider interface or an authorization to add another ATS.

### Entry checklist for a posting source

For one provider and one advertised capability, record:

1. **User value and exact URL identity.** Write accepted saved URL examples and the fields that identify provider, board/tenant/region, and posting. State how a source response is associated with that exact saved posting. Test missing optional identifiers, conflicting identifiers, wrong boards/tenants, duplicate matches, malformed paths, lookalike hosts, credentials, ports, and encoded separators. Do not treat a board/tenant slug as a verified employer name.
2. **Trusted retrieval.** Derive one fixed HTTPS endpoint from validated URL components. Record whether it is public or authenticated, which credential (if any) is used, method, timeout, redirect policy, request context, and response-byte limit. Never fetch a URL supplied by a response field, an `applyUrl`, description link, redirect, or arbitrary JSON field.
3. **Private wire shape.** Decode only documented or observed fields into provider-private Go structs. Mark every field as documented, observed, constructed-test-only, or unknown. Keep absent, null, empty, unknown enum, malformed optional field, and malformed required identity distinct where useful. Optional unknown data should normally remain absent rather than fail an otherwise usable posting.
4. **Canonical projection and source fidelity.** Record the exact current canonical title/company/description/salary output and its precedence. Preserve original provider content only when a demonstrated reading need requires it; do not let HTML parsing, richer presentation, or facts silently change copy, exports, matching, generation evidence, or saved-question identity. Keep source HTML/plain alternatives distinct when both add fidelity.
5. **Closure and failure.** Record current status-code and “not found” behavior separately from evidence that an employer closed a posting. Timeout, malformed JSON, association mismatch, missing optional fields, and unsupported shapes are retrieval/availability outcomes, not proof of closure. Preserve existing stage policy until source evidence and a separately reviewed behavior change justify otherwise.
6. **Evidence and fallback.** Keep attributable minimal sanitized examples, fixture provenance/date/hash, exact expected canonical output and hashes, selected-source/envelope sizes, positive/negative identity cases, and exact `-count=1` commands. Separate documented behavior, live observations, deterministic synthetic tests, controlled-browser checks, and native Wails checks. Keep the existing manual URL/details/edit path usable when source retrieval is unavailable.
7. **Capability boundary.** Report posting retrieval, public application-question discovery, live read-only browser observation, and consent-based employer-form mutation as separate capabilities. A posting response does not prove application questions, current controls, attachment rules, applicant eligibility, or authority to fill or submit a form.

### Ashby public posting endpoint: qualified example

**Source:** [Ashby Job Postings API](https://developers.ashbyhq.com/docs/public-job-posting-api). The documentation describes a public `GET` to `https://api.ashbyhq.com/posting-api/job-board/{JOB_BOARD_NAME}` and the optional `includeCompensation=true` query; its request example uses no credentials. It documents `apiVersion`, `jobs[]`, title, location, workplace/employment fields, HTML/plain descriptions, publication/listing fields, job/apply URLs, and compensation. It states that `isListed=false` means a posting should be available only by direct link. The documented workplace values are `OnSite`, `Remote`, and `Hybrid`; `isRemote=true` can mean either Remote or Hybrid. Employment values include FullTime, PartTime, Intern, Contract, and Temporary. A documented primary address is nested under `address.postalAddress`.

Current code constructs `https://api.ashbyhq.com/posting-api/job-board/{url.PathEscape(board)}?includeCompensation=true` after parsing the saved board/posting URL. The client is a ten-second, no-redirect client; board responses are bounded at 32 MiB. These are current Ashby implementation limits, not conventions to impose on other providers.

**Evidence status (2026-10-24):** The official documentation was re-read for this milestone. The dated Ashby and Linear board observations recorded in [the implementation plan](job-intelligence/IMPLEMENTATION_PLAN.md#b3-ashby-positive-source-contract-evidence-recorded-in-this-revision) are prior read-only observations, not immutable fixtures: Ashby was 2,199,625 bytes / 67 jobs (SHA-256 `6637679dd94b240a5a33e6b7d0e32739e15d6fc5cedda5fdfd0f608bea506e79`); Linear was 459,262 bytes / 34 jobs (SHA-256 `d55cac637bd75346f10cf7e58f728c18e5579527387f1e67c87a751d74e7c1e0`). Those observations found per-job compensation and both nonempty description alternatives in all sampled jobs; both Remote and Hybrid were observed. The raw responses were temporary research data and are not retained. Direct board GETs were **NOT VERIFIED in this lane**; shell DNS access to `api.ashbyhq.com` was unavailable.

| Contract point | Evidence and current handling | Qualification limit |
|---|---|---|
| Saved URL | Current parser accepts HTTPS `jobs.ashbyhq.com/{board}/{UUID}` with no user info or explicit port and a safe board token (`internal/platforms/ashby.go`). | Official docs identify the board name but their sample `jobUrl` does not establish this exact path shape. The board/UUID path was observed in the dated boards; aliases, case equivalence, and direct-link-only examples remain NOT VERIFIED. Intake removes a fragment and retains the query; neither establishes board alias equivalence. |
| Selected posting | Current `extractAshbyMetadata` selects a matching `id`, or falls back to a `jobUrl` whose host and two path segments end in the requested UUID. It does not receive the requested board. An ID match bypasses any contradictory `jobUrl`; fallback does not validate scheme, port, credentials, or returned board; first match wins. | The docs field table does not list `jobs[].id`; all 101 sampled jobs had an `id`. Missing-ID plus exact matching `jobUrl` is a required constructed positive, not a live observation. Wrong-board same-ID, contradictory ID/URL, and duplicate match are synthetic negative cases that the M1 identity check must reject. Do not mistake current selection behavior for a qualified trust rule. |
| Destination | Current posting extraction does not decode `applyUrl`. | The docs example shows `/apply`; the dated sample observations recorded `/application`. Neither shape grants browser ownership. Validate any destination independently against the exact saved posting and existing application-browser ownership rules. |
| Descriptions | Current mapping trims `descriptionPlain` and uses it as canonical when nonempty; otherwise it converts `descriptionHtml` through `descriptionText`. That helper entity-unescapes up to twice, drops script/style/noscript, flattens structure and links, and normalizes whitespace. | Both alternatives were nonempty in all dated samples. HTML-only, plain-only, empty, malformed, and escaped-content fixtures are synthetic robustness cases. Preserve any future rich source without changing this plain-first canonical behavior. |
| Compensation | Current private wire type reads `jobs[].compensation.scrapeableCompensationSalarySummary`; `completeMetadata` prefers that structured summary and otherwise applies the existing description salary extractor (`internal/platforms/compensation.go`). | The docs field table places compensation under `jobs[]`, while the illustrative JSON prints it at the root. The dated board responses had per-job compensation and no root compensation. Do not broaden the decoder from the example alone or replace the existing salary owner. |
| Direct facts | The docs define `jobs[].location`, `workplaceType`, `isRemote`, and `employmentType`; dated samples had these fields, including Remote and Hybrid. | Current Ashby wire mapping does not retain them. OnSite/non-FullTime values are documented, not observed in these samples. `isRemote=true` alone is not evidence of fully remote work or geographic eligibility. Secondary-location/address variants need separate qualification. |
| Listed/published state | Docs say `isListed=false` indicates direct-link-only availability and define `publishedAt` as last publication. | Neither is a retrieval timestamp; `isListed=false` is not closure. No direct-link-only response was observed. |
| Errors | Current API client uses a 10-second timeout, rejects redirects, and caps an individual response at 4 MiB; Ashby board retrieval passes a 32 MiB limit (`internal/platforms/api.go`, `internal/platforms/ashby.go`). HTTP 404/410 map to `ErrClosed`; a missing selected job also currently returns `ErrClosed`. | The board endpoint's not-found/direct-link-only/closed semantics were not qualified. Do not claim that a mapper miss independently proves closure. Malformed or contradictory identity must not be treated as closure evidence. |

The current Ashby decoder's private wire shape is limited to `id`, `title`, `descriptionPlain`, `descriptionHtml`, `jobUrl`, and nested compensation salary summary. It ignores other documented fields. Treat retained HTML as untrusted source data: never serve it directly as template HTML, execute embedded content, or fetch resources referenced by it. The exact current canonical output and salary precedence belong in the provider characterization tests and sanitized fixture, not in a second copy of test data here. No actual Ashby application page is needed to qualify this posting endpoint.

### Separate application capabilities

| Capability | Owner and evidence | Does not establish |
|---|---|---|
| Posting retrieval and job reading | `internal/platforms`: validated saved URL, fixed endpoint, bounded request, private wire decode, selected posting mapping, canonical/source behavior. | Public questions, live page state, candidate fit, or form-transfer authority. |
| Public application-schema discovery | Existing inspector/provider path under `internal/application/providers` and discovery. Qualify its endpoint and output independently. | That an advertised question is visible, required, or still present in the live form. Posting metadata is not a question schema. |
| Live browser observation | `internal/application/browser`: read-only, value-free observation under the exact existing posting ownership rules. | Values, later/conditional steps, hidden/cross-origin controls, server acceptance, or posting-source authority. |
| Employer-form mutation | Existing app/chromium operations after explicit user authorization and exact destination/scope checks. Resume/answer/letter transfers retain their independent guards and outcomes. Final employer submission remains a human action. | Any broader authority from a posting ID, `applyUrl`, metadata, or a public question response. |

### Provider-specific entry status — M3 update (2026-10-24)

Ashby remains the first posting-source example above. Its positive and negative source matrix is qualified only to the cited documentation, dated board observations, and retained characterization fixture. The M0 fixture is [synthetic](../../internal/platforms/testdata/job-intelligence/ashby-long-synthetic.json) (SHA-256 `008818ef01d7578e6f5b96a8c2ee0907d24ac63273d256bc0299019ee0842b0c`); its [characterization tests](../../internal/platforms/ashby_m0_characterization_test.go) pin the canonical output/hash and current salary projection.

Greenhouse and Lever now implement the posting-source path through `FetchJob`, the existing intake transaction, SQLite source persistence, and the same Overview. The shared source envelope remains small: provider/board/posting identity and a `region` only for Lever's global/EU identity. It does not carry application destinations, question inventory, or browser authority.

| Provider | Qualified wire and identity boundary | Canonical/source behavior | Evidence and limit |
|---|---|---|---|
| Greenhouse | The official [Job Board API](https://docs.greenhouse.io/job-board) documents public GET posting endpoints, numeric posting `id`, nested `location.name`, and `content` encoded as HTML entities. The adapter constructs `boards-api.greenhouse.io/v1/boards/{board}/jobs/{id}`, requires the returned numeric ID to match, decodes the retained source content twice, and maps only `location.name`. | Existing `descriptionText(content)` remains the canonical projection. Salary is not inferred from description text in this adapter. Malformed optional location/content is treated as absent; unknown fields are ignored. | Current hosted Figma page was read without opening its application form: [public posting](https://job-boards.greenhouse.io/figma/jobs/6180053004). Adapter tests use the explicitly synthetic [fixture](../../internal/platforms/testdata/job-intelligence/greenhouse-encoded-posting-synthetic.json) and notes. The raw API JSON response was not accessible to the available fetch tool and remains **NOT VERIFIED**. |
| Lever | The official [Postings API](https://github.com/lever/postings-api/blob/master/README.md) documents global and EU instances, `GET /v0/postings/SITE/POSTING-ID`, unique `id`, combined and component descriptions, named lists, closing content, categories, workplace type, and optional salary range. The adapter fixes the API host from the validated saved URL and binds provider, region, site, and response ID. | Existing canonical precedence is unchanged. Rich source uses a combined description once when present, appends ordered named lists and closing content, and uses `descriptionBodyPlain` only for rich reading when body HTML is missing. Source inputs and envelope are bounded at 1 MiB; the existing retrieval body limit remains 4 MiB. | Current EU hosted [posting page](https://jobs.eu.lever.co/lever/065f4538-7347-4207-909f-4ea68f63b4af) was read without opening its application form. The ordered fixture is explicitly [synthetic](../../internal/platforms/testdata/job-intelligence/lever-ordered-synthetic.json), with field provenance and limits in its notes. Direct live API JSON remains **NOT VERIFIED**. |

**Working-tree evidence:** `go test -count=1 ./internal/platforms` and `TestJobIntelligenceM3ProvidersUseIntakeOverviewAndPersistAcrossRestart` pass. The opt-in `TestJobIntelligenceM3ProviderOverviewDemo` passed in pinned Chrome Canary 158.0.8122.0 for both providers at 1280×900 and 390×900 in light/dark themes. It checks canonical copy, direct facts, Lever priority navigation and zero application submission. Raster captures are retained in the local review archive and excluded from the source PR. The lifecycle test also verifies manual edit precedence and source retention after SQLite restart. The controlled path uses only synthetic provider transport. Native Wails acceptance is **NOT VERIFIED** and remains JI-024c; raw live API JSON remains **NOT VERIFIED**. These are posting-reading capabilities only: Greenhouse public questions, live application observation, employer form mutation, and submission are separate capabilities. Do not generalize an Ashby endpoint, wire type, salary field, or identity rule to another provider.
