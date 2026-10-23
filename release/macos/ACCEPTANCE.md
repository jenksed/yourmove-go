# YourMove macOS candidate acceptance record — template

Copy this template into a *candidate-specific* sanitized acceptance record. Use synthetic applicant/job data and a separate workspace. Do not commit personal resumes, employer forms, private identifiers, credentials, cookies, or unredacted screenshots. See [release policy](../../docs/RELEASE_POLICY.md) and the [release decision](../decisions/v0.1.0.md).

## Exact artifact and environment

| Item | Value |
| --- | --- |
| Release/version/build | |
| Source commit SHA | |
| Candidate binary ZIP SHA-256 | |
| Matching source ZIP SHA-256 | |
| CI run and result | |
| macOS version / arm64 machine | |
| Chrome/Chromium version or absence | |
| Tested installation path | Local executable / locally extracted ZIP / actual browser download |
| Distribution audience | Controlled alpha / normal public |
| Tester and date | |

**Do not substitute a successful build or a test of a different SHA for acceptance of this exact candidate.** Mark every item PASS / FAIL / NOT TESTED / NOT APPLICABLE with brief sanitized evidence.

## Installation and identity

- [ ] Verify the distributed ZIP checksum, extract it, and confirm app name, icon, version/build, bundle ID, and arm64 code signature.
- [ ] Install/launch through Finder on a clean Mac/account without developer tools or local caches.
- [ ] For any real distribution, exercise an actual **browser-downloaded** ZIP and the normal macOS security/quarantine path.
- [ ] For a standard public download, Developer ID signature, accepted notarization, stapling, and Gatekeeper assessment pass. For a controlled unnotarized alpha, document the actual security warning and approved tester guidance without disabling security protections.
- [ ] Confirm local organization opens without Chrome or an AI connection.

## Persistent applicant data

- [ ] Import valid synthetic PDF and DOCX resumes; cancel a native import once.
- [ ] Create/edit an opportunity; save a resume association; quit and relaunch. Verify job, association, document hashes, and any prepared answers/letters remain correct.
- [ ] Export PDF/DOCX and a saved-letter PDF. Compare exact bytes; exercise Save, Cancel, and an invalid destination.
- [ ] Confirm local completion history is recorded only after explicit applicant confirmation; verify it survives restart and remains attached to the correct job/document.
- [ ] Before a patch upgrade, open an existing version workspace and verify non-destructive migration, job/resume links, answers, letters, and submission history.

## Employer authority and core workflow

- [ ] Preparation A: record exact provider/form/controls; reach a reviewable employer page, preserve manual edits, and document all unsupported fields.
- [ ] Preparation B: use a meaningfully different application or form path; repeat explicit scope, result observation, and manual fallback checks.
- [ ] Confirm saved local edits do **not** mutate an employer form until explicit authorization.
- [ ] Confirm only the authorized resume, letter and question values are transferred; no unauthorized overwrite or duplicate document upload.
- [ ] Confirm browser observation/attachment selection is never described as employer acceptance or submission.
- [ ] Confirm YourMove never automatically submits the employer application; the applicant remains the submitter.

## Recoverability and optional providers

- [ ] Test browser interruption, reload/return, native picker cancellation, and a recoverable employer-control failure; previously saved work remains intact.
- [ ] Confirm missing Chrome gives clear instructions and preserves local organization.
- [ ] When AI is advertised, verify consent, slow-generation behavior, error handling, locally saved drafts, and that unrelated app actions remain usable. Otherwise record the provider/workflow as excluded.
- [ ] Confirm known failures do not automatically replay uncertain employer uploads or values.

## Gate disposition and release approval

| Gate | PASS / RESTRICTED / BLOCKED | Evidence / exclusion |
| --- | --- | --- |
| Frozen candidate | | |
| Data integrity | | |
| Applicant authority/privacy | | |
| Core workflow | | |
| Distribution | | |
| Recovery | | |

- Overall result: PASS / CONDITIONAL / FAIL
- Known defects and excluded functionality:
- Release decision: SHIP / SHIP RESTRICTED / HOLD
- Release owner, date, and decision record reference:

A normal public installer cannot be marked accepted on the basis of a local ad-hoc signature or a ZIP extracted without downloaded Gatekeeper checks. A restricted release must state exactly what it does **not** promise.
