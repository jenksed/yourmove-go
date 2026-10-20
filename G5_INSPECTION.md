# G5 — Public application inspection and candidate context

## Purpose

YourMove's **Go** application is the product. The prior TypeScript implementation is an R&D reference for employer-specific failure cases, identity, field observation, document upload and recovery. **Do not port its UX, generic automation runner or question bank as-is.**

G5 introduces platform-separated, read-only application inspection and a local, user-approved candidate profile. It does not write to employer forms, attach documents, launch an automated employer browser, click Next, or submit applications. These must remain separate, separately authorized capabilities. The work is a useful independent milestone, not a representation of completed autofill.

## Using G5

1. **Candidate profile** in the sidebar. Enter verified facts and save. An approval checkbox must be explicitly checked on each save to authorize local mapping suggestions; saving without it revokes approval. Approval never authorizes employer transmission.
2. Open an active job. In **Prepare**, select your resume, then use **Inspect application**.
3. On the inspection page, click **Inspect public requirements**. This makes only a bounded *GET* request to a provider's documented public API. The target comes from the saved posting, never from a form URL parameter.
4. The inspection page shows provenance, actual public question definitions, field types, choices, sensitivity, exact profile matches and possible/uninspected steps. You can revisit these persisted observations after restarting. Repeating unchanged discovery updates the timestamp without multiplying records.
5. Open **Utilities → Discovered requirements** to see question-label frequencies across the latest public inspections. These counts are not proof of answer equivalence.

### Provider behavior

- **Greenhouse**: `GET https://boards-api.greenhouse.io/v1/boards/{board}/jobs/{id}?questions=true`. Parses regular, location and compliance question groups. Alternative file/text controls remain distinct fields. Demographic groups with a different shape are flagged for manual review rather than silently dropped and declared complete. Public question definitions do **not** establish live employer form readiness.
- **Ashby**: documented public board API validates a posting and resolves its published application destination when available. **It does not expose application question schemas**. The result stores zero invented fields, a partial finding and explicitly anticipated/unknown navigation steps. Browser-only GraphQL protocols from TS R&D are not adopted as a supported product API.
- **Other platforms**: public application question discovery not yet supported. Existing posting metadata extractors and manual employer-site launch work unchanged.

## Packages and technical boundaries

- `internal/application/schema`: persisted field/step contracts and deterministic fingerprint. These objects contain no applicant answer values.
- `internal/application/providers/greenhouse`: bounded, public, read-only ATS API adapter.
- `internal/application/providers/ashby`: bounded public board adapter, explicit partial result.
- `internal/application/discovery`: provider dispatch, fixed-host allowlist, context budget, injectable inspector for tests.
- `internal/application/flow`: conservative step-action classification. `Continue` is not proof of a safe click; potentially transmitting actions and unknown steps always require a person. It currently **classifies only**; it has no browser driver or click operation.
- Root `application_inspections.go`: SQLite persistence and strict, non-sensitive mapping of approved basic candidate facts to exact Greenhouse field IDs. No generic fuzzy matching or answers are persisted.
- Root `inspection_handlers.go`: browser/Wails routes and templates. The existing `withDesktopPostRedirects` allows the inspection route. The native resume picker and default external-browser opener are unchanged.

## Migrations and safety

Additive tables: `candidate_profile`, `application_inspections`, `application_fields`, `application_steps`. The job, role-family, resume, selected document and submission models do not change. Deleting a job also deletes its inspections and dependent fields/steps; it still does not delete shared resume files. Reusing a canonical URL is unaffected.

Candidate facts reside unencrypted in YourMove's existing private local SQLite database. Backups containing `jobs.db` include those facts. Store no applicant answer values, credentials, consent choices, or document bytes inside inspection records. No employer form POST is implemented. Calls fail closed on invalid provider URLs, cross-origin local POSTs, non-200 remote responses and oversized JSON.

## Browser/multi-page next milestone

To support actual Ashby multipage flows, introduce a **separately owned browser session** and a page-observation interface that records current URL, frame identity, visible controls, current filled values, and buttons. Each navigation or conditional change generates a new observed step. A transition is allowed only after verifying the target, packet/document IDs, potential data transmission and whether it could submit. Unknown, authentication, consent, CAPTCHA and final-submit controls always pause for the operator. Reopen/restart never claims that browser values survived. Resume parser changes trigger a fresh inspection; uncertain attachments are not blindly replayed.

Only when this is verified should we add authorized profile filling and resume file delivery. The future question bank should derive its taxonomy from actual persisted API and browser observations, including step and provider context.

## Qualification

```sh
go test ./...
go test -tags desktop ./...
go test -race ./...
go run -tags desktop .
```

Test with an isolated data folder: `YOURMOVE_DATA_DIR=/absolute/test/path go run -tags desktop .`.

**Synthetic provider tests** use injected HTTP transports to assert GET method and fixed origin, preserve multiple alternatives under Greenhouse questions, and refuse invented Ashby fields. Pure flow tests reject automatic step and submission actions. SQLite regression tests cover approval revisioning, repeat inspection idempotence, evidence deletion, cross-origin form protection and the native redirect shim.

This container can run pure Go provider tests and compile/type-check the project with isolated dependency stubs, but it cannot perform native macOS/Wails or actual SQLite integration verification with the project's Go 1.27.2 dependencies. Run the commands on macOS before merging. Real employer discovery must be checked separately, without candidate transmissions.
