---
name: browser-workflow-testing
description: Verify YourMove Go employer-browser sessions, observation, or authorized form mutations through the actual Go-controlled browser path and synthetic fixtures. Not for public posting retrieval or ordinary UI tests.
---

# Browser workflow testing — YourMove Go

Test the **application's authorized browser operation**, not a separate browser script that imitates it. `AGENTS.md` defines applicant authority; this skill chooses effect-specific evidence.

## Choose only the necessary depth

- Go authorization/state rule: focused tests in the owning `internal/app/` or `internal/application/` package. Fake sessions do not prove real browser filling.
- Selector, browser control, upload, or effect changes: relevant opt-in synthetic Chromium/CDP qualification through existing `internal/app/browser_handlers.go`, `browser_session.go`, and `internal/application/browser/chromium/`. Inspect current test guards before running; use the specific implementation history only if its fixture instructions matter.
- Applicant-visible Wails continuation: add `native-qualification`. Do not require a full ATS/browser matrix for a narrow control change.

The active branch's actual source decides which browser backend exists. Do not import YourMove Next's `internal/browserproof`, `YMN_*` conventions, or unmerged cross-browser work as though already implemented.

## Check the effect

Exercise one authorized operation and a meaningful refusal/partial/uncertain case on a controlled synthetic employer fixture. Verify exact selected document bytes or observed field values where supported, no mutation to applicant-entered/manual fields, excluded data stays excluded, and **zero employer Submit operations**. Separate requested, dispatched, observed, partial and unknown results. A crash/timeout after dispatch is not permission to replay an uncertain effect.

Record source revision, fixture/backend, command/result, observed form values or file identities, submission count when exercised, and what remains unverified. Never use real credentials, applicant data or employer submissions for qualification.
