---
name: durable-state-integrity
description: Protect YourMove Next's SQLite records, managed résumé files, revisions, associations, migrations and recovery when implementing or reviewing persistent-state changes. Use for changes to Go-owned writers, storage adapters, schema, file lifecycles, deduplication, durable operation receipts, backup/restore or restart semantics; not for presentation-only edits.
---

# YourMove Next: durable state integrity

Protect the applicant's **existing work and the truth of what was committed**, not merely a successful new write. This skill supplements `AGENTS.md`, the current authorized YMN task, and the writer map in `docs/next/RELEASE_BOUNDARY.md`; it grants no permission to begin another task or inspect real candidate data.

## Locate the owner and invariant

1. Read the relevant `docs/next/TASK_INDEX.md` acceptance conditions and current `CHANGE_CONTROL.md` state. Identify the durable record, its sole owning Go use case, the concrete storage adapter, and every related ID, revision, file, hold or history association. Start from the actual call path; use targeted source inspection.
2. Reuse existing boundaries: `internal/opportunity`, `internal/candidate`, `internal/documents`, `internal/routing`, `internal/preparation`, `internal/answerbank`, `internal/careerevidence`, and `internal/answerreuse` call their owners; `internal/storage` implements their SQLite/file ports. Do not let a second feature write another owner's tables or introduce a generic repository, event log, ORM, or parallel state store.
3. State the **one specific invariant** the change must preserve, plus the meaningful failure or stale-state case. Keep the test scope proportional to the change.

## Preserve the write contract

- **Concurrency and identity:** Go owns validation, stable IDs and compare-and-swap revisions. Check existence/expected revision and mutate related rows atomically in the owning adapter. Return current revision or an actionable failure; never silently overwrite later edits or convert a failed write into a success message.
- **Transactions and associations:** Use existing SQLite transaction patterns for multi-row decisions, aliases, holds, approvals, immutable snapshots, revisions and receipt records. Verify relevant foreign keys, deletion/restore behavior, and independent histories. A saved Answer Bank copy or reviewed evidence revision is not a live pointer to mutable job text.
- **Managed files:** Follow `internal/documents` and the storage adapter's staged-write, verified-byte/hash, revision and deletion-hold conventions. Confirm exact document identity and stored bytes before dependent operations. Handle DB/file partial failure without dangling references or silent data loss; cleanup and orphan outcomes must be truthful.
- **Multi-step local work:** For combined job/Answer Bank saves, follow `internal/answerreuse/coordinator.go`, `internal/preparation/recovery.go` and their existing intent/receipt contracts. Reconcile the **original flow ID** before retrying; a successful leg must not execute twice, and an unknown leg must remain unknown until checked. Treat browser/AI effects under their separate authority rules; never replay an uncertain employer effect as a local retry.
- **Schema and workspace:** Changes to `internal/storage/storage.go` initialization or `migrate*` steps require coordinated ownership of schema/composition. Respect `PRAGMA user_version`, recognized layouts, foreign-key/integrity checks, and refusal of corrupt, unfamiliar or newer schemas. Do not repair a real workspace by guessing a migration or removing its writer lock.

## Verify the relevant failure, not a new framework

1. Use a newly created, **explicitly marked synthetic workspace** for tests. Never reseed an existing fixture, mutate a live native app's SQLite database behind its writer, remove locks, or copy real applicant data. Reuse the test helpers already in `internal/storage/*_test.go`.
2. Test the successful write and the relevant negative path: stale revision, duplicate/concurrent request, deleted/changed source, interrupted partial work, failed file/transaction step, corrupt receipt/snapshot, or incompatible migration. Select cases that could actually break the changed invariant; do not manufacture an exhaustive matrix.
3. For changed schema or stored associations, exercise at least one **representative supported earlier synthetic schema**, retain IDs/bytes/revisions/links, confirm the latest layout and SQLite integrity, and verify restart. YMN-002 backup feasibility is not blanket semantic migration coverage for later features.
4. Run focused Go tests with race checking, for example `go test -mod=readonly -race -count=1 ./internal/storage ./internal/answerreuse`, adjusting packages to the changed owner; then `./scripts/check.sh` for an integrated slice. Run `./scripts/build.sh` when a native build is relevant. Report unavailable tools or checks rather than claiming a pass.
5. When the claim concerns an **applicant-facing persisted outcome**, load `.agents/skills/native-qualification/SKILL.md` and prove the real Wails save, quit and same-executable/same-workspace reopen. Unit, SQLite, preview, and component tests support but do not replace native evidence. For employer-browser effects, also load `browser-workflow-testing`.

## Completion handoff

Report the invariant and owning writer, changed files/records, exact tests and results, migration/rollback or partial-result observations, and any **NOT VERIFIED** risk. Separate **IMPLEMENTED**, **VERIFIED**, and **OWNER-ACCEPTED**; do not self-accept, migrate real data, push, merge or release without explicit authorization. Stop once the assigned data-integrity risk is covered—no speculative persistence redesign.
