# Delivery record

- Branch: `feat/phase2-product-experience`.
- Base: `6709840d70e84e1df0d66425479c1bf3ac17574a` (Phase 1 / PR #6).
- Initial Phase 2 worktree head for final native qualification: `539f952`; the user-reported PR head at assignment start was `49f7062`. The existing worktree already included the description/salary follow-up.
- Visual qualification production source: `f050e16`; controlled-test harness: `2553ece` (documentation and evidence commits follow).
- Target: `main`; draft review only. No merge or release authorized/performed.
- PR: [#7 — Implement Phase 2 labeled workbench product experience](https://github.com/jenksed/yourmove-go/pull/7) (draft, targeting `main`). Closeout diff is uncommitted; native merge gate remains open.

## Implementation commits

| Commit | Contribution |
|---|---|
| `f4054b4` | Labeled shell and compact Opportunities. |
| `0346e36` | Direct Answers/Cover letter and exact transfer review. |
| `ad18f24` | Document identity, versions, sizes and resume suggestions. |
| `624042e` | Question-first library and reviewed experience. |
| `9287e9c` | Applicant approval and optional AI setup. |
| `2e766ca` | Local completion history and cross-activity recovery. |
| `7cfb7ee` | Whole-product polish, Submissions label and new qualification tests. |
| `99790d7` | Short-window navigation and breakpoint focus. |
| `b3ad960` | Saved-writing-first disconnected recovery regression. |
| `4b35159` | Explicit activity vs query-bound return destinations. |
| `d5cf28c` | Await actual no-script disclosure state; failure diagnostics. |
| `bcc65cf` | User-requested transfer-review simplification; Back/history and no-JS activities. |
| `f6cc186` | Completion/clipboard qualification, owner warning and consistent recording action. |
| `f050e16` | Optional focused writing on wide windows; compact navigation remains the sole compact control. |
| `2553ece` | Stable controlled pointer press/release on one connection; focused-writing regression. |
| `1635809` | Engineering handoff, acceptance matrix and measured before/after evidence. |
| `f0100bc` | Dynamic loopback port for runtime restart qualification; refreshed stale recovery assertions. |
| `bac809c` | Final Wails acceptance evidence, updated status matrix and native screenshots. |
| `a2ab7da` | Reproducible manual VoiceOver/zoom/reduced-motion and loopback-transfer acceptance procedures. |

Subsequent description-spacing and compensation-capture work is included at `539f952`. See the follow-up section in HANDOFF.md. Final ordinary and desktop suites pass after `f0100bc`; required opt-in product journeys, isolated QuestionWorkbench rerun, dynamic-port RuntimeRestart and guarded Chromium trials pass. One initial combined-run QuestionWorkbench disclosure timeout did not reproduce in isolation.

## Final native qualification run

- Source: `f0100bc57adb47f324222d7b1020830bb12d42fe`; bundle ID `run.yourmove.workflowqualification`.
- Ad-hoc executable and bundle signature: strict verification passed before launch and after screenshot capture. This is development signing only.
- Startup: Wails `v3.0.0-beta.32`, PID 46556, isolated synthetic workspace. Process remained alive during native interaction.
- Native outer dimensions were set/read at 1100×800, 900×700, 750×650 and 700×550. AX scroll-area sizes were respectively 1101×769, 901×669, 751×619 and 701×519; direct WebView `innerWidth`/`innerHeight` were not observed.
- App-only Wails captures show Opportunities at all four sizes in dark appearance; Prepare and Documents in dark; Answers, Cover letter and Role families in light. The Light choice was made in the native app.
- Accessibility settings were updated by the user and System Events then exposed the actual Wails accessibility tree. Native interactions included named navigation, opening the synthetic job, selecting Answers and Cover letter, opening Role families and adding built-in examples, native picker cancellation, and importing `Synthetic résumé 漢字.pdf`. The 142-byte imported PDF matched the source SHA-256, and resume association persisted in the synthetic job.
- Free-form text edit/dirty dialog and multiple dirty editors were not verified. Export/save dialog, History/completion, quit/relaunch recovery, VoiceOver, zoom/enlarged text/reduced motion remain unverified. Native transfer was not attempted because the current fixture is not a safe loopback destination; no native employer fixture/factory was built. Controlled Chromium evidence is separate.
- Port 3000 remains untouched. The full ordinary-app restart qualification now uses a dynamic loopback port and passed.
- PR #7 remains draft; no merge or release occurred.

## Review scope

Production changes are concentrated in existing Go view data, templates and shared CSS/JS. There are no new dependencies, database migrations, SPA/router, release-signing changes, autosave, employer submission calls or provider integrations. Existing browser policy/identity/authority implementations remain intact.

New files include unit/controlled-browser product regression coverage and the implementation handoff/acceptance/gallery. Existing tests are updated for deliberate navigation/copy changes; transfer, revision, dialog, dirty-state and privacy assertions are retained.

Screenshots are synthetic before/after production renders (roughly 5.4 MiB), plus app-only native Wails captures (roughly 4.5 MiB). They do not contain applicant data, other desktop windows, browser profiles, build outputs or secrets. Historical design/stabilization documents and captures are preserved; the old brief test now writes its regenerated evidence to temporary storage.

## Outstanding merge/release gates

Current closeout verification: salary mixed benefit excerpts abstain and preserve clean salary/structured precedence; dirty completion confirmation uses the application-owned modal. Full ordinary and `desktop` Go suites pass, as do the affected controlled Chromium profile-completion/submission workflows. Commits `799528e` and `02726ab` contain those fixes. A fresh Wails bundle launched from a new isolated workspace and passed ad-hoc strict signing. Native attachment failed before reaching YourMove: `osascript` requests for System Events return `-10827`, and LaunchServices reproduces `LSOpenStuffCallLocal` / `kLSNoExecutableErr` for System Events, Accessibility Inspector, Finder and Calculator despite each declared executable being present. GitHub CI for the pushed closeout commits remains to be checked. Native Journeys A–D remain NOT VERIFIED; this condition prohibits merge.

- Direct WebView `innerWidth`/`innerHeight` measurement; only AX scroll-area dimensions were recorded.
- Native dirty-editor keyboard/dialog/focus behavior and multiple dirty editors.
- Native PDF/letter export dialog, History/completion, and quit/relaunch recovery.
- VoiceOver, 200% zoom, enlarged text and reduced-motion interaction evidence.
- Dedicated synthetic loopback transfer fixture/factory with exact destination ownership, independent outcomes and zero submissions.
- Owner review of design, especially the simplified recovery scope and compact navigation.
- Existing independent release signing/notarization/acceptance requirements.

See [ACCEPTANCE.md](ACCEPTANCE.md) for exact status and reproduction. The draft PR must not imply full native or release acceptance.
