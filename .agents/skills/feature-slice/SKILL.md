---
name: feature-slice
description: Implement an authorized, applicant-visible YourMove Next capability end to end across React, generated Wails bindings, Go use cases and SQLite or browser adapters where needed. Use when a feature crosses layers or must be demonstrated in the real native application; not for an isolated refactor, small styling change or documentation-only task.
---

# YourMove Next: feature slice

Deliver the smallest complete **applicant or operator outcome**, not a set of disconnected backend/frontend subtasks. Do not build supporting infrastructure without a concrete need in this slice.

## Before changing code

1. Follow `AGENTS.md` and `CONTRIBUTING.md`. Consult the four canonical documents under `docs/next/`, especially `RELEASE_BOUNDARY.md` for ownership and `CHANGE_CONTROL.md` for the **current** assignment/acceptance state. Historical descriptions and older instructions may lag behind accepted work; report conflicts instead of inventing authority.
2. Establish the explicitly authorized YMN task, source branch/commit, integration owner, expected user workflow, and what is **out of scope**. Scope approval is not start authorization. If no start authorization exists, inspect and propose; do not implement.
3. Name the observable end-user behavior in one sentence and identify one realistic success path and one material failure/recovery path. Defer other hypothetical improvements.
4. Locate a similar working slice using targeted source inspection. When available, use the repository's optional Graft navigation procedure from `AGENTS.md`; source is authoritative. Inspect relevant donor implementations only under the ledger's read-only and source-rights rules. Do not copy donor code without the required approval.

## Work through the real path

The reference example is YMN-005 résumé routing:

`frontend/src/components/ResumeSection.tsx` → `frontend/src/api.ts` → generated `frontend/bindings/.../routingservice.ts` → `routing.go` → `internal/routing/usecases.go` → `internal/storage/routing.go`, composed by `main.go`.

Use only the layers the requested behavior actually needs:

- **React/TypeScript:** applicant interaction, focus, loading/errors, recoverable unsaved drafts, disposable display state. Use the existing thin API adapters and injected component-test fakes. Never make React the authority for committed state, readiness, approvals, or browser effects.
- **Generated Wails contract:** define service methods/results in Go, keep exported service wrappers thin, regenerate/check the TypeScript bindings with `./scripts/bindings.sh`, and consume generated DTOs. Do not hand-edit generated files or create duplicate wire models.
- **Go use cases and policy:** own validation, revisions, decisions, consent/authorization and truthful typed outcomes. Cross-feature work calls the owning use case rather than writing another feature's tables.
- **Storage or effects:** extend the owning SQLite adapter/managed-file path only when needed; serialize shared migrations and composition changes with the integration owner. Browser commands go through Go-authorized policy and the existing trusted Playwright/CDP mechanics; an uncertain effect is never a confirmed success or a reason for automatic replay.

Implement one vertical path to a usable result before broad refactoring or generalized hardening. Do not introduce a new framework, generic repository layer, independent writer, orchestration layer or permanent state model to avoid using the existing one. Keep the no-Packet applicant flow and explicit applicant control of submission intact.

## Demonstrate, don't infer

1. Exercise the actual path early with invented data in an **explicitly marked synthetic workspace**. Never access a real applicant workspace, credentials or employer application under this skill.
2. Add focused tests for the new behavior and its material negative/recovery case. When changed contracts or writers justify it, test binding generation, revisions/transactions, restart, and stale-result behavior. Do not write unrelated test infrastructure.
3. Run relevant targeted checks during implementation, then `./scripts/check.sh` and `./scripts/build.sh` for an integrated native feature when executable in the environment. Binding drift is included in `check.sh`. Report an unavailable tool or failed check accurately.
4. If claiming native behavior, exercise the **real Wails executable** in the marked workspace; for durability claims, quit and reopen the same executable/workspace without reseeding. Component mocks, preview mode, CI compilation and test passes are useful supporting evidence, not substitutes for native behavior. Where native execution is unavailable, leave it **NOT VERIFIED** and give the operator the smallest precise check.
5. Inspect changed paths, relevant writer/effect boundaries and unexpected diffs. Stop expansion at the assigned feature; propose adjacent cleanup separately.

## Completion report

Report concisely:

- **Outcome:** what the applicant can now do, or where the path remains blocked.
- **Implementation:** branch/commit, changed layers and important boundaries.
- **Evidence:** exact checks/results and the native workflow actually observed (or **NOT VERIFIED**).
- **Limits:** real failures, unknowns, source-rights or effect concerns, and the smallest next action.
- **State:** distinguish **IMPLEMENTED**, **VERIFIED**, and **OWNER-ACCEPTED**. Do not self-accept, push, merge, release, start the next milestone or access real data without explicit authorization.

A slice is not complete merely because code compiles. Conversely, do not delay a working slice for speculative polish or verification machinery that protects no demonstrated risk.
