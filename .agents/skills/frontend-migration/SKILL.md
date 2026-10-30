---
name: frontend-migration
description: Move a YourMove Go candidate workflow from server-rendered Go templates and static JavaScript to React + strict TypeScript + Vite inside the existing Wails 3 desktop app, without rewriting Go application policy or losing user capabilities. Required for an authorized screen-by-screen frontend migration.
---

# Wails frontend migration — YourMove Go

Modernize one applicant workflow, retaining its **useful behavior**. The architecture is **Wails 3 + React/TypeScript/Vite + existing Go/SQLite**, not Tauri, Rust, or a Go application rewrite.

## First inspect the working user path

1. Locate the current screen in `templates/`, its `static/ui.js` or `static/inspection.js` interactions, the route in `internal/app/app_http.go`, the owning handler, and the SQLite/file effects. Inspect `docs/design/DESIGN_SYSTEM.md` and `docs/design/RULES.md` for behavior and composition, not as instructions to build a design system.
2. Record the user's action, current successful result, and materially important states: empty, loading, failed, unsaved/dirty, saved, and restart. Keep only those actually relevant to the slice. Note native dialogs, redirects, keyboard/focus, file transfer, and explicit consent when encountered.
3. Before adding a transport, determine what the current protected Go handler already provides. Use the smallest viable seam for the real React action: existing protected local HTTP, a narrow Go-owned JSON endpoint, or Wails binding where a native capability actually requires it. Do not assume generated bindings exist or copy YourMove Next's service tree.

## Migrate one thin vertical slice

- Introduce `frontend/src/features/<workflow>/` for screens, local state and feature-specific API use; add `shared/` only after real reuse. Keep one app entry and no empty placeholder features.
- Derive authoritative data from Go. TypeScript types describe the transport; do not duplicate Go validation/policy or store business records in localStorage. If using generated Wails bindings, generate from Go with tooling confirmed against the pinned Wails version, never edit generated output. If using explicit DTOs initially, add a narrow contract-alignment test/fixture where drift could silently break the slice.
- Keep Wails launching and embedding the new frontend without regressing unaffected Go-rendered paths or browser mode. Do not replace `desktop.go` with a sidecar or introduce a second unrestricted listener. Account for Wails WebView versus browser origin/transport behavior.
- Preserve the user's actual decisions and results, including stable record IDs and file associations, form dirty-state recovery, actual error messages, explicit save, and consent. Existing Go-rendered pages can remain during transition. A React screen is not required to operate with JavaScript disabled, but do not remove that existing fallback from unmigrated paths inadvertently.

## Acceptance for a migrated workflow

1. A real native Wails window opens the new screen and performs its read/write journey against existing Go storage in a synthetic workspace.
2. The UI shows the result of a committed Go write, not merely an optimistic success toast; a representative refusal or failed save remains recoverable.
3. After a full Wails quit, the *same app executable* reopens the *same workspace* and retains saved identities, associations and values. React-only unsaved drafts are not promised to persist.
4. An adjacent legacy page and affected browser-mode route still work, unless the authorized task explicitly retires them. Verify keyboard/focus and narrow-window usability for the changed view.

Use `feature-slice`, `native-qualification`, and `durable-state-integrity` when relevant; do not repeat their full procedures. Stop after one usable screen and revise folder/shared boundaries from observed needs. Do not start migrating Profile, Preparation, Applications, or History just because their directories are planned.
