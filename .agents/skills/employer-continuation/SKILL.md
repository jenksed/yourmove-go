---
name: employer-continuation
description: Implement or review a bounded, explicitly applicant-authorized YourMove Go transfer from saved preparation to an employer form, including partial results and manual recovery. Not for job discovery, posting retrieval or the initial Opportunities migration.
---

# Employer continuation — YourMove Go

Change **one applicant-controlled Continue-to-employer behavior**, not a general ATS integration or browser-engine framework.

## Trace the existing operation

Start at `POST /jobs/{id}/continue` in `internal/app/app_http.go`, then only the involved code in `application_continuation.go`, transfer handlers, `browser_session.go`, and `internal/application/browser/chromium/`. Follow the actual active branch; a different YourMove project's generated bindings or Playwright bridge are not evidence of this implementation.

## Preserve the transfer decision

- Show the exact approved destination and independently selected saved résumé, eligible contact facts, letter, and answers; distinguish omitted/manual controls. Saving or recommending material is **not** permission to transfer it.
- Go rechecks destination, session, source/control identity, selected material and applicable revision/hash before each browser effect. React/HTML does not authorize the effect, and the browser driver does not decide applicant consent.
- Do not transmit unsaved/unapproved/stale material, replace the applicant's manual entries, automate Submit/login/CAPTCHA/consent, or treat a filled control as an accepted application.
- Record outcomes independently per field/file. An interrupted or unknown effect is not retried blindly; allow inspection and manual continuation.

## Demonstrate the narrow path

Use `browser-workflow-testing` for one actual authorized synthetic transfer and one material refusal/partial case, with exact observed outcome and zero employer submissions. Use `native-qualification` only when claiming the Wails interaction. Report what is supported, still manual, and NOT VERIFIED; do not silently broaden provider/control support.
