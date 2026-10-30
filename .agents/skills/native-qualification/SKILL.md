---
name: native-qualification
description: Qualify an authorized YourMove Next applicant-facing change in the genuine macOS Wails desktop application, including generated bindings, Go/SQLite behavior, native interaction, and same-executable/workspace recovery when relevant. Use before planning or claiming native GUI verification, acceptance walkthroughs, and native regressions; not for documentation-only or isolated backend tests.
---

# Native qualification — YourMove Next

Prove what the applicant can **actually do in the native Wails window**, without treating mock tests, a browser preview, or a Go-only probe as a substitute. This is a verification procedure, **not** authorization to start a YMN task, change a workspace, execute browser effects, merge code, or accept a milestone.

First confirm the assigned scope and current authorization in the applicable sections of `docs/next/TASK_INDEX.md` and `docs/next/CHANGE_CONTROL.md`. Follow `AGENTS.md` and the normative boundaries in `docs/next/RELEASE_BOUNDARY.md`. For native launch/restart, consult `docs/next/LIVE_DEVELOPMENT.md`; for precedent, read only the relevant portions of `docs/next/YMN-001_EVIDENCE.md`, `docs/next/YMN-007_NATIVE_QUALIFICATION.md`, or `docs/next/YMN-008_IMPLEMENTATION_EVIDENCE.md`. Historical walkthroughs are evidence of **past** runs, not reusable fixtures or automatic permission to rerun fault injection.

## Select the claim before selecting tests

| Verification surface | Can establish | Cannot establish |
| --- | --- | --- |
| Vitest and focused Go tests | Component states; Go policy, revisions, storage and controlled negative cases | Actual Wails GUI interaction |
| `./scripts/dev-browser.sh` | Disposable React preview, presentation and draft interactions | Wails transport, SQLite persistence, native dialogs or browser effects; native transport is blocked |
| `./scripts/dev-native.sh` | Genuine Wails interactions on a newly marked synthetic workspace during development | Same-*executable* qualification across automatic rebuilds; packaged installation |
| `./scripts/build.sh` + `bin/YourMoveNext` | A fixed production-build executable exercised in real Wails | Installable/signed/notarized distribution or a release decision |

Use the **smallest** native walkthrough that supports the assigned acceptance claim. Do not run a full product regression or accessibility matrix for every isolated change. For YMN-014's accessibility scope or YMN-016's frozen journey, follow their wider task-specific matrices. If the change includes employer browser operations, also load `.agents/skills/browser-workflow-testing/SKILL.md` and preserve its separate Playwright/CDP evidence requirements.

## Preflight before opening the app

1. Identify checkout, source revision, dirty changes, host architecture, and intended executable. Confirm the assigned worktree and that the binary corresponds to the source being qualified. `git rev-parse --show-toplevel`, `git rev-parse HEAD`, `git status --short` and (for a fixed production binary) `shasum -a 256 bin/YourMoveNext` provide useful identity. Never assume an old screenshot or binary belongs to current HEAD.
2. Identify any already-running Wails app, dev watcher, Vite listener, browser, and **workspace owner** before launching or stopping anything. `pgrep -fl YourMoveNext` and `lsof -nP -iTCP:9245 -sTCP:LISTEN` are inspection aids; a matching port/PID is not permission to kill a process. Never launch a second writer on one root or remove a lock to bypass isolation.
3. Use **only** a newly created, explicitly marked synthetic workspace for new qualification, or an existing *verified* synthetic workspace expressly assigned for continuation. Never mark an existing unmarked directory or inspect real applicant records. Record the absolute root and its baseline state; do not assume it is empty if reused.
4. Run relevant focused tests and, when warranted by the source change, `./scripts/check.sh` and `./scripts/build.sh`. The check script covers frontend, Go race/vet, binding drift and architecture import checks; it does **not** exercise the native UI. Do not regenerate bindings/build concurrently with another active dev watcher or integration stream.

For a **new fixed-binary** walkthrough, after an authorized build and preflight:

```sh
mkdir -p "$HOME/Projects/yourmove-next-testdata/native-qualification"
root=$(mktemp -d "$HOME/Projects/yourmove-next-testdata/native-qualification/native.XXXXXX")
printf 'YMN_SYNTHETIC_WORKSPACE_V1\n' > "$root/SYNTHETIC_ONLY"
printf 'Native synthetic workspace: %s\n' "$root"
shasum -a 256 bin/YourMoveNext
./bin/YourMoveNext --workspace "$root"
```

Retain the printed root for the **entire walkthrough**, including restart. `./scripts/dev-native.sh` creates a *different* root on each invocation, so running it twice cannot prove same-workspace persistence. Native dev/HMR is valid for interaction checks; use a recorded fixed binary when claiming *same-executable* restart. Avoid rebuilding between qualifying the binary and its restart unless the test explicitly measures an upgrade, in which case record both hashes.

## Exercise the real applicant path

Trace each material claim through the actual Wails window → generated service binding → Go-owned validation/use case → persistent writer or authorized adapter → visible result. Use actual keyboard/mouse/accessibility interactions in the native window. Direct service calls or SQLite queries can corroborate, **not replace**, the UI interaction.

For the affected feature, select representative scenarios:

- **Working action:** Perform the exact applicant action (such as editing an Opportunity, choosing a retained résumé, saving an answer, or reviewing an evidence entry). Record the visible result and any relevant revision, document/version identity or operation receipt. Do not turn an optimistic UI message into proof that a write committed.
- **Relevant refusal/recovery:** Exercise invalid input, failed save, revision conflict, cancel, missing file, incomplete operation or another negative case *only if applicable*. Verify recoverable unsaved text/focus and that refused changes do not silently persist. Do not fabricate a conflict by writing directly to an active app's SQLite database. Use only separately reviewed, explicitly authorized synthetic fault-injection helpers with their preconditions, or mark that native scenario NOT VERIFIED and rely on scoped Go tests for the narrower claim.
- **Native-only behavior when affected:** Verify actual open/save dialogs, file identity, keyboard focus/navigation, accessibility semantics, window sizing, theme, or browser process ownership only to the extent required by this task. A successful browser-preview screenshot or AX-free component test does not cover these claims.
- **Durability when affected:** Before exit, record which values were **explicitly saved** and which were only unsaved drafts. Quit Wails (for example Cmd-Q); confirm the original process has stopped. Relaunch the **same executable/hash** with the **same absolute root**, without reseeding. Verify saved values/revisions/associations and relevant error or history state in the native UI, optionally corroborated by read-only SQLite/file checks. Unsaved React drafts are not expected to survive process shutdown. In-flight external effects must not be blindly replayed.

If the agent cannot control/observe the real macOS window, arrange a bounded owner-operated walkthrough, state which results are **owner-reported**, and leave unobserved paths NOT VERIFIED. Do not install a new desktop automation framework or add app hooks merely to manufacture a passing receipt.

## Evidence and handoff

Report concise scenario results with **PASS / FAIL / NOT VERIFIED / NOT APPLICABLE** and the exact observation supporting each result. Include source HEAD/dirty-state qualification, test/build commands and exit results, executable hash and mode (dev/fixed/packaged), synthetic root identity, native interaction method (manual/AX/other), relevant SQLite/file or operation receipts, same-root restart result when tested, and outstanding limitations. Capture screenshots or AX/short recordings only from invented data; sanitize paths, private content and credentials. Distinguish agent-observed from owner-reported evidence.

Keep **IMPLEMENTED**, **VERIFIED**, and **OWNER-ACCEPTED** separate. A green CI run, successful Go test, visual preview, or native screenshot does not by itself establish the whole acceptance matrix. Never declare DONE, push/merge/release, use real applicant data, or start the next milestone without the owner's explicit authorization. Stop and report specific missing evidence rather than inventing a PASS.
