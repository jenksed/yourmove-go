# YourMove Go v0.2.0 — Cross-Platform Acceptance & Testing Specification

**Status:** B / Working draft for review — **no new platform qualification has been performed**  
**Date:** November 5, 2026  
**Audience:** Product owner, engineering, release owner, platform testers  
**Product contract:** The **approved** v0.2.0 Product Experience Specification (A). This document defines how we determine whether those experiences actually work.  
**Repository snapshot checked:** `jenksed/yourmove-go` `main` at `fd835f3258cfb7ca928f27cd5ae16058678219a7`. Successful CI run `42100935870` checks existing Ubuntu source and macOS desktop jobs; it does **not** qualify Windows or Linux native hosts.

> **Acceptance principle:** A build is not a platform release. A passing fixture is not universal employer compatibility. Shipping is an artifact-specific human decision supported by the **installed application**, preserved user work, and observed applicant-authorized behavior.

## 1. Decision this specification is intended to support

**Question:** Which platform/artifact combinations can a job seeker install, use for a *second* application, and safely update — and what specific capabilities are supported on each?

The approved product specification establishes two equal directions: a materially easier second application, and native delivery beyond Apple Silicon macOS. It also permits **independent, platform-specific qualification** rather than making every OS release an all-or-nothing dependency. This document does not reopen the product principles or silently change the approved scope.

**In-scope target platform matrix**

| Target | OS / architecture | Proposed distribution | Intended outcome | Baseline status |
| --- | --- | --- | --- | --- |
| M1 | macOS 26+ / arm64 | Existing `.dmg` pathway, revised for v0.2 | Preserve v0.1.0 journey, qualify new reuse and real upgrade | Prior restricted technical alpha; requalification required |
| W1 | Windows 11 / amd64 (x64) | One native installer (`.exe`, likely NSIS) | Installed Windows app + controlled employer handoff | No native release/acceptance |
| L1 | Ubuntu 24.04 / amd64 (x64) | One `.deb` | Installed Ubuntu app + controlled employer handoff | No native release/acceptance |

Windows ARM64, Linux ARM64, older/Intel macOS, other Linux distributions, other browser engines, and universal live-ATS support are **not** included in v0.2.0 qualification. A passing build for one of them does not add it to the advertised matrix.

**Terminology for external claims**

1. **Not evaluated:** no tested artifact or native-path evidence.
2. **Build-only:** code compiled or packaging generated; not a usable-user claim.
3. **Technical preview:** explicitly limited audience and features; actual installation and local workflow have been observed. Missing employer-assistance qualification must be stated.
4. **Qualified beta for an OS:** exact downloaded artifact passed the native user journeys, application-authority tests, restart/upgrade preservation, and distribution gates defined below, with any excluded ATS/AI capabilities documented.
5. **Release approved:** owner records `SHIP`, `SHIP RESTRICTED`, or `HOLD` for specific artifacts and audiences under the repository's existing release policy. An automated job cannot approve shipment.

## 2. What exists today; what remains unproven

**Verified source observations at the snapshot:**

- `desktop.go` has `//go:build desktop && darwin && !qualification`; the native Wails entry point is macOS-only. `main.go` remains a non-desktop local HTTP host.
- `.github/workflows/ci.yml` runs Go source verification on Ubuntu and native desktop compilation/packaging verification on macOS. There is **no** Windows/Linux native desktop acceptance job.
- `internal/application/browser/chromium/executable.go` searches Unix-style command names and Mac paths; `isExecutableFile` checks Unix execute bits. Windows browser discovery must be implemented and exercised rather than assumed to work.
- `internal/storage/paths.go` uses per-OS application storage via `os.UserConfigDir`, except a macOS-specific Application Support location; this is a useful starting design, **not proof** of Windows/Linux file or upgrade behavior.
- `go.mod` pins `github.com/wailsapp/wails/v3 v3.0.0-beta.32` and Go `1.27.2`. Current Wails guidance describes later stable releases; any dependency change is a **separate compatibility decision**, not automatic scope expansion.
- The public `v0.1.0` alpha release has a `-installer-r2.dmg` asset. It was accepted as **SHIP RESTRICTED** on macOS, with fixture-based Chromium/Firefox browser qualification, not live universal ATS coverage.

**Evidence status:** The previous release acceptance is reusable as a regression *reference*, not as a v0.2.0 result. No Windows 11 GUI run, Ubuntu desktop run, downloadable Windows/Linux installation, or v0.2.0 upgrade has been independently observed in this review.

## 3. Release gates and what counts as evidence

Use the six gates already defined in `docs/RELEASE_POLICY.md`, with one explicit v0.2 product-outcome check. Do not invent a competing scoring framework.

| Gate | Must be true for an advertised platform | Minimum acceptance evidence | Disposition if failed |
| --- | --- | --- | --- |
| **G1 Artifact identity** | Source, app, package, build version, OS/arch, digest are tied together | Git SHA; exact candidate; packaging logs; SHA-256; passing relevant CI | **HOLD** artifact |
| **G2 Installability** | A tester downloads the exact package in a clean account/VM and launches the real native host | OS/build, installation steps, WebView/runtime dependencies, first-run observation | **HOLD** advertised install claim; preview only with truthful restriction |
| **G3 Data integrity** | Saved opportunities, document bytes/identities, answers, letters, history and associations survive restart and supported upgrades | Representative synthetic before/after record and file hashes; app relaunch; real upgrade | **HOLD** affected platform |
| **G4 Applicant authority / privacy** | No unapproved transfer, overwrite, hidden disclosure, or employer submission; uncertain writes not replayed | Controlled real-browser fixture path; authorization and negative-path observations | **HOLD** affected assistance capability; not waived by faster workflow |
| **G5 Core native workflow** | A job seeker can complete the product's first and second application preparation paths in the installed app | Journeys A/B/C with a real WebView, native dialogs, and controlled employer fixture; limitations shown | **HOLD** or explicitly narrower technical preview |
| **G6 Recovery** | Browser loss, canceled native file dialog, optional AI absence, and app restart leave local work intact with an honest continuation path | Controlled interruption/cancel paths, state comparisons, usable recovery UI | **HOLD** if data loss/authority failure; otherwise restrict/repair based on effect |
| **G7 v0.2 experience outcome** | Second application actually involves less repeated work, with equivalent authority boundaries | Same scenario measured on v0.1.0 and v0.2.0; observed differences and remaining corrections | Do not claim the repeat-use improvement without evidence |

**Important distinction:** G4 and G3 are non-compensable. A quick or attractive application cannot earn acceptance by offsetting a data-integrity or consent failure. A failed optional engine/provider may be excluded if a usable manual path remains and the release claim is narrowed. Inability to launch the advertised package cannot be called a qualified release.

## 4. The minimum synthetic scenario (shared across platforms)

Create **one synthetic two-opportunity workspace**. Reuse it in the v0.1.0 baseline, v0.2.0 product tests, OS-native qualification, recovery, and upgrade checks. Build only the fixture support needed to replay this scenario; do not construct a generalized employer simulator first.

**Synthetic person and materials:**

- Applicant with fictional contact information and one or more *approved* Career Evidence records; no personal data from actual job seekers.
- At least two distinct résumé revisions with tracked file bytes/SHA-256, plus one supported DOCX or PDF import using a native file picker. Keep the originally selected version explicit.
- Opportunity **A**, already prepared, with one answer saved into the Answer Library, an optional prior letter containing employer-specific text, and a separate locally recorded completion state.
- Opportunity **B**, a different employer and role in the same approximate role family: one employer question *exactly matching* the saved answer, another *similar but not identical*, and one *new/unanswered* question.
- Controlled employer form fixture with a supported text field, file attachment, a field that requires manual action, and a form/field-identity change path. **Do not perform real employer submission** in this fixture.
- Missing/closed browser and optional AI-unavailable conditions available as controlled scenario variations.

**Why this fixture:** It exposes the principal v0.2 capability (reviewed reuse), the dangerous shortcut (silently applying similar answers or a résumé), transfer-state honesty, and the upgrade/data preservation properties in one meaningful path.

**Baseline method:** Drive v0.1.0 through its actual interface (or an existing fixture that reproduces its persisted state), record the operator's manual typing, choices, corrections, elapsed time to a reviewable form, and uncertainty. Drive v0.2.0 from the comparable starting state. Document any scenario differences rather than presenting the comparison as a controlled performance benchmark when it is not.

## 5. Native acceptance case matrix

The cases below define **outcomes**, not a mandate for one dedicated automated test per row. Prefer running them as one or two continuous installed-app journeys, with a few targeted interruptions.

| Case | Acceptance outcome | M1 Mac | W1 Win | L1 Ubuntu | Evidence form |
| --- | --- | :---: | :---: | :---: | --- |
| **N01** | Verify downloaded package hash, OS/arch, app version; install in clean user profile | R | R | R | Digest, environment, installer result, first launch |
| **N02** | Launch native WebView and reach Opportunities without account, AI key, or Chrome | R | R | R | Native screen/process observation |
| **N03** | Create opportunity; import PDF/DOCX; select exact résumé; restart with correct associations | R | R | R | Before/after IDs, content hashes, UI state |
| **N04** | First-application preparation works, including job with no narrative questions | R | R | R | Native walkthrough and observed next action |
| **N05** | Second application offers distinct exact reuse, suggested adaptation, and new answer; résumé suggestion requires explicit choice | R | R | R | UI observation, saved material identity, operator steps |
| **N06** | Prior-letter starting draft remains editable; no previous employer-specific claim silently transferred | R | R | R | Draft before/after and transfer selection |
| **N07** | Managed browser fixture opens; applicant reviews/authorizes Fill; actual observed outcomes reported; zero auto submissions | R | R | R | Browser process/fixture observations and result summary |
| **N08** | Unsupported/changed field and uncertain write require manual review; no unsafe retry/replay | R | R | R | Mutation count, review prompt, manual fallback |
| **N09** | Browser disappears / app terminates; drafts and bindings persist, employer-side work is not falsely reported as recovered | R | R | R | Interrupted native run, restart comparison |
| **N10** | Native picker select/cancel, native save/export, external link opening, keyboard access for primary path | R | R | R | Screen observation and exported file identity |
| **N11** | Install newer package on same profile without losing workspace; uninstall behavior matches published data-retention policy | R | R | R | Real before/after upgrade and retained-data check |
| **N12** | No secrets or applicant text in shareable release diagnostics; optional AI/browser prerequisites fail locally without breaking job management | R | R | R | Sanitized error output + unaffected workspace |

`R` = required to claim the full *qualified beta* experience on that OS. Where a particular optional capability is intentionally unoffered, its row may receive **RESTRICTED** with a published scope reduction and an approved owner decision; it must not be silently marked PASS. For a new OS with no prior published version, **N11** means *v0.2 candidate A → v0.2 candidate B*; for macOS it means *public v0.1.0 → v0.2.0 candidate*.

**Do not test actual submission:** Inspect employer fixture state before and after authorized Fill, ensure no submission request occurred, and let the tester manually record a synthetic completion event in YourMove. That proves the authority boundary without submitting to an employer.

## 6. Platform-specific qualification notes

### M1 — macOS 26+ arm64: regressions and the first real upgrade

- Test the **actual v0.1.0 published artifact** and its workspace, then install the new v0.2 candidate on the **same synthetic user's profile**. Validate exact documents, stored answer/letter contents, selected résumé revision and hash, Career Evidence relationships, and application history.
- Preserve the existing native Wails/React application path and its manual employer submission boundary. Exercise file-picker cancel and exported files, previously noted as incomplete in the v0.1 acceptance record.
- Test the distribution path claimed for v0.2. An **ad-hoc signed/unnotarized** tester package is not a normal frictionless public Mac installer; if Developer ID/notarization is still absent, document Gatekeeper behavior and mark distribution appropriately restricted.
- Existing Chromium and Firefox controlled fixture evidence supports **regression planning only**; re-run against the v0.2 native candidate and its selected browser engine(s). Do not infer Safari or WebKit employer-browser support from the macOS WebView.

### W1 — Windows 11 x64: a new native and managed-browser path

- Start with the actual Wails entry point using **WebView2**. Confirm that an expected Windows 11 user profile can launch and use it. Test and document missing/unsupported WebView2 behavior instead of relying on the developer machine.
- Replace/extend macOS-only Wails build conditions where required, and implement browser discovery that understands Windows `.exe` locations and file semantics; Unix execute-bit validation is not portable evidence.
- Exercise the native open/save dialogs, external browser link, isolated managed-browser process/profile lifetime, and workspace location determined by Windows application configuration behavior.
- Deliver **one** candidate installer, preferably NSIS; verify start-menu/uninstall paths, upgrade into the existing install location, and the resulting data-retention policy. Do not expand to MSIX unless needed.
- The exact Windows 11 x64 clean graphical VM and Chrome build used must be recorded. Passing `go test` on a Windows GitHub runner alone is not a WebView or installer acceptance run.

### L1 — Ubuntu 24.04 x64: native GTK/WebKit and Debian package behavior

- Start with a genuine Ubuntu 24.04 desktop x64 environment. Wails v3 documentation identifies **GTK4 + WebKitGTK 6.0** as the default Linux stack for this distribution; explicitly record the available runtime package versions and desktop session.
- Verify a GUI launch and input handling through the embedded WebView, native file dialogs, exported files, persistent XDG/config paths, and the controlled Chromium process.
- Create **one** `.deb`; verify its declared runtime dependencies on a **clean** Ubuntu desktop, graphical launch after installation, icon/menu entry, upgrade and uninstall outcomes. A developer image with all build dependencies already installed is insufficient.
- Record whether the qualified session is Wayland or X11. Do not claim both without testing both; test the alternate session only if supporting it is part of the advertised audience or a real failure makes it necessary.

### Shared technical decision — Wails and Go version

The pinned app uses Wails `v3.0.0-beta.32` while official Wails documentation shows `v3.0.0` released October 26 and `v3.1.0` on November 1, 2026. **Before broad native integration**, run one bounded compatibility comparison: current pin vs a selected stable Wails release, using *YourMove's* asset handler, POST form bodies, dialogs, and native launch. Select a version based on observed compatibility, not novelty.

Official Wails release notes also identify patched Go toolchains (including Go 1.27.3) for a `crypto/x509` security fix; YourMove currently declares Go `1.27.2`. Verify and plan the relevant Go patch-level update before release qualification. This is not a reason to delay the first functional platform spike.

## 7. Testing layers: right evidence at the right cost

| Layer | When to run | What it proves | What it does **not** prove |
| --- | --- | --- | --- |
| **T0 — fast checks** | Each meaningful PR | Frontend build, Go tests, focused storage/authorization logic | Native platform experience |
| **T1 — native build/package CI** | Platform enablement PRs and candidate | OS-native compilation, package structure, source/artifact identity | Clean end-user install or GUI usability |
| **T2 — controlled browser qualification** | Browser integration changes and release candidates | Real browser process; inspect/authorize/fill/observe against synthetic employer fixtures | Live-ATS coverage or final employer submission |
| **T3 — frozen installed-native acceptance** | Every advertised release artifact on every advertised OS | Actual download, install, WebView, dialogs, persisted user journey, recovery, upgrade | General correctness across machines or employers |

**CI direction (not prescribed implementation):** Keep the existing Ubuntu Go and macOS desktop jobs. Add Windows and Ubuntu desktop build/package checks as those hosts become executable. Run focused browser-fixture qualification where real browsers and display sessions exist. Maintain one small **human-observed** installation acceptance per release OS; automation may assist it but should not substitute for confirming the actual UI and installer.

**Efficiency constraint:** Do not multiply every UI case by every engine, platform, and ATS. Qualify the same core fixture on each advertised OS's **primary supported engine**, plus targeted alternate-engine tests only if those alternatives are included in that OS's support promise. Live employer-specific sampling is separate opt-in qualification, not a prerequisite for honest manual-only fallback claims.

## 8. Evidence and privacy contract

Each release candidate must have a compact, inspectable **per-platform acceptance record**. Save it under `release/` or the existing release-decision process, with links from `release/decisions/v0.2.0.md`; avoid parallel reporting frameworks.

Minimum record fields:

```text
release: 0.2.0
source_sha: <full Git SHA>
app_version_build: <shown version/build>
platform: <macOS 26 arm64 | Windows 11 x64 | Ubuntu 24.04 x64>
package: <exact filename>
package_sha256: <verified digest>
webview: <engine and observed version, if accessible>
managed_browser: <name/version/path or unavailable>
environment: <clean VM/device, OS build, dependencies, session>
fixture: <synthetic workspace revision and document SHA-256 list>
results: <N01 ... N12, with PASS / RESTRICTED / BLOCKED / NOT RUN>
notes: <first-run prompts, manual fallbacks, data/authority observations>
evidence: <sanitized screenshots, CI run links, command summaries>
owner_disposition: <SHIP / SHIP RESTRICTED / HOLD, audience, exclusions>
```

**Capture enough to verify the claim, not private user data.** Do not publish resumes, actual form answers, credentials, cookies, Chrome profiles, raw CDP payloads, or provider API keys. Synthetic fixture content is acceptable. Error logs should be checked for accidental field values before sharing. CI success and screenshots are supporting evidence; neither alone decides that the user intended a transfer.

**Comparison worksheet for Journey C:** Record number of repeated text entries, explicit document selections, edits/corrections, elapsed time to reviewable employer form, unsupported/uncertain states, and operator comprehension of prepared / filled / submitted. Use v0.1.0 and v0.2.0 on a comparable scenario. Prefer observed absolute counts over an arbitrary percentage target; do not publish an improvement figure until measurements exist.

## 9. Release decision rules

| Verified outcome | Permitted decision | External statement |
| --- | --- | --- |
| M1 + W1 + L1 all pass their artifact-specific gates | `SHIP` or `SHIP RESTRICTED` depending on installer trust and documented limitations | List three **qualified** OS/architecture targets and precise browser/ATS boundaries |
| M1 + W1 pass; L1 fails or remains untested | Ship only passing platform artifacts if owner approves, with L1 absent or explicitly preview | State macOS + Windows support only; Ubuntu is not qualified |
| M1 + L1 pass; W1 fails or remains untested | Ship only passing platform artifacts if owner approves, with W1 absent or explicitly preview | State macOS + Ubuntu support only; Windows is not qualified |
| Only M1 passes | Product-owner decision whether to narrow/postpone v0.2; cross-platform expansion outcome **not** achieved | Do not market v0.2 as cross-platform expansion |
| Any advertised artifact loses/misassociates data or exceeds authorization | `HOLD` that artifact/capability; do not average the failure away | No unsupported trust or completion claims |

A restricted release must say exactly **which feature/platform/audience is restricted** and what usable manual alternative exists. If a platform can organize jobs but fails the employer-assisted journey, call it a **technical preview with limited capabilities**, not a qualified equivalent of Mac.

**Decision authority:** The release owner signs the exact artifacts and their status after reviewing evidence. A failed gate moves the release or narrows the scope, rather than lowering the quality standard to meet December 4.

## 10. Smallest useful next actions

1. Approve or amend this qualification contract; treat A's product direction as already approved. The remaining specification decisions concern **test environments, accepted package formats and release claims**, not reopening the product vision.
2. Get one Windows 11 x64 and one Ubuntu 24.04 x64 graphical test environment. Identify the operator and how screenshots/logs can be captured without exposing personal data.
3. Seed the two-opportunity synthetic workspace and capture the v0.1.0 Journey C baseline on macOS; retain exact document hashes and operator observations.
4. Run the Wails/current-pin-versus-stable compatibility spike. First demonstrable outcome: a native Windows or Ubuntu window opens the real YourMove UI and preserves a saved opportunity across restart.
5. Implement the first native platform slice, followed by the controlled employer fixture and only then an installer. Keep the repeat-use product slice parallel where it does not modify shared browser/storage decisions.

**Stop adding pre-development process once:** The selected platforms and qualification levels are clear, one reproducible test workspace exists, native test access is available, and the first platform/product vertical slices have observable acceptance. Unresolved technical details belong in a short executable spike, not a speculative architecture document.

## Sources and provenance

**Approved product basis:** `YourMove-Go-v0.2.0-Product-Experience-Spec-DRAFT.md` (approved by owner in conversation on November 5, 2026); `YourMove-Go-v0.2.0-proposed-scope.md` (scope discussion, not independently approved as an engineering backlog); `yourmove-go-300-v020-candidates.md` (ideation inventory). The `DRAFT` filename of A is historical; owner approval applies to its content, and any changes require explicit review.

**Repository observations:**

- Source and CI: https://github.com/jenksed/yourmove-go/blob/main/desktop.go and https://github.com/jenksed/yourmove-go/blob/main/.github/workflows/ci.yml
- Browser locator: https://github.com/jenksed/yourmove-go/blob/main/internal/application/browser/chromium/executable.go
- Storage: https://github.com/jenksed/yourmove-go/blob/main/internal/storage/paths.go
- Release policy and prior acceptance: https://github.com/jenksed/yourmove-go/blob/main/docs/RELEASE_POLICY.md and https://github.com/jenksed/yourmove-go/blob/main/release/macos/ACCEPTANCE-v01-build3-048d0a3.md
- Release archive: https://github.com/jenksed/yourmove-go/releases/tag/v0.1.0

**External documentation consulted for technical constraints (not YourMove execution evidence):**

- Wails installation/platform dependencies: https://wails.io/quick-start/installation/
- Wails Windows packaging and WebView2: https://wails.io/guides/build/windows/
- Wails Linux packaging: https://wails.io/guides/build/linux/
- Wails v3 release history: https://wails.io/changelog/

**Explicit evidence limit:** This is a proposed *test and acceptance contract*, not a claim that any N01–N12 test has been executed for v0.2.0 or that additional operating systems are supported today.
