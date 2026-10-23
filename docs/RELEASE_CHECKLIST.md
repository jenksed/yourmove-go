# YourMove release readiness

**Status reviewed:** October 23, 2026. **Initial target:** v0.1.0 invited macOS beta, Apple Silicon/macOS 26+.

The [macOS packaging PR #4](https://github.com/jenksed/yourmove-go/pull/4) merged into `main` at commit `2d2f16b8965bdea6aad07229e2edba06247ac5e2`. [CI on that merge](https://github.com/jenksed/yourmove-go/actions/runs/39908694684) completed successfully, including the bundle build/verification job. No public GitHub release was published at this review.

Read the [release policy](RELEASE_POLICY.md), [roadmap](ROADMAP.md), [runbook](RELEASE_PROCESS.md), [v0.1.0 notes](releases/0.1.0-macos.md), and [artifact decision](../release/decisions/v0.1.0.md).

## Locally established

- [x] Go 1.27.2 and macOS 26.5.2 / arm64 environment verified by the maintainer.
- [x] Both Go suites passed locally (normal and `desktop` tags).
- [x] Native Wails `.app` built and checked with a local ad-hoc signature.
- [x] New brand icon included in the generated application bundle.
- [x] Maintainer tested application behavior and restart persistence using an isolated copy of an existing job-hunt workspace.
- [x] Copied workspace file inventories, key record counts, and SQLite integrity checks passed.
- [x] `doctor.sh` and `prepare.sh` are included in the merged packaging system; maintainer reported successful release preparation including ZIP and source archives.
- [x] Packaging PR #4 merged; CI on merge passed, including the actual app-bundle packaging verification.

**Boundary:** The locally exercised candidate and CI packaging checks establish a reproducible development build. They do **not** establish browser-downloaded normal installation, a complete provider matrix, notarization, or candidate-specific public beta approval. The copied workspace contains real user data; future shareable acceptance records must use synthetic test data.

## v0.1.0 gates still requiring evidence

- [ ] Freeze exact release commit, `CFBundleVersion`, candidate ZIP SHA-256, supported functionality and distribution audience.
- [ ] Complete a candidate-specific synthetic [macOS acceptance record](../release/macos/ACCEPTANCE.md) with pass/fail evidence.
- [ ] Verify at least two distinct application-preparation journeys to a reviewable employer form; document exact ATS/controls and manual fallbacks.
- [ ] Verify preserved documents, saved answers, letters and completion associations across restart and expected failure recovery.
- [ ] Confirm explicit scope, no unintended overwrite/upload, no automatic submission, and controlled browser fixture results.
- [ ] Verify absent/unconfigured Chrome and AI behavior; exclude any optional generation provider that blocks routine use.
- [ ] Decide whether distribution is a controlled unnotarized technical alpha or a normal public installer.
- [ ] For normal public installation: Developer ID signing, accepted notarization, stapling/Gatekeeper, and clean downloaded-install check.
- [ ] For any offered alpha: actual browser-downloaded ZIP validation, accurate security warnings, and explicit owner approval.
- [ ] Record SHIP / SHIP RESTRICTED / HOLD with evidence in `release/decisions/v0.1.0.md`.
- [ ] Only after approval: publish app/source archives, checksums, platform requirements, installation guidance, and feedback links.

## Ship/hold rule

Do not ship an artifact that may lose or misassociate records, exceed the applicant's authorization, disclose sensitive information, or fail to install under its advertised distribution path. Unqualified optional features or ATS providers can be removed from the claims, but a broken core workflow cannot be relabeled as a pass.

The target date in [ROADMAP.md](ROADMAP.md) is a planning checkpoint, not a release promise. Early shipment is allowed when the applicable gates are met; otherwise restrict the scope or hold.
