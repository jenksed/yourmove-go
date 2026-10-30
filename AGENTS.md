# YourMove Go — contributor instructions

## Purpose and authorization

YourMove is a local-first job application workbench. Applicants approve their materials, review employer-side effects, submit applications personally, and explicitly record completion. Preserve the working journey as the code improves.

Work only on the owner's currently authorized task. Before editing, confirm the assigned worktree, branch/HEAD, dirty or untracked changes, intended result, and scope. `main` is a baseline, not permission to modify it. Do not overwrite unowned work, interfere with another process/worktree, or push, merge, publish, or release without authorization. An architecture plan or skill is not task authorization.

Modernization has **two goals**: migrate presentation incrementally to React/TypeScript/Vite, and improve Go ownership, navigation, and maintainability. Prefer a small working vertical slice. Refactor Go along the touched path when it fixes demonstrable coupling/confusion. Standalone maintainability work requires explicit authorization and a concrete improvement; folder symmetry, generic scaffolding, and arbitrary file-size targets are not justification.

## Application architecture

**Keep Wails 3 and the existing Go application. No Tauri/Rust migration or second backend.**

| Location | Responsibility |
| --- | --- |
| `desktop.go` / `main.go` | Wails macOS host / local-browser host of the same Go application |
| `assets.go` | Embedded resources |
| `internal/app/` | Go composition, routes, protected HTTP transport, Wails integration and legacy compatibility; existing operations may remain here |
| `internal/features/<feature>/` | **Optional** Go feature packages, only for safely isolated cohesive operations |
| `internal/storage/` | SQLite setup, workspace paths and durable state |
| `internal/platforms/` | Public posting retrieval |
| `internal/application/` | Application requirements and employer-browser/Chromium control |
| `templates/`, `static/` | Unmigrated Go UI and progressive enhancement |
| `frontend/src/app/` | React shell/composition and routing, when needed (not a product feature) |
| `frontend/src/features/<feature>/` | React feature screens, interactions and feature-specific transport adapters |
| `frontend/src/shared/` | UI code with demonstrated cross-feature reuse only |
| `frontend/` | **Established** React, strict TypeScript and Vite implementation |

Keep `go.mod` and Wails entry points at the root. Separate transport, Go decisions, persistence, and external effects where it improves an actual change path. Prefer cohesive feature-named files within existing packages before extracting new ones; avoid default ORM/service/repository frameworks. Do not move `app` under `features/` or create empty feature/shared directories.

React and Go-rendered routes may coexist in the **same Wails window**, with full-page navigation between them. Organize migrated UI under `frontend/src/features/<workflow>/`; create `shared/` only for demonstrated reuse. First inspect existing protected HTTP handlers; add a narrow JSON response or Wails binding only when the UI requires it. Never introduce a second unprotected listener/API. Embed Vite's production build in the executable; preserve existing Wails navigation, native dialogs and unaffected browser-mode routes.

**Go remains authoritative** for validation, saved state, document identity, AI/provider policy, consent, browser effects and completion recording. React owns presentation, interaction and disposable drafts—not parallel rules or applicant-data storage.

### Feature ownership during modernization

The standing architecture decision is **`docs/architecture/FEATURE_OWNERSHIP.md`**. For a React workflow migration, Go ownership refactor, feature-package extraction, or change to a cross-feature writer/transaction, read the **relevant sections** before deciding where code belongs. Do not require it for isolated styling, docs, or routine fixes unrelated to those boundaries. It guides placement; it does **not** authorize extra work.

`internal/app` may import an extracted `internal/features/<feature>` package; feature packages must **never import `internal/app`**. Before extraction trace real callers, locks, transactions, source-of-truth models, and operations spanning Opportunities, Résumés, deletion, or submission history. Keep one authoritative writer and existing coordination. Compose using concrete dependencies rather than speculative interfaces or a DI framework. A cohesive same-package refactor is often better than creating wrappers or duplicate models.

Treat each screen migration as one usable React → Go → persistent-state capability. Improve a demonstrated Go responsibility problem on that path, or record why its existing ownership is adequate. Do not extract packages or migrate other screens just to match the architecture diagram.

## Applicant and workspace boundaries

- Only specifically approved candidate information leaves the app. AI is optional, consent-aware, accurately disclosed, and never silently changes providers.
- Employer-browser actions require specific authorization. No automated employer Submit, login, CAPTCHA, privacy/consent decisions, or replay of uncertain effects.
- `POST /jobs/{id}/submit` records the applicant's **manual submission declaration locally**; it never submits to an employer.
- Preserve job/source IDs, manual edits, résumé bytes/associations, saved answers, letters and application history. No silent overwrites or misassociation.
- Use only isolated synthetic workspaces selected through `YOURMOVE_DATA_DIR` for tests/qualification. Never access real applicant data. Respect the single SQLite writer. Employer-browser sessions do not survive restart.

## Read documentation and skills only when useful

Start from the assigned behavior. Search the specific route/symbol, inspect its Go owner and tests, then consult **only the relevant documentation section** when necessary. Source/tests outrank outdated prose. Do not pre-read all docs, all skills, or historical evidence.

| Task need | Read when needed |
| --- | --- |
| Product overview | `README.md` |
| Unfamiliar code path | Relevant section of `docs/CODEBASE_GUIDE.md` |
| Running app or isolated workspace | Relevant section of `docs/TRY_IT.md` |
| UI composition or styling | Relevant sections of `docs/design/RULES.md` and `docs/design/DESIGN_SYSTEM.md` |
| Selecting backlog work | Relevant `docs/BACKLOG.md` entry; verify current code/PR |
| Release or beta readiness | `docs/RELEASE_POLICY.md`, `docs/RELEASE_CHECKLIST.md`; specific procedure as needed |
| Prior qualification/decision | Only the applicable record in `docs/implementation/` |

Graft is optional when its index is current (`graft map`, `graft ask "..." --source`, `graft callers`, `graft grep`). Verify findings in source. Prefer `rg` for simple searches; do not generate/commit `graft/` output for an isolated task.

Skills live under `.agents/skills/<name>/SKILL.md` and **supplement**, never supersede, these rules. Load only skills relevant to the current task; delegated agents load their own. Each skill owns its procedure rather than repeating this file.

| Skill | Use for |
| --- | --- |
| `feature-slice` | Applicant-visible functionality crossing layers, not isolated styling/docs/refactors |
| `frontend-migration` | Replacing Go-rendered UI with React; required for Opportunities migration |
| `native-qualification` | Planning or claiming actual Wails GUI, native interaction or restart evidence |
| `durable-state-integrity` | Changed persisted writer, schema, document lifecycle, association or recovery contract |
| `browser-workflow-testing` | Changed/verified employer-browser sessions, controls or effects |
| `employer-continuation` | Changed authorized employer form transfer/fill or recovery, not public posting import |

## Verification and handoff

Exercise the smallest real path early, then focused existing tests and one material failure/recovery case. Broader baselines, when relevant, are `go test ./...` and `go test -tags desktop ./...` on supported macOS. Inspect `scripts/qualify-native-workflow.sh` and its fixture limitations before using it. A build, Go test, browser preview or Chromium fixture does **not** prove Wails GUI acceptance. For restart claims, quit and reopen the **same executable** on the **same synthetic workspace**, without reseeding.

Report observable behavior, modified paths, exact source revision/dirty state, checks actually run/results, native/browser observations when relevant, and **NOT VERIFIED** items. Distinguish **IMPLEMENTED**, **VERIFIED**, and **OWNER-ACCEPTED**. Stop at authorized scope; report optional cleanup separately.
