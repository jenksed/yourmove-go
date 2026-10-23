# Delivery record

- Branch: `feat/phase2-product-experience`.
- Base: `6709840d70e84e1df0d66425479c1bf3ac17574a` (Phase 1 / PR #6).
- Final production code: `f050e16`; final controlled-test harness: `2553ece` (documentation and evidence commits follow).
- Target: `main`; draft review only. No merge or release authorized/performed.
- PR: pending creation after final evidence commit.

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

## Review scope

Production changes are concentrated in existing Go view data, templates and shared CSS/JS. There are no new dependencies, database migrations, SPA/router, release-signing changes, autosave, employer submission calls or provider integrations. Existing browser policy/identity/authority implementations remain intact.

New files include unit/controlled-browser product regression coverage and the implementation handoff/acceptance/gallery. Existing tests are updated for deliberate navigation/copy changes; transfer, revision, dialog, dirty-state and privacy assertions are retained.

Screenshots are synthetic before/after production renders, roughly 5.4 MiB total, with selected compact-window/dialog evidence and measured CSS dimensions. They do not contain applicant data, browser profiles, build outputs or secrets. Historical design/stabilization documents and captures are preserved; the old brief test now writes its regenerated evidence to temporary storage.

## Outstanding merge/release gates

- Actual native interaction and visual acceptance at all four outer sizes, with inner WebView measurements.
- Native picker/export/cancellation, VoiceOver, 200% zoom, multiple dirty editors, restart and controlled employer transfer.
- Full HTTP runtime restart test: skipped because port 3000 was occupied; existing process left untouched.
- Owner review of design, especially the simplified recovery scope and compact navigation.
- Existing independent release signing/notarization/acceptance requirements.

See [ACCEPTANCE.md](ACCEPTANCE.md) for exact status and reproduction. The draft PR must not imply full native or release acceptance.
