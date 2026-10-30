---
name: employer-continuation
description: Implement or review YourMove Next's applicant-controlled Continue-to-employer workflow (YMN-012), including exact résumé/contact/saved-answer transfer, qualified Playwright/CDP control adapters, consent, partial results, and manual recovery. Use for employer form filling or transfer architecture, not ordinary job-posting retrieval or browser-only testing.
---

# YourMove Next: employer continuation

Deliver one usable **Continue** path from a saved Opportunity to an employer form. Keep the applicant in control and leave unsupported work in the same browser for manual completion. This skill is for **implementation and review**, not general browser QA; load `browser-workflow-testing` for browser execution evidence and `native-qualification` when claiming the Wails user journey. Use `feature-slice` too when the assignment crosses UI/Go/storage layers.

## Establish the authorized slice

1. Check the **current** `docs/next/CHANGE_CONTROL.md` assignment and the YMN-012 scope/acceptance in `docs/next/TASK_INDEX.md`; follow `RELEASE_BOUNDARY.md` and `AGENTS.md`. Approved scope is not start authorization. If no specific task start is authorized, inspect and propose only.
2. Inspect actual source before designing changes. Existing seams include `internal/browserproof/{service.go,backend.go,cdp.go,bridge/driver.mjs}`, `browserproof.go`, `internal/routing/usecases.go`, `internal/preparation/usecases.go`, and their owning stores. **YMN-003 is a bounded synthetic proof**, not a general ATS filler: its bridge supports only `launch`, `observe`, `execute`, `close` on its owned fixture, and its one-operation/session policy must not be bypassed to simulate product readiness.
3. Name the applicant outcome, one supported employer/provider/control subset, and a meaningful failure/recovery path. Reuse qualified mechanisms; do not copy donor source without the rights, provenance, and explicit permission required by `CAPABILITY_LEDGER.md`.

## Follow the actual applicant flow

- Start from the saved Opportunity's contextual **Continue** action. Read the applicant's **explicitly selected** résumé through Routing/Documents, saved answers through Preparation, and eligible approved contact facts through Candidate. Do not infer consent from saving, résumé recommendation, answer-bank suggestions, or local readiness.
- Go computes eligibility and prepares the exact outgoing scope: Opportunity/job identity, verified destination and current observed control identities, backend, session, operation ID, selected document version and verified bytes/hash, saved answer/fact values and revisions, and per-control intended effects. Keep unapproved, unsaved, stale, unsupported, or unavailable material out of the transfer. Revalidate affected scope at dispatch; a harmless tab change or display-name change must not demand a new approval.
- Present the **actual outgoing material, destination, and omitted/manual fields** at the consequential action. Obtain explicit applicant authorization for that scope. No Packet/Accept Packet step, second answer editor, mandatory AI/provider/letter dependency, or automatic employer submission.
- Execute only narrow, Go-authorized commands through the existing Go-owned Playwright Chromium bridge (primary) or explicitly chosen direct Chromium/CDP alternative. React never calls browser drivers, evaluates employer scripts, decides permission, or manufactures transfer success. The driver reports observations; Go decides and persists each outcome.
- Bind effects to their session/backend/destination/material/control identity. Persist authorization **before** dispatch and use an atomic authorized-to-dispatch transition to reject duplicate clicks and concurrent invocation. Read-only observations are not grants. Keep the one owning employer-operation writer; other features interact through their owning use cases, not direct table writes.
- Treat résumé upload, contact fields, and saved answers as **independent results**. Capture observed exact values and file bytes when qualified, and report filled, excluded/manual, refused, partial, or unknown honestly. Preserve any employer-page values the applicant entered manually. A successful command alone does not prove the expected form state; filling is never submission or employer acceptance.
- A cancel/timeout/bridge crash/browser exit after dispatch may leave an effect **unknown**. Reconciliation observes; it never replays. Do not automatically retry the same upload, switch from Playwright to CDP, or restore a prior live page/consent. Inspect the original operation, require fresh session and scope authorization when necessary, and provide a usable manual continuation path if the browser or site is unavailable.

## Qualify only what is offered

- YMN-012 initially scopes **qualified Ashby, Greenhouse, and Lever control subsets**. Record support separately per provider, control type, and **Playwright versus direct CDP**; do not presume parity or universal form coverage. Public posting/discovery support, including other ATS integrations, is **not** evidence of employer-fill support.
- Exclude automated employer **Submit**, CAPTCHA/login, privacy/consent decisions, speculative selectors, cross-site navigation, unsupported controls, Firefox/WebKit, and public listing discovery through an employer session. Keep all these actions manual unless a separate owner decision changes the scope.
- Use invented data and explicitly marked synthetic workspaces/owned employer fixtures only; never run against real applicants, credentials, or live employer destinations under this skill. Exercise a successful transfer and a stale/denied/partial or interrupted case through the **actual Go policy and browser adapter**, not just component mocks. Assert exact observed values/file bytes, preservation of manual fields, non-effects on omitted fields, and **zero submit requests**. When claiming desktop behavior, qualify the real Wails window with `native-qualification`.
- Run relevant focused checks, binding generation/drift and `./scripts/check.sh` / `./scripts/build.sh` when appropriate. Delegate detailed browser/test setup to the existing testing skills; do not add a second runner, generic browser engine, packet model, or speculative adapter framework.

## Report and stop

Give the smallest reviewable handoff: implemented applicant outcome; source branch/commit; qualified provider/control/backend matrix; actual test environment and observed Go/browser/native receipts; unsupported/manual cases; unknown/recovery handling; and remaining **NOT VERIFIED** items. Distinguish **IMPLEMENTED**, **VERIFIED**, and **OWNER-ACCEPTED**. Never claim YMN-012 complete from the YMN-003 fixture proof, auto-mark acceptance, push, merge, release, or begin another milestone without owner authorization.
