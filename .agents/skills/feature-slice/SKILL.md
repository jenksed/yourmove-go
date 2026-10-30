---
name: feature-slice
description: Implement an authorized YourMove Go applicant-facing behavior end to end through its existing Wails/Go UI, HTTP handlers, SQLite, or browser integration. Use for cross-layer workflows, including the new React Opportunities slice, not isolated styling or documentation edits.
---

# Feature slice — YourMove Go

Deliver one meaningful applicant outcome through the real application, not a set of unconnected scaffolding tasks.

## Find and bound the path

1. Confirm the current owner-authorized task, `main`-based branch/worktree, clean/owned changes, user action and out-of-scope work. Consult `AGENTS.md` and `docs/CODEBASE_GUIDE.md`; read the relevant `docs/BACKLOG.md` entry if planning from the backlog.
2. Follow the existing feature from view (`templates/` and `static/` for legacy UI) to `internal/app/app_http.go` route and handler, application decision, `internal/storage/` or browser adapter, and visible result. Inspect actual source before choosing a new interface.
3. Describe one normal path and one material failure or recovery case. Implement the smallest slice that establishes the new behavior. Avoid broad Go package moves and generalized UI/API frameworks.

## Respect responsibility

- Wails supplies a native host; it is not the authority for business decisions.
- React, where introduced, handles interaction, unsaved form state, accessibility and loading/error presentation. It cannot decide what is committed, transferable, or submitted.
- Existing Go handlers and workflow operations own validation, decisions and saved state. When adding a UI seam, prefer reuse of existing behavior; keep HTTP protection and Wails compatibility intact.
- SQLite and managed document files preserve IDs, relationships and historical records. Browser operations remain separately authorized, with unknown results reported as unknown.

## Prove the outcome

Use a synthetic workspace, existing focused Go tests, and a real action through Wails early enough to discover integration faults. For frontend replacement, also load `frontend-migration`. For changed persistent behavior, load `durable-state-integrity`. For any native completion claim, load `native-qualification`. For employer-side effects, load `browser-workflow-testing` and (if implementing transfer) `employer-continuation`.

A pass in `go test ./...`, `go test -tags desktop ./...` or a browser/Chromium probe supports but does not replace the actual native applicant action. Report what changed, the exact evidence, the first observed failure, and what remains unverified. Stop after the authorized capability; do not self-accept, merge, or grow into a general rewrite.
