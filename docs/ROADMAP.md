# YourMove initial release roadmap

**Planning baseline:** October 23, 2026. These are *targets for readiness reviews*, not promised download dates. An earlier release is allowed whenever its applicable gates pass. A failed gate moves the shipment, not the safety standard.

| Release | Proposed target | Audience | Outcome to prove |
| --- | --- | --- | --- |
| **v0.1.0** | **November 6, 2026** | Small invited Apple Silicon/macOS 26+ beta | Install → prepare → personally submit → record with intact data |
| **v0.1.1** | **November 13, 2026** | Existing beta testers | Remove defects and friction observed in v0.1.0 |
| **v0.2.0** | **December 4, 2026** | Wider macOS beta, conditional | Make the second and later applications meaningfully easier |

## v0.1.0 — first usable desktop journey

**Primary acceptance:** An invited job seeker can install YourMove, save/select the correct resume, prepare answers and an optional letter, inspect the employer form, submit it themselves, and accurately record completion. Resumes, answers, letters, and submission records persist and retain correct associations across restart.

Initial matrix: Apple Silicon, macOS 26+, with installed compatible Chrome/Chromium for employer-form assistance and **only evidence-qualified employer workflows**. AI is optional; provider work that blocks the rest of the application must be fixed, isolated, or excluded from beta claims.

A locally ad-hoc-signed candidate exists, with successful native testing and macOS packaging CI. It is not a qualified normal public download. Remaining release work is artifact-specific acceptance, supported-workflow qualification, distribution choice, and clean downloaded-install verification. A standard low-friction installation also needs Developer ID signing and notarization.

**Explicit non-goals:** Windows/Linux, Intel/older macOS, SaaS integration, general ATS support, auto-submission, auto-update, and a broad architectural rewrite.

## v0.1.1 — stabilize the actual user path

Reserve this release for faults confirmed by tester evidence: incorrect/missed field behavior, lost context, document or answer persistence, native export, confusing completion, failed installation, and blocked continuation. Prioritize defects by whether they prevent or materially delay completing an intended application. Verify upgrade preservation from 0.1.0 before shipping.

No feature-growth requirement. If v0.1.0 hasn't shipped, this date is revisited rather than creating an artificial stabilization release.

## v0.2.0 — repeat-application differentiation

Improve reuse of validated answers, saved evidence, resume choices, and continuation so subsequent applications require less repeated typing and fewer repeated decisions. Evaluate with observed workflow time, manual corrections, and blocked attempts rather than test execution speed alone.

Additional ATS coverage or another platform enters scope only if it directly serves this outcome **and passes its own qualification gates**. A larger audience is conditional on actual beta results.

## Operating rhythm

| Checkpoint | Decision |
| --- | --- |
| Monday | Choose one release outcome and the current top blockers |
| Tuesday–Thursday | Build, exercise the real workflow, correct observed failures |
| Friday | Review candidate evidence and decide SHIP, SHIP RESTRICTED, or HOLD |

If the gates pass Tuesday, ship Tuesday. If they fail Friday, hold or restrict the advertised scope. Document the exact decision in `release/decisions/`, linked from the [release policy](RELEASE_POLICY.md) and [execution checklist](RELEASE_CHECKLIST.md).
