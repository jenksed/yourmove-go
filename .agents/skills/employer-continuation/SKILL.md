---
name: employer-continuation
description: Implement or review an explicitly authorized YourMove Go Prepare-to-employer continuation, résumé/contact/cover-letter/saved-answer transfer, partial-result presentation, and manual recovery. Use only when changing employer assistance, not for the initial Opportunities frontend migration or public posting imports.
---

# Employer continuation — YourMove Go

This skill is **inactive for the initial React Opportunities slice**. Use it when the assigned task changes what YourMove sends to an employer form or what the applicant sees during that consequential action. It does not authorize any real employer effect.

## Find the existing implementation

Start at `POST /jobs/{id}/continue` in `internal/app/app_http.go`, then inspect `internal/app/application_continuation.go`, `browser_handlers.go`, `browser_session.go`, `cover_letter_transfer.go`, `application_answer_transfer.go`, and the owning `internal/application/browser/chromium/` paths. The actual workflow and qualification tests take precedence over donor documentation. Do not assume YourMove Next's Playwright backend, generated binding API or YMN task numbering exists here.

## Preserve the applicant's authority

- Present the exact saved and eligible résumé, approved contact facts, letter and answers as independent choices. Saving or recommending material is not approval to transmit it.
- Go rechecks destination, material identity and version/hash, active session and supported employer controls before each requested effect. Browser code performs the narrow authorized action; it does not infer consent or decide eligibility.
- Keep unsaved edits and stale/unmatched answers excluded unless the user explicitly resolves/saves them. Preserve manual employer values; report each attempted field/file independently and truthfully.
- An uncertain browser write stays uncertain; do not automatically replay it. Unsupported, login, CAPTCHA, and consent interactions remain manual. **Never auto-submit** the employer application. `POST /jobs/{id}/submit` remains a distinct, user-confirmed local history record.

## Deliver one controlled scenario

Use the current synthetic Go/Chromium qualification path, with a narrow supported provider/control subset and one meaningful refusal or partial case. Load `browser-workflow-testing` for actual fixture and test commands; load `native-qualification` only if asserting the real Wails user experience. Confirm zero employer-submit requests. Report observed versus expected field/file outcomes, unsupported manual steps, source/fixture identity and NOT VERIFIED claims. Do not broaden ATS coverage, rewrite the browser engine, or build a generic adapter framework within a targeted continuation fix.
