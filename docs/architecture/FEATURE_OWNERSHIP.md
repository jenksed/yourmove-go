# YourMove Go — Feature ownership and migration boundaries

**Status:** Agreed architecture direction (October 30, 2026)  
**Applies to:** React workflow migrations, Go responsibility refactors, and decisions to extract Go feature packages.  
**Authority:** This is a placement/decision guide, not permission to start a task, change a schema, or modify unrelated code.

## Decision

Modernize YourMove Go on **two equally important axes**: React + strict TypeScript + Vite in the existing **Wails 3** application, and clearer Go ownership of feature behavior. Organize React UI by workflow. Organize Go by responsibility, **extracting feature packages only when they make the real code easier to change without splitting authoritative state or introducing disproportionate coupling**. Existing Go-rendered pages coexist by route until migrated.

A coherent `internal/app` implementation is a valid result. A new directory is not a result.

## Where changes belong

| Concern | Default ownership |
| --- | --- |
| Wails/native and local-browser entry points | Root `desktop.go`, `main.go`, `assets.go`; keep the Go module at root |
| React composition and routing | `frontend/src/app/` **when needed**; current entry may remain in `frontend/src/main.tsx` |
| React workflow | `frontend/src/features/<feature>/`: view, interaction, temporary drafts, feature-specific Go adapter |
| Reused React UI | `frontend/src/shared/` only after real cross-feature reuse |
| Go composition and transport | `internal/app/`: route registration, request validation at the transport edge, HTTP/JSON contracts, Wails host integration, request security, lifecycle, legacy responses |
| Go feature decisions | Cohesive functions in the current owning package; optionally `internal/features/<feature>/` if the extraction rule below is met |
| Persistent/workspace infrastructure | Existing SQLite initialization, transactions, document identity/files, and workspace behavior; preserve current owners |
| External integrations | Existing `internal/platforms/`, `internal/application/`, and other established concrete packages |
| Unmigrated UI | `templates/`, `static/` |

These are **ownership destinations, not a required tree**. Do not add `frontend/src/app/`, `frontend/src/shared/`, or `internal/features/` just to satisfy the map. Do not move `app` under `features` in either language.

### Dependency direction when Go extraction is justified

```text
React feature (presentation and disposable state)
        ↓ protected existing Go transport
internal/app (HTTP, security, composition, Wails, legacy)
        ↓ calls concrete operations
internal/features/<feature> (optional: cohesive Go-owned rules)
        ↓ uses existing authoritative dependencies
current storage / document / posting integrations
```

- `internal/app` may depend on extracted Go features. **Extracted features must not import `internal/app`**.
- Keep request/response contracts, route registration, origin checks, and Wails details at the composition/transport edge.
- Prefer explicit concrete dependencies. Introduce interfaces only for an existing, demonstrated consumer/seam, not to imitate a layered architecture.
- Preserve single-process, single-writer authority. Do not create parallel stores, competing mutexes, independent transaction owners, duplicated `Job` representations, or a generic service/repository framework.
- No Tauri/Rust conversion, second backend module/server, or parallel TypeScript business-rule engine.

## Go extraction decision

**Start from the changed capability, not the target directory.** Inspect actual callers, write paths, transaction scopes and affected tests. Extract only when all of these are true:

1. The operation is cohesive and has a clear owner independent of HTTP/Wails presentation.
2. Its callers and concrete dependencies can be traced without broad unrelated moves.
3. It can reuse the current writer, transactions, locks, IDs and coordination rather than reimplementing them.
4. The resulting dependency direction is simple and acyclic.
5. The change measurably reduces duplication, hidden coupling or navigation cost **now**.

If any fail, keep the operation in `internal/app` or its existing infrastructure package. Prefer a well-named function/file and shared operation over forwarding wrappers, adapter proliferation, package symmetry or arbitrary file-size targets.

If a cross-feature operation owns an atomic transaction or synchronized state, **keep that coordination together**; feature folders must not fragment it.

### Source-of-truth boundaries to trace before moving code

- Opportunities: single and bulk intake, URL normalization, duplicate/race handling, metadata lookup, saved manual titles/edits, stage and priority.
- Résumés/documents: saved document identity and exact bytes, selection, associations to an opportunity, revision/approval semantics.
- Deletion: dependent records, history-bearing refusal, and all-or-nothing behavior.
- Application history: applicant's explicit local declaration of manual submission, never employer-side automatic Submit.
- Employer/browser effects: separate, specifically authorized Go-owned operations; no silent retry or inferred consent.

Do not assume these behaviors can each acquire an independent writer simply because they appear under different feature names.

## Repeatable migration pattern

For every authorized screen migration (including the completed combined **YM-REACT-002 + YM-GO-002 Résumés** milestone):

1. **Name the applicant outcome** and trace its current UI → handler → Go decision → storage/external effect. Identify one material failure/recovery path.
2. **Choose the narrow React seam** using existing protected HTTP where appropriate; introduce only needed Go-owned JSON contracts or supported Wails bindings. Keep unaffected Go-rendered routes available.
3. **Make one feature deliverable** in `frontend/src/features/<feature>/`, with Go authoritative for validation, durable state and effects. Avoid duplicating rules in React or storing applicant records in browser storage.
4. **Review the touched Go ownership.** Resolve a concrete duplication/coupling problem where useful. Decide *extract / keep / defer* from actual dependencies. A no-extraction decision is acceptable when justified.
5. **Verify the real user path**, wire keys and error shapes, persisted identities/associations, representative refusals, adjacent legacy routes and actual Wails GUI when claiming native acceptance. For restart claims, reuse the same executable and synthetic `YOURMOVE_DATA_DIR` without reseeding.
6. **Stop at the accepted capability.** Record only material deferred architecture work; do not treat the target tree as a checklist. Do not silently broaden the task to other screens.

The Go improvement may be part of the same migration or a deliberately bounded follow-up when the dependency ordering requires it. Neither track should disappear: React migration is not complete engineering modernization if the touched Go path accrues new duplicate decisions, and Go cleanup does not substitute for a usable applicant interface.

## Current reference example: Opportunities

As reported for **YM-REACT-001** and **YM-GO-001** on `feat/wails-react-opportunities`:

- The React Opportunities UI lives under `frontend/src/features/opportunities/` and reads/writes through protected Go endpoints.
- `internal/app/opportunities_api.go` owns JSON transport; legacy form handlers still exist. `internal/app/opportunities_hosting.go` handles React hosting; actual route ownership is defined in source.
- The bounded YM-GO-001 refactor introduced `internal/app/single_intake.go` so both HTML and JSON single-URL handlers call one `savePostingURL` operation. It should remain a single authority for normalization, deduplication and manual-title precedence.
- The current shared `Job` model, SQLite writer/transaction coordination, résumé associations and history protections must be traced before any subsequent package extraction.
- The existence of `internal/features/opportunities/` is **not** required and has not been established by this decision.

This section is a **handoff snapshot**, not proof of local HEAD, a completed commit, or permission to continue YM-GO-001. Inspect source, diffs and tests to establish current state; update this example only when a material architectural decision changes.

### YM-002 completion record

YM-REACT-002 and YM-GO-002 were owner-accepted and integrated on `feat/wails-react-opportunities`. Accepted source and fast-forward integration commit: `ab2a7553ed4a2fed8245030f9da5cee4f0bfe560`. The React Résumés library supports document import and management, and shares Go-owned operations with legacy consumers; the native Wails workflow and persisted document/job association were qualified across restart. The isolated qualification workspace began without preexisting résumé records, and disallowed selection was covered by Go tests rather than a direct native UI attempt.

## Review questions when a boundary changes

Can another engineer identify where the user action enters, where Go decides, what actually writes, and how to verify it? Is there exactly one authority for mutable state and consequential effects? Are legacy and React transport responses consistent with their contracts? Does the new ownership reduce maintenance burden without expanding the scope or breaking cross-feature coordination?

If yes, proceed with the bounded change and relevant qualification. If not, keep the existing boundary and make the smallest useful improvement within it.

**Guiding rule: Prefer correct ownership and simple, traceable dependencies over package extraction.**
