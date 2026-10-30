---
name: browser-workflow-testing
description: Test YourMove Go's real Go-owned Chromium/CDP employer browser assistance and associated Wails workflow against controlled synthetic fixtures. Use for browser sessions, employer controls, file/answer transfer, or changes that claim form-fill behavior; not ordinary React layout work.
---

# Browser workflow testing — YourMove Go

The relevant implementation is `internal/app/browser_handlers.go`, `internal/app/browser_session.go`, transfer/continuation handlers and `internal/application/browser/chromium/`. **Do not use YourMove Next's Playwright bridge, `internal/browserproof`, or `YMN_REAL_BROWSER` tests**: those are different code paths.

## Select the smallest useful level

- UI layout, focus and copy: view/component tests or controlled browser rendering. This does not prove employer mutations or native Wails behavior.
- Go permission and persistence: focused `internal/app/` and `internal/application/` tests. Fakes do not prove live Chromium execution.
- Actual controlled browser effect: existing opt-in qualification tests using invented job/employer fixtures and the Go-owned Chromium/CDP implementation. Inspect test guards and their environment variables before running. `docs/implementation/application-workflow-stabilization/HANDOFF.md` documents examples such as `YOURMOVE_QUALIFY_BROWSER=1` with `-tags qualification`, but check current test source before use.
- Applicant-visible desktop claims: also load `native-qualification` and exercise the actual Wails window.

## Preserve consequential boundaries

Follow the actual path from explicit Go-authorized applicant action to supported browser control, then observed result. Confirm the intended employer destination, selected saved material and relevant revision/hash, current session, and exact control eligibility before mutation. Keep unsaved and unapproved text out. Preserve manual employer-field content. Never automate Submit, login, CAPTCHA or consent. An employer-side fill is not employer acceptance.

Inspect exact observed file bytes or field values where the fixture supports them, distinguish filled / skipped / partial / unknown, and check zero employer submissions for assistance-only tests. A timeout or crash after dispatch is not proof that nothing happened: do not silently replay or switch to another automation backend. If unsupported, leave manual recovery available.

Report source revision, exact synthetic fixture, Go/native/Chromium execution level, test command and result, observed transfer result, zero-submit evidence when exercised, and limitations. Do not run against real employer URLs or applicant credentials under a qualification skill. Avoid building a new browser-testing framework merely to obtain evidence.
