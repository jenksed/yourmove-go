---
name: durable-state-integrity
description: Protect YourMove Go persisted records, document bytes, associations, schema upgrades, and recovery when their owning write path changes. Do not load for pure presentation using unchanged Go storage behavior.
---

# Durable state integrity — YourMove Go

Protect the specific existing applicant record and relationships affected by the task; do not create generic storage infrastructure.

## Find the invariant

- Trace the real owning operation under `internal/app/`, its SQL/files and `internal/storage/` startup/path code. Consult only the relevant persistence section of `docs/CODEBASE_GUIDE.md` when needed.
- Name the invariant: job ID/source/manual edit, résumé identity/hash/bytes, job-document link, approved candidate data, saved answer/letter, curated evidence, or confirmed local submission record. Identify the plausible failure that could violate it.
- Preserve existing validation and revision conventions; **do not assume** this repository has YourMove Next's packages or a universal compare-and-swap contract.

## Change and verify the writer

- Keep a single Go owner. Use existing transactions and file patterns for related writes; don't introduce a parallel store, ORM, event log, or speculative recovery framework.
- Ensure failed writes never masquerade as saved results; preserve original values, IDs and associations on refusal or duplicate import. Database-only backup is insufficient for separately stored résumé files.
- Test a real write and its relevant negative path using existing Go test helpers or an explicitly synthetic `YOURMOVE_DATA_DIR`. For changed schemas, test a representative supported prior layout on synthetic data and validate identities/associations. Do not perform schema/migration qualification when schema is unchanged.
- If claiming applicant-facing durability in Wails, use `native-qualification` for the fixed-executable/same-workspace restart.

Report the actual writer, protected invariant, tests/results and any unverified partial-failure or migration scenario. Defer unrelated storage cleanup.
