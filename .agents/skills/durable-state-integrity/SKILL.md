---
name: durable-state-integrity
description: Protect YourMove Go SQLite records, imported résumé bytes, candidate approval, saved answers/letters, job-document relationships, and local submission history when changing persistent behavior or moving a screen to React. Not needed for presentation-only work that leaves the data path unchanged.
---

# Durable-state integrity — YourMove Go

Preserve what the applicant has actually saved. Do not treat a successful HTTP response, optimistic React state, or a green mocked test as proof of a durable, correctly associated write.

## Inspect the actual writer

Trace the owning handler/operation in `internal/app/` and its SQL/file work through `internal/storage/sqlite.go` and `internal/storage/paths.go`. `docs/CODEBASE_GUIDE.md` maps current tables, files, and associations. This repository does **not** have YourMove Next's `internal/opportunity`, `internal/routing`, `internal/answerreuse`, or generalized durable-operation writer interfaces. Do not create them for a UI migration.

Identify which concrete item matters: job ID/status, selected résumé ID/hash, imported file bytes, applicant-approved profile fields, saved answers with inspection identity, cover-letter draft/version, curated evidence/answer-library copy, or explicit local submission history. Follow existing validation and revision behavior; do not assume all records share one compare-and-swap scheme.

## Keep the save/restart contract

- Go remains the sole authority for committed state. React state can be unsaved and disposable; report failed writes, avoid stale optimistic confirmations, and respect existing form conflicts/revision checks where present.
- Preserve existing IDs, foreign-key relationships, exact document bytes/hashes, user edits, approved flags and independent history. A database copy alone is not a complete backup when résumé files are external to SQLite.
- Use existing transactions and file-lifecycle patterns for related writes; don't add an ORM, second SQLite writer, event-log framework, or storage layer merely to support React.
- An open Chromium/employer session is transient and does not become durable as part of migration.

## Verify only the implicated invariant

Use a fresh synthetic directory via `YOURMOVE_DATA_DIR` or existing isolated test helpers; check the legacy-database import behavior documented in `docs/TRY_IT.md` before assuming an empty directory is harmless. Test one successful write and the realistic failure that could damage the affected data/association. Run the relevant Go tests (normally `go test ./...`; add `-race` or focused tests when justified).

If a persistent result is exposed through a new Wails screen, invoke `native-qualification`: save in native UI, quit, relaunch the same executable with the same workspace, and verify the exact value and association. When schema changes are genuinely needed, qualify the supported prior layout and migration path on a synthetic copy; don't touch a real workspace.

Report what writer and invariant were exercised, observed old/new identities, exact commands/results, native restart evidence if claimed, and unresolved failure. Do not expand into general storage refactoring.
