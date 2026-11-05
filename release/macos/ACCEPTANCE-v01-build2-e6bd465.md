# YourMove macOS candidate acceptance record — v0.1.0 build 2 (controlled alpha)

> SUPERSEDED by `ACCEPTANCE-v01-build3-048d0a3.md` (CI fix + rebuild). Retained as history.

Candidate-specific evidence for source SHA `e6bd465`. All applicant/job data below is synthetic (isolated `YOURMOVE_DATA_DIR` workspaces). No private resumes, employer values, credentials, or cookies were used or committed.

## Exact artifact and environment

| Item | Value |
| --- | --- |
| Release/version/build | 0.1.0 / 2 |
| Source commit SHA | `e6bd46507cac82fa7a6aa089a5247b88f3b5c488` (branch `design/desktop-ux-rescue`) |
| Candidate binary ZIP SHA-256 | `0a77e1ed3389e49b666ec6e91d7c32e22ebebf6f7c4f97a8fda812ec87128d95` |
| Matching source ZIP SHA-256 | `2a8e6c00326128295f7393ccc9221544c373b0b79166b2b32646bec20d4d416e` |
| Candidate output dir | `~/Projects/yourmove-builds/mac-candidate-20261103-154547-72831/` |
| CI run and result | BLOCKED — PR #16 (draft, CI-only), run 41726853009: both jobs fail identically on first run and rerun. Root cause is pre-existing and unrelated to the candidate diff: `go.mod` requires private repo `github.com/jenksed/yourmove-career-discovery` (added to this line Oct 30, not present in `main`), and CI runners have no auth for it (`git ls-remote` exit 128). Local full suites (both tags) pass on the exact frozen tree, including inside `prepare.sh`. Fix requires owner decision (CI token vs vendor vs public dep) + refreeze + rebuild. |
| macOS / machine | macOS 26.5.2, Apple M1 Pro (arm64) |
| Chrome versions | Stable 156.0.8078.80 (`/Applications/Browsers/`); Canary 159.0.8140.0; no Firefox app; Safari 26.5 |
| Playwright cache | Pre-existing `~/Library/Caches/ms-playwright` (chromium-1200…1248, firefox-1509, webkit-2248); bridge deps installed via `scripts/setup-playwright-bridge.sh`; firefox binary downloaded via `npx playwright@1.58.2 install firefox` for qualification only |
| Tested installation path | Local ZIP extraction + signature check (this record); browser-downloaded install on clean account: PENDING owner run |
| Distribution audience | Controlled unnotarized technical alpha (no Developer ID identity exists on the build Mac) |
| Tester and date | Agent-assisted prep, 2026-11-03; owner sign-off pending |

## Installation and identity

- [x] Verified ZIP checksums (`shasum -a 256 -c SHA256SUMS` via `prepare.sh`), extracted locally: `YourMove.app`, bundle ID `com.yourmove.go`, version 0.1.0 build 2, `arm64` only, ad-hoc signature (`codesign --verify --deep --strict` exit 0, `Signature=adhoc`, no TeamIdentifier).
- [ ] Exercise an actual **browser-downloaded** ZIP and the normal macOS quarantine path on the clean account — PENDING (owner action, exact steps in "Still required from the owner").
- [x] Confirmed local organization opens without Chrome or an AI connection (server starts; `/jobs` renders with empty workspace).
- [ ] Normal public installation (Developer ID, notarization, stapling, Gatekeeper) — NOT APPLICABLE: no signing identity exists; the 0.1.0 promise is a controlled alpha only.

## Browser matrix (UI surfaces, identical synthetic script)

Synthetic workspace: 1 opportunity ("Synthetic Support Engineer", `example.test`), 3 imported PDFs, 1 assigned resume, 1 manual cover letter, browser preference `firefox`.

- [x] Chrome stable 156 headless: `/jobs` (job listed), `/jobs/1` (workspace tabs render), `/resumes` (library renders), `/utilities` (enabled selector renders), `/career-evidence` (Go page renders). No console errors on any page.
- [x] Chrome Canary 159 headless: same four markers, byte-identical renders to stable.
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
| Frozen candidate | BLOCKED (CI) | `e6bd465`, build 2, ZIP SHAs verified, ad-hoc signature verified; GitHub CI cannot fetch the private `yourmove-career-discovery` module — pre-existing branch-line condition, fix + refreeze required |
| Data integrity | PASS | kill -9 recovery identical; migration test; export bytes; upgrade N/A (first release) |
| Applicant authority/privacy | RESTRICTED | Qualified fixture journeys only; live ATS untested; no auto-submit asserted; manual fallbacks documented; synthetic data throughout |
| Core workflow | RESTRICTED | Chromium+Firefox fixture transfers pass; parked Discovery/Studio excluded; workspace null-questions crash found and fixed in this candidate |
| Distribution | RESTRICTED | Controlled alpha only; local extraction verified; downloaded-install + quarantine test pending on clean account; no Developer ID |
| Recovery | PASS (with noted gaps) | Missing-binary, kill -9, AI-absent verified; native picker cancel NOT VERIFIED |

- Known defects and excluded functionality: Career Discovery/Studio parked behind `careerDiscoveryEnabled`; WebKit not offered as an application browser; Safari surface not verified; native picker cancel not verified; live employer forms untested (manual path advertised).
- Release decision: PENDING owner SHIP RESTRICTED / HOLD in `release/decisions/v0.1.0.md`.
- Decision record reference: PENDING.

## Still required from the owner (exact steps)

1. Push `design/desktop-ux-rescue` (or authorize the push) so CI runs against `e6bd465`; record the run URL and result in the table above. Gate 1 stays pending until then.
2. On the clean account: download the binary ZIP through a browser (do not copy it), confirm the quarantine attribute (`xattr -p com.apple.quarantine <file>`), extract via Finder, open, and record the exact Gatekeeper warning and first-launch behavior. Then: import a synthetic PDF, save an opportunity, assign the resume, cancel one native import, quit, relaunch, and confirm everything persists. Paste the outputs back for the record.
3. Optional: enable Safari remote automation and run `python3 /tmp/safari_matrix.py` (server on :3000, `safaridriver -p 4723`) to close the Safari gap; otherwise it ships as a documented exclusion.
