# YourMove macOS candidate acceptance record — v0.1.0 build 3 (controlled alpha)

Supersedes the build 2 record (`ACCEPTANCE-v01-build2-e6bd465.md`). Delta since build 2: CI private-module auth (`.github/workflows/ci.yml`) and the version-string bump only; all behavior evidence below was gathered against identical application code, and `prepare.sh` re-ran both full Go suites against this exact frozen tree during packaging.

Candidate-specific evidence for source SHA `048d0a3`. All applicant/job data below is synthetic (isolated `YOURMOVE_DATA_DIR` workspaces). No private resumes, employer values, credentials, or cookies were used or committed.

## Exact artifact and environment

| Item | Value |
| --- | --- |
| Release/version/build | 0.1.0 / 3 |
| Source commit SHA | `048d0a3bb6371c237c99c707797563b68b5507b1` (branch `design/desktop-ux-rescue`) |
| Candidate binary ZIP SHA-256 | `d8d4baf30f65b233f2671459925d17d4b0f94a77e29521e42a54ccc90fdf2ad1` |
| Matching source ZIP SHA-256 | `48c597715826f6a902457ce0c71cf9849ab711c5e24dd284ce0d14d9addde731` |
| Candidate output dir | `~/Projects/yourmove-builds/mac-candidate-20261104-201712-32006/` |
| CI run and result | PASS — PR #16 (draft, CI-only). Run 41947478225 on the frozen push: `Go source verification: success`, `macOS desktop verification: success` (includes frontend build, both Go suites, desktop executable, `.app` bundle verification). Earlier run 41728747013 on the workflow-fix commit also success/success. The private-module fetch works via `CAREER_DISCOVERY_READ_TOKEN`. One non-blocking warning: pnpm/action-setup targets deprecated Node 20. |
| macOS / machine | macOS 26.5.2, Apple M1 Pro (arm64) |
| Chrome versions | Stable 156.0.8078.80 (`/Applications/Browsers/`); Canary 159.0.8140.0; no Firefox app; Safari 26.5 |
| Playwright cache | Pre-existing `~/Library/Caches/ms-playwright` (chromium-1200…1248, firefox-1509, webkit-2248); bridge deps installed via `scripts/setup-playwright-bridge.sh`; firefox binary downloaded via `npx playwright@1.58.2 install firefox` for qualification only |
| Tested installation path | Local ZIP extraction + signature check; owner reports completing clean installation November 5 (detailed browser-download/quarantine results not supplied) |
| Distribution audience | Controlled unnotarized technical alpha (no Developer ID identity exists on the build Mac) |
| Tester and date | Agent-assisted prep, 2026-11-04; owner sign-off pending |

## Installation and identity

- [x] Verified ZIP checksums (`shasum -a 256 -c SHA256SUMS` via `prepare.sh`), extracted locally: `YourMove.app`, bundle ID `com.yourmove.go`, version 0.1.0 build 3, `arm64` only, ad-hoc signature (`codesign --verify --deep --strict` exit 0, `Signature=adhoc`, no TeamIdentifier).
- [ ] **Owner reported clean installation completed November 5** and chose SHIP RESTRICTED. This record does not independently establish the browser-download/quarantine attribute, exact Gatekeeper message, or approval steps.
- [x] Confirmed local organization opens without Chrome or an AI connection (server starts; `/jobs` renders with empty workspace).
- [ ] Normal public installation (Developer ID, notarization, stapling, Gatekeeper) — NOT APPLICABLE: no signing identity exists; the 0.1.0 promise is a controlled alpha only.

## Browser matrix (UI surfaces, identical synthetic script)

Synthetic workspace: 1 opportunity ("Synthetic Support Engineer", `example.test`), 3 imported PDFs, 1 assigned resume, 1 manual cover letter, browser preference `firefox`.

- [x] Chrome stable 156 headless: `/jobs` (job listed), `/jobs/1` (workspace tabs render), `/resumes` (library renders), `/utilities` (enabled selector renders), `/career-evidence` (Go page renders). No console errors on any page.
- [x] Chrome Canary 159 headless: same markers, byte-identical renders to stable.
- [x] Wails desktop (`go run -tags desktop`, native window, dark mode): `/jobs` renders with the seeded job, assigned-resume state, and nav without Discovery/Studio. Screenshot `/tmp/wails-window.png` (repair workspace). No visual defects observed.
- [ ] Safari (WebDriver): NOT VERIFIED — `safaridriver` refuses sessions until the owner enables Safari Settings → Developer → "Allow remote automation" (or `sudo safaridriver --enable`). Rerun script: `/tmp/safari_matrix.py` (kept outside the repo) against port 4723.
- [ ] Narrow-window and keyboard walkthroughs in the alpha window: partially covered (1000px/760px breakpoints exercised via WebView snapshot during development); full pass deferred to the clean-account run.

## Persistent applicant data

- [x] Imported synthetic PDF resumes (3); assigned resume 1 to job 1; saved a manual cover letter (draft 1); set browser preference to `firefox`.
- [x] `kill -9` recovery: restarted on the same workspace — resume bytes identical (sha256 `0122bf67…` pre/post), DB document hashes match stored files, letter draft + selection survive, `firefox` preference survives, job association (`selectedResumeId: 1`) survives.
- [x] Export bytes: `/resumes/1/download` identical pre/post restart.
- [x] Prior-layout migration: unit test opens a synthetic pre-`settings` database; `initJobsSchema` adds the table, preserves the saved job, defaults to chromium.
- [ ] Native picker cancel: NOT VERIFIED (Wails-only dialog; clean-account human step).
- [ ] Version-upgrade check: NOT APPLICABLE for 0.1.0 (no prior release to upgrade from). Required before 0.1.1.

## Employer authority and core workflow

Engine qualification (`YOURMOVE_QUALIFY_BROWSER=1`, `qualification` tag, production frontend build):

- [x] `TestReactJourneyDocumentTransfer` on chromium: full document-transfer journey passes — exact resume bytes selected, zero employer submissions, submission recorded only through the explicit control, second opportunity reached.
- [x] Same journey on `YOURMOVE_QUAL_ENGINE=firefox`: passes (18.6s).
- [x] `TestReactQuestionsUnified*` updated for the renamed workspace tabs and re-passing.
- [x] Missing-binary drill: real session factory with unresolvable binary refuses with a clear "executable" error, leaves no session, preserves the saved job (`TestApplicationBrowserMissingBinaryFailsCleanly`).
- [x] Manual-value preservation and no-replay are asserted inside the qualified journeys (prepared answer transferred exactly once; uncertain items not replayed).
- [x] AI-absent behavior (live): generation without consent refused with explanation; unconfigured provider key returns "unavailable, prior drafts unchanged"; app remains fully usable.
- [ ] Live-ATS journeys: NOT DONE — qualification uses the controlled Ashby-shaped fixture, not live employer forms. Advertised scope is the manual path plus qualified fixture behavior; unsupported fields stay manual. This bounds the core-workflow gate to RESTRICTED (see dispositions).

## Browser selector decision (0.1.0)

- [x] ENABLED for this candidate. Backend was dead scaffolding (`AvailableEngines` had no callers; factory honored only env vars). This candidate adds: `settings` table, single-owner preference read/write/resolve (env override preserved), session-factory wiring, Utilities GET/POST with per-engine availability, honest WebKit exclusion (factory cannot launch it), and a fixed firefox cache-layout glob. Chromium availability follows the real system-binary resolution path, not the cache.
- [x] Qualification: both offered engines complete the controlled transfer journey; missing-binary and invalid-save negative paths tested; `/utilities` shows Firefox available and Chromium available on this Mac, each with reasons when unavailable.
- [ ] Clean-machine note: on a Mac without the Playwright cache, Firefox shows unavailable with reason; Chromium resolves via installed Google Chrome at the standard path. Verify during the clean-account run.

## Recoverability and optional providers

- [x] Browser interruption paths: covered by qualified journeys (close/reopen, second Fill opens a fresh session) and existing handler unit tests.
- [x] Missing Chrome: covered by the missing-binary test (clear error, no session residue, local work intact).
- [x] AI providers: consent + missing-key behavior verified live; no provider is required for the advertised journey.

## Gate dispositions (draft — owner signs in the release decision)

| Gate | Draft | Evidence / exclusion |
| --- | --- | --- |
| Frozen candidate | PASS | `048d0a3`, build 3, ZIP SHAs verified, ad-hoc signature verified; CI success/success on the frozen push (run 41947478225); `prepare.sh` ran both suites on the exact tree |
| Data integrity | PASS | kill -9 recovery identical; migration test; export bytes; upgrade N/A (first release) |
| Applicant authority/privacy | RESTRICTED | Qualified fixture journeys only; live ATS untested; no auto-submit asserted; manual fallbacks documented; synthetic data throughout |
| Core workflow | RESTRICTED | Chromium+Firefox fixture transfers pass; parked Discovery/Studio excluded; workspace null-questions crash found and fixed in this candidate |
| Distribution | RESTRICTED | Controlled alpha only; local extraction verified; downloaded-install + quarantine test pending on clean account; no Developer ID |
| Recovery | PASS (with noted gaps) | Missing-binary, kill -9, AI-absent verified; native picker cancel NOT VERIFIED |

- Known defects and excluded functionality: Career Discovery/Studio parked behind `careerDiscoveryEnabled`; WebKit not offered as an application browser; Safari surface not verified; native picker cancel not verified; live employer forms untested (manual path advertised); pnpm/action-setup Node 20 deprecation warning in CI (non-blocking).
- Release decision: **SHIP RESTRICTED**, owner-approved November 5, 2026, in `release/decisions/v0.1.0.md`.
- Decision record reference: `release/decisions/v0.1.0.md` (recorded approval, not publication).

## Owner follow-up and publication boundary (November 5, 2026)

1. Owner reports clean installation completed and has approved the restricted release; exact Gatekeeper prompt and any follow-on native acceptance outcomes were not supplied. The release instructions must continue to state unnotarized behavior without assuming a specific warning is universal.
2. Safari automation, native picker cancel, and live ATS behavior remain unverified and excluded from any broad compatibility claim. Before publishing, provide complete corresponding source for the private discovery module and confirm public artifact checksums.
