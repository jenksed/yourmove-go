---
name: feature-slice
description: Implement a bounded, applicant-visible YourMove Go workflow spanning UI, Go decisions, and state or integrations. Use for real cross-layer capability work, not documentation, isolated styling, or standalone refactoring.
---

# Feature slice — YourMove Go

Own **one complete applicant outcome**, rather than separate frontend, backend, and test scaffolding milestones. The global authorization and safety rules are in `AGENTS.md`.

## Locate the real path

1. State the exact applicant action and visible successful result; name one plausible refusal/recovery case.
2. Trace the existing screen/action to its registration in `internal/app/app_http.go`, owning Go handler/decision, and relevant SQLite/file or integration effect. Use the matching section of `docs/CODEBASE_GUIDE.md` **only if ownership is unclear**.
3. Identify the minimum source files and current behavior to preserve. Distinguish a missing user capability from a nice-to-have structural cleanup.

## Implement and demonstrate

- Implement the real UI → Go → saved/result path before generalizing types, packages, tests or infrastructure.
- Keep Go authoritative; use the existing writer/effect boundary and avoid introducing another policy or persistence owner.
- Show committed state or truthful failure in the interface; don't count an optimistic screen update as completion.
- Test the relevant successful action and one material negative case using synthetic data. Add only focused regression tests needed to protect the change.

Load `frontend-migration` for a React replacement, `durable-state-integrity` when a write/storage contract changes, `employer-continuation` plus `browser-workflow-testing` for employer transfer, and `native-qualification` when planning or claiming a native result. Follow the specific skill instead of duplicating its procedure here.

End with observed user behavior, changed code, evidence, unresolved limitations, and any optional cleanup left for later.
