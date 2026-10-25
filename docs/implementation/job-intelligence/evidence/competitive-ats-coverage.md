# Competitive ATS Job Intelligence coverage

Status date: 2026-10-24. This is a follow-on provider-expansion record. It does not replace the accepted Ashby M1 record or the separate M2/M3 native-acceptance gates.

## Source preservation and evidence

- Starting branch/HEAD: `feature/ashby-provider-finalization` / `5da7ff214ae1e9ace05d47eed9183d37f7a8c754`.
- Starting source archive: `/private/tmp/yourmove-go-competitive-start-20261024/source.tar`, SHA-256 `5ebc06935cc67adf3f9b6041491cda37646157d43078ee400dd868ed72fa4d19`.
- Starting per-file manifest: `/private/tmp/yourmove-go-competitive-start-20261024/source-manifest.sha256`, SHA-256 `43816e8eb845e9eabfaaad8bca3207343d98f36feb0d617b7983812428b5ebb5`.
- The archive includes the actual dirty working tree, including untracked Job Intelligence implementation/evidence and the pre-existing `AGENTS.md` modification. `AGENTS.md` was not edited. No Git history or working-tree cleanup operation was performed.
- Read-only live response bodies and hashes are under `/private/tmp/yourmove-go-competitive-start-20261024/live-sources/`; they are not copied into the repository. Requests used fixed public HTTPS destinations, no credentials, no redirects, 8-second timeouts, and a 4 MiB read cap.

## Frozen shared source contract

The four implementation lanes use the existing concrete `Metadata` and `PostingSource` contracts. The envelope format remains version 1; its provider allowlist, `FetchJob` dispatch, and `PostingIdentity.ValidateURL` now recognize Workday, SmartRecruiters, Workable, and Jobvite. No source schema fields, storage tables, migrations, app abstractions, browser ownership rules, application-question behavior, or UI components are added.

- `Metadata.Description` remains the canonical writing/copy input. Its value is derived deterministically from the selected provider description before sealing; the rendered `jobdocument.Document` never replaces it.
- Rich source content and direct facts are sealed with `SealPostingSource` against the exact saved posting URL and canonical description. Existing 1 MiB source and JSON limits apply. Oversize or invalid source is unavailable; it is never truncated and marked successful.
- Identity uses the existing `Provider`, `Board`, `Region`, and `PostingID` fields. Each provider parser validates HTTPS, host, path, and identity-bearing query values before constructing a fixed HTTPS request. Returned posting/company identity must reconcile with the requested record wherever the public response provides it.
- Retrieval is anonymous, context-bound, time-bounded, response-bounded, and refuses redirects. No returned `externalUrl`, `applyUrl`, posting link, or employer-controlled value is fetched. A public-posting absence is not closure evidence unless the provider response/status establishes closure under the existing policy.
- The orchestrator owns `FetchJob`, `PostingIdentity.ValidateURL`, provider allowlisting, and any shared HTTP changes. Provider lanes own only their new `internal/platforms/<provider>.go`, matching tests, and provider-specific synthetic fixtures. The app integration and cross-provider lifecycle/Chromium tests remain centrally owned.
- No Workday listing/search fallback is used for a saved posting. The public listing endpoint was characterized as POST; the production adapter retrieves the exact posting by the validated detail URL using GET.

### Provider identity mapping

| Provider | `Board` | `Region` | `PostingID` |
|---|---|---|---|
| Workday | Exact Workday hostname plus tenant/site path identity | URL locale when present | Full validated path below `/job/`, including location and posting slug |
| SmartRecruiters | Company identifier | Empty | Numeric posting ID before an optional title suffix |
| Workable | Account slug | Empty | Exact public shortcode |
| Jobvite | Career-site slug | Empty | Exact opaque job ID |

## Integrated URL variants and production path

| Provider | Implemented posting URL variant | Retrieval target |
|---|---|---|
| Ashby | `https://jobs.ashbyhq.com/{board}/{postingUUID}` (full UUID required) | Anonymous public posting API detail GET for the exact board and UUID. |
| Greenhouse | `https://boards.greenhouse.io/{board}/jobs/{numericId}` and `https://job-boards.greenhouse.io/{board}/jobs/{numericId}` | Anonymous public job-board API detail GET; returned ID must match. |
| Lever | `https://jobs.lever.co/{site}/{postingId}` and `https://jobs.eu.lever.co/{site}/{postingId}` | Anonymous public postings API detail GET, with global/EU region identity preserved. |
| Workday | `https://{tenant}.wd{pod}.myworkdayjobs.com/{locale?}/{site}/job/{location}/{slug}`; also `https://wd{pod}.myworkdaysite.com/{locale?}/recruiting/{tenant}/{site}/job/{location}/{slug}` | Exact posting CXS detail GET at the validated hostname/tenant/site/path; locale is identity metadata when present. |
| SmartRecruiters | `https://jobs.smartrecruiters.com/{company}/{numericId}` or `/{numericId}-{slug}` | Anonymous public Posting API detail GET for the parsed company and numeric ID. |
| Workable | `https://apply.workable.com/{account}/j/{shortcode}` | Fixed anonymous widget-account GET, then exact shortcode selection. |
| Jobvite | `https://jobs.jobvite.com/{careerSite}/job/{jobId}` | Anonymous GET of the exact hosted job detail page; JSON-LD identity must match. |

All seven supported provider entries above are integrated through saved job URL intake, identity validation, source retrieval and persistence, Overview reading, manual edit precedence, refresh, and database restart. The four adapters added in this wave use the same source envelope, canonical description path, storage, reading document, Overview, refresh handler, and copy control as Ashby, Greenhouse, and Lever.

### Cross-provider controlled portfolio

`TestJobIntelligenceCompetitiveProvidersUseProductionLifecycle` exercises each new provider from form intake through identity and facts persistence, Overview rendering, manual edits, refresh, and SQLite restart. `TestJobIntelligenceCompetitiveProviderOverviewDemo` runs controlled Chromium against one synthetic workspace containing all seven currently implemented providers: Ashby, Greenhouse, Lever EU, Workday, SmartRecruiters, Workable, and Jobvite. For every provider, it checks identity and provider-specific rich content in the common Overview, captures 1280px and 390px screenshots, submits a manual description edit through the browser, refreshes from a changed provider fixture, verifies the manual edit remains primary while refreshed employer source and freshness are visible, and checks canonical copy bytes. It then closes and reopens SQLite, opens each Overview in Chromium again, and verifies the manual description, refreshed source, freshness, and copy text persisted. No employer application is opened and no submission is recorded.

Controlled Chromium evidence (light theme; generated from synthetic provider fixtures):

| Provider | Desktop | Narrow |
|---|---|---|
| Ashby | Captured locally | Captured locally |
| Greenhouse | Captured locally | Captured locally |
| Lever EU | Captured locally | Captured locally |
| Workday | Captured locally | Captured locally |
| SmartRecruiters | Captured locally | Captured locally |
| Workable | Captured locally | Captured locally |
| Jobvite | Captured locally | Captured locally |

Raster captures and their SHA-256 manifest are retained in the local review archive and excluded from the source PR. The controlled browser run used `YOURMOVE_QUALIFY_COMPETITIVE=1`, `YOURMOVE_COMPETITIVE_EVIDENCE_DIR=<absolute evidence directory>`, and `go test -tags=qualification -count=1 ./internal/app -run '^TestJobIntelligenceCompetitiveProviderOverviewDemo$'`.

The mapping is association evidence only. It does not create application or browser-transfer authority.

## Seven-provider source dossiers

| Provider | Evidence and supported URL scope | Source disposition before adapter integration |
|---|---|---|
| Workday | `myworkdayjobs.com/{locale?}/{site}/job/{location}/{slug}` and `wdN.myworkdaysite.com/{locale?}/recruiting/{tenant}/{site}/job/{location}/{slug}`. Anonymous CXS listing POST and exact detail GET both returned JSON; the selected record's title and `jobPostingId` matched the requested external path. Listing is not used as intake fallback. | **READY / source-qualified** for these two hosted families. Other host families and internal/authenticated postings are unsupported. |
| SmartRecruiters | `jobs.smartrecruiters.com/{company}/{numericId}` with optional `-{slug}`. Anonymous Posting API detail response returned matching company and posting IDs and four ordered job-ad sections. | **READY / source-qualified** for this URL/API shape. Custom domains, internal postings, UUID URL forms, and credentialed APIs are unsupported. |
| iCIMS | Classic `/jobs/{numericId}/{slug}/job` examples were inspected, but bounded raw reads returned employer career landing pages without the requested job content or a stable posting payload. Modern/custom-domain variants were not validated. | **BLOCKED / research only** until a repeatable applicant-facing detail response and content boundary are demonstrated. No authenticated API or custom-domain scraper. |
| Taleo | Oracle documents explicit `careersection/<section>/jobdetail.ftl?job=...` identity, but multiple public samples returned only career-site shells or an unavailable section. Actual detail markup/encoding was not verified. | **BLOCKED / research only**. Do not implement from URL documentation or third-party snippets alone. |
| Avature | Hosted `*.avature.net/.../careers/JobDetail/.../{id}` routes were found; the tested live route redirected to a branded error route, and the raw job detail structure was not available. | **BLOCKED / research only** for now. No universal branded-domain or custom-domain scraper. |
| Workable | `apply.workable.com/{account}/j/{shortcode}`. The documented public endpoint redirected to the Workable widget route; a direct anonymous widget response returned the requested shortcode, title, employer, and description. | **READY / source-qualified** for the hosted widget-backed variant. Customer SPI APIs and custom domains are unsupported. |
| Jobvite | `jobs.jobvite.com/{careerSite}/job/{jobId}`. A live anonymous detail page returned `JobPosting` JSON-LD whose `identifier` matched the URL job ID and whose description contained the posting content. | **READY / source-qualified** for the hosted JSON-LD variant. Paid feeds, customer APIs, embedded custom domains, and application routes are unsupported. |

### Live source observations

| Provider/sample | Result and bounded response evidence |
|---|---|
| Workday `myworkdayjobs.com` | Anonymous listing POST: HTTP 200, 17,051 bytes, SHA-256 `a659fae78436d3246249892a65559bc977ecee9faadc8eb433cccde23c9af63d`; selected detail GET: HTTP 200, 5,290 bytes, SHA-256 `a7e76d4e4739b93ddc45e005708a7e5f54b0d8d7a5fa6e1273b6fcdf3956387c`. |
| Workday `myworkdaysite.com` | Anonymous exact-title listing POST: HTTP 200, 2,358 bytes, SHA-256 `d88230f0e650569e7abd67255c27eb5740ff9cf16e306bd98b8dfe2732746ae4`; selected detail GET: HTTP 200, 14,532 bytes, SHA-256 `dbc7ad6f9cfcc575969ccb12738a6bfbdc45e6cfd8dc8181c3f315ff5ee079b2`. |
| SmartRecruiters | Anonymous Posting API GET: HTTP 200, 8,619 bytes, SHA-256 `3491af5b21e02780fcc8624f50f97fb4b2cb99dacdf644214c7cf19ead6fe559`; company `Sportradar`, posting ID `744000139295439`. |
| Workable | Direct anonymous widget GET: HTTP 200, 37,522 bytes, SHA-256 `38a31b0d6911dc73d87b84dea53bf893922a5c97d1ba0367405de3f70e251caa`; account `Hugging Face`, six public jobs, requested shortcode `81B46579FE` present with a 5,431-character description. |
| Jobvite | Anonymous detail HTML: HTTP 200, 80,609 bytes, SHA-256 `759ffa1f00c498ae78e120793748a217e3404ef370016e9de00bf3f1dd6cf5f7`; JSON-LD `JobPosting` ID `oVqQAfwv`, description 29,433 characters. |
| iCIMS / Taleo / Avature | iCIMS tested responses were HTTP 200 but lacked requested posting content; Taleo returned career-site shell HTML; Avature redirected to `careers.mantech.com/.../Error`. These do not qualify source support. |

## Qualification ledger

This ledger is completed after adapter integration. A live source observation is Level 1 evidence only; it is not an adapter, controlled app, or native acceptance.

| Provider | Source-qualified (L1) | Adapter-qualified (L2) | Controlled-supported (L3) | Native-accepted (L4) | Restrictions |
|---|---|---|---|---|---|
| Ashby | Existing M1 evidence | Existing M1 evidence | Existing M1 evidence | Existing maintainer-reported M1 acceptance; this wave does not reopen or retest it | Does not imply application autofill or submission |
| Greenhouse | Existing M3 evidence | Existing M3 evidence | Existing controlled M3 evidence | Pending M3 native acceptance | Public posting retrieval only |
| Lever | Existing M3 evidence | Existing M3 evidence | Existing controlled M3 evidence | Pending M3 native acceptance | Public posting retrieval only |
| Workday | PASS | PASS | PASS | Pending | Two hosted Workday URL families only |
| SmartRecruiters | PASS | PASS | PASS | Pending | Public Posting API only |
| iCIMS | BLOCKED | — | — | — | No repeatable public detail payload verified |
| Taleo | BLOCKED | — | — | — | Public page returned shell; detail content unverified |
| Avature | BLOCKED | — | — | — | Tested hosted route redirected to error; custom domains unsupported |
| Workable | PASS | PASS | PASS | Pending | Public widget route only; account response capped at 4 MiB |
| Jobvite | PASS | PASS | PASS | Pending | Hosted JobPosting JSON-LD only |

Wave counts: **7 providers investigated; 4 of 7 source-qualified; 4 of 7 implemented; 4 of 7 controlled-qualified.** Three investigated providers (iCIMS, Taleo, Avature) remain research-only and are not counted as supported.

Across the full 10-provider portfolio: **7 of 10 are controlled-supported** (Ashby, Greenhouse, Lever, Workday, SmartRecruiters, Workable, Jobvite); **1 of 10 is native-accepted** based on existing Ashby M1 evidence. For the eight explicitly named benchmark platforms, **5 of 8 are controlled-supported** and **1 of 8 is native-accepted**. Greenhouse/Lever M3 native acceptance remains pending; the four new adapters have not received native Wails acceptance.

Research-only and fixture-only providers do not count as supported. Level 3 is controlled Chromium evidence for intake, reading, manual precedence, refresh, freshness, canonical copy, and restart persistence; it does not replace native Wails acceptance. The exact local macOS candidate for this source is recorded below; this wave does not claim a Level 4 result.

## Independent review and dispositions

- **Workable metadata storage bound — fixed.** Review found that the public widget's selected title and account name could exceed the posting-source envelope limit and be stored directly on the job. The adapter now rejects normalized title and company values above 512 bytes; regression tests cover both fields. Provider tests pass.
- **Level 3 workflow scope — closed.** Review found the first controlled browser demonstration split refresh/manual/persistence coverage into handler-only tests. The Chromium portfolio now drives edit, refresh, canonical copy, and SQLite restart verification for every implemented provider. The final controlled run passes.
- Review found no remaining material identity, redirect/SSRF, source-binding, storage, or browser-authority issue. iCIMS, Taleo, and Avature remain research-only and outside supported counts.

### Local native candidate handoff

| Item | Identity |
|---|---|
| Source branch / HEAD | `feature/ashby-provider-finalization` / `5da7ff214ae1e9ace05d47eed9183d37f7a8c754` |
| Candidate app | `/private/tmp/yourmove-go-competitive-candidate-final-20261024/YourMove.app` |
| Candidate source manifest SHA-256 | `33941116221ec46c2cdd79e77d3a9486c9d67ce6be7c504110a109cf34e43dcf` |
| Executable SHA-256 | `a9a23b331d2e38223adc6707a946e684ae51936d1d3ad1c078a70b2e05e65d55` |
| Candidate build | `darwin/arm64`; macOS minimum 26.0, SDK 26.5; ad-hoc signed and locally verified |
| Acceptance | **NOT VERIFIED** — native Wails interaction is pending; this local ad-hoc build is not a release artifact. This evidence document was finalized after the build; no product source or test files changed afterward. |

## Not in scope

This expansion covers public job posting intake, source reading, refresh, persistence, and canonical copy only. It does not qualify or expand employer application form discovery, browser authority, autofill, or submission. M2/M3 native acceptance and deferred YM-017 VoiceOver work remain separate gates.
