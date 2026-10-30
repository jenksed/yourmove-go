# YourMove Go — contributor instructions

## Product and scope

YourMove is a local-first job application workbench. The user selects and approves material, reviews employer-side effects, submits applications personally, and records completion explicitly. Preserve the existing working applicant journey before improving its implementation.

Work only on the owner's currently authorized task. Confirm the branch/HEAD, relevant worktree changes, intended user result, and scope before editing. `main` is the starting baseline, not permission to commit directly to it, merge, publish, or operate on someone else's worktree. Do not modify unowned/untracked files or stop another process without confirming ownership.

Do the smallest useful end-to-end change. An implementation that improves only directory symmetry, scaffolding, test infrastructure, or architecture without enabling the assigned user outcome needs a specific current blocker to justify it. Report adjacent cleanup separately.

## Runtime and migration direction

**Keep Wails 3. No Tauri or Rust migration.** This repository currently runs one Go application in two hosts:

- `desktop.go`: macOS Wails window, native dialogs and embedded Go HTTP handler.
- `main.go`: local browser-hosted HTTP mode.
- `internal/app/`: HTTP routes, workflows, applicant-facing application behavior.
- `internal/storage/`: SQLite schema, paths, and durable workspace handling.
- `internal/platforms/` and `internal/application/`: public posting data, application requirements, browser control and Chromium/CDP integration.
- `templates/` and `static/`: existing Go-rendered UI and progressive enhancement.

The authorized frontend direction is **React + TypeScript + Vite within the existing Wails app**, migrated screen by screen. `frontend/` is a *proposed* new source tree, not an already-established package. Prefer organizing migrated code under `frontend/src/features/<workflow>/` with only genuinely reused code under `shared/`.

Preserve existing Go behavior and SQLite records. Go remains authoritative for validation, saved state, document identities, AI/provider policy, consent, browser effects, and completion recording. React owns display, user interaction and disposable unsaved state. Do not implement a parallel business-rule engine or durable store in TypeScript.

For the first migration slice, trace the present Go HTTP route/handler and select the smallest working React-to-Go seam. Existing protected HTTP endpoints may be reused when suitable; new narrow JSON handlers or Wails bindings must be justified by the actual UI need. Do not assume a generated-binding stack exists. If introducing bindings, define Go contracts first and use the pinned Wails version's supported generation mechanism. Preserve local request protections, Wails navigation and file-dialog behavior, and browser-mode compatibility for unaffected screens. Do not establish a second unprotected local API.

## Unchanged authority and data boundaries

- Only explicitly approved candidate information may leave the app. AI is optional, accurately disclosed, consent-aware, and never silently switches providers.
- Employer-browser actions require specific applicant authorization. No automatic employer submission, login, CAPTCHA, privacy/consent decisions, or replay of uncertain external effects.
- `POST /jobs/{id}/submit` records the applicant's explicit declaration of manual submission locally; it does not submit to an employer.
- Keep job IDs, résumé/file bytes and associations, saved answers, letters, source identity, and recorded application history intact. Never silently overwrite or misassociate candidate data.
- Qualification uses isolated synthetic data/workspaces selected by `YOURMOVE_DATA_DIR`. Never point tests, fixtures, or repair commands at a real applicant workspace. Respect the single application writer and its SQLite state. An employer-browser session does not survive app restart.

## Finding the real implementation

Start with `README.md`, `docs/CODEBASE_GUIDE.md`, and `docs/TRY_IT.md` as needed; the current source and tests resolve discrepancies in documentation. For UI changes, consult `docs/design/DESIGN_SYSTEM.md` and `docs/design/RULES.md`, especially the current decision, object-first composition, and truthful states. For backlog selection, read `docs/BACKLOG.md`, then verify dated claims against current code/PRs. Release work additionally consults `docs/RELEASE_POLICY.md` and `docs/RELEASE_CHECKLIST.md`.

<!-- graft:start -->
### Graft (optional code navigation)

If Graft is available and its index is current, use it to orient within unfamiliar code or trace callers. `graft map`, `graft ask "<question>" --source`, `graft skeleton <file>`, `graft callers <symbol>`, and `graft grep "<literal>"` can narrow inspection. A graph result is navigation evidence, not proof of runtime behavior; check the relevant source and tests. Use ordinary search/source inspection when simpler, when Graft is unavailable, or when the graph is stale. Do not treat Graft as a gate or build a new index for an isolated change. Refresh after substantial structural changes if needed; do not commit generated `graft/` artifacts.
<!-- graft:end -->

## Load project skills when applicable

Project skills live in `.agents/skills/`; they supplement this file and do not authorize work or supersede the code. Read only the ones relevant to the task, and have delegated agents load them directly:

- `feature-slice` — an applicant-visible workflow spanning UI/Go/storage or other layers.
- `frontend-migration` — replacing an existing Go-rendered screen or `static/` behavior with React/TypeScript under Wails; **required for the planned Opportunities slice**.
- `native-qualification` — claims about the actual Wails UI, native host, or saved behavior after quit/relaunch.
- `durable-state-integrity` — changes to persisted records, document files, schema, saved associations or recovery.
- `browser-workflow-testing` — Chromium/CDP, employer sessions, forms, controlled transfer testing or related effect boundaries.
- `employer-continuation` — changing applicant-authorized employer transfer/filling, not ordinary public job retrieval.

Do not load every skill for every task. Pure layout work does not need a database recovery procedure; an Opportunities read/write slice ordinarily needs `frontend-migration`, `feature-slice`, `native-qualification`, and `durable-state-integrity` only where its write path changes.

## Verification and handoff

Exercise the real path early. Use focused existing tests first; `go test ./...` and, on supported macOS, `go test -tags desktop ./...` are baseline commands. For native workflow qualification, inspect `scripts/qualify-native-workflow.sh` and its documented limits before running it. A browser preview, Go test, Chromium fixture or successfully built bundle does **not** establish native Wails acceptance. For restart claims, fully quit and reopen the *same executable* against the *same synthetic workspace* without reseeding it. Do not treat signing a development bundle as approval to distribute.

Report the observable outcome, modified paths, source revision, tests actually run/results, native or browser observations, and anything **NOT VERIFIED**. Distinguish **implemented**, **verified**, and **owner-accepted**. Do not push, merge, release, access real applicant data, or start the next task without explicit owner authorization.
