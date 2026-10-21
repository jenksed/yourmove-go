# Application experience implementation — 20 October 2026

## Product progress

Implemented saved cover-letter delivery, application-specific questions, verified native-control answer transfer, optional exact reusable answers, and conditional continuation in the existing Prepare workspace. This extends Increment A; it does not replace its contact/resume path or declare universal Ashby compatibility.

A user can now:

1. Use the existing approved-contact and exact-resume transfer.
2. Select a saved letter through the existing draft editor/version links and authorize its exact saved content for the open application. Return navigation preserves the selected draft, including an older draft when a newer one exists.
3. Attach a deterministic PDF to a unique cover-letter upload control, or explicitly choose a labeled native cover-letter text control. Letter-only transfer does not arm the resume or contact operations again.
4. Enter discovered questions inside Prepare, save locally, leave, and return. Qualified approved profile facts are prepared without retyping. Sensitive/unsupported/ambiguous questions remain manual.
5. Authorize the ready saved answers. Supported text inputs, textareas, native selects, native checkboxes and qualified radio groups are revalidated and independently observed after interaction.
6. Optionally mark an answer reusable inside its existing editor. Another application at the same employer can prepare it only when provider identity and the complete question definition match exactly. Application overrides do not change the reusable original without an explicit selection.
7. Answer newly exposed conditional questions and transfer again in the same session. Existing correct fields, manual corrections and previously selected documents remain in place. Newly discovered questions have a refresh link; status polling avoids reloading over unsaved edits.
8. Review individual outcomes and finish personally in Chromium. No operation submits an employer application.

The initial resume/contact action, letter action and ready-answer action still have distinct, narrowly scoped approvals in the same workspace. They are not a single combined authorization. This keeps the existing resume consent path intact, but further consolidation remains a UX improvement.

## Engineering changes and invariants

### Reused and simplified

- `browser_session.go`: existing session host/watcher, cancellation, lifecycle and state polling. New approvals belong only to the current session and exact destination.
- `internal/application/browser/chromium/attachment.go`: one shared file-selection implementation for resumes and cover letters. Purpose selection, existing-file preservation and format checks precede mutation. The cover letter is hashed before selection and the browser File bytes are hashed after selection. Application ownership is checked again near mutation and verification.
- `internal/application/browser/chromium/chromium.go`: `lockContext` replaces repeated cancellable-lock loops across observation, contact, answer and document operations. There is still one serialized browser writer.
- `internal/application/browser/inspect.js`: the read-only structural observation is also the answer transfer's identity contract. Ashby field-entry IDs and question titles are preferred; qualified native radio groups are observed together. Nested control contents are removed from cloned labels so select options/default response text do not become question wording. Sensitive labels abstain conservatively.
- Existing Phase 0 localhost authority, trusted Wails entry point, reviewed resume ID/hash submission confirmation, Go POST forms and desktop redirect adapter remain intact.

### Focused new implementation

- `cover_letter_document.go`: deterministic, bounded, dependency-free PDF text export and saved-body SHA-256. It renders saved text without AI, pagination timestamps or hidden content substitution.
- `cover_letter_transfer.go` and `chromium/cover_letter.go`: exact draft/content/destination approval, staged document integrity, explicit file/text mode, local outcomes and safe pre-mutation review. Only approvals known not to have written can be replaced. Successful or uncertain uploads are not replayed.
- `application_answers.go`: exact question keys, deterministic resolution, one authoritative editor model and optimistic SQLite revision checking. Definition changes leave earlier answers available for manual review rather than moving or deleting them.
- `application_answer_transfer.go` and `chromium/answer.go`: reviewed actual-value snapshots, per-field current-value checks, exact live metadata/options, appropriate native events and readback. Failures stay local. Unchanged content does not need a new approval merely because field order or a nonmaterial revision changed.
- `reusable_answers.go`: exact employer-scoped memory keys and revisioned values. No taxonomy, embeddings, label-similarity matching, AI resolution or extra question screen.
- `database.go` and `deletion.go`: two additive tables, `application_answers` and `reusable_answers`. Application answers are removed with their opportunity. Explicitly reusable originals are independent of the source opportunity. Answer plus optional reusable-original changes commit together in a SQLite transaction.
- `templates/job.html`, `application_handlers.go`, `handlers.go`, `app_http.go`, `desktop_redirect.go`, `static/styles.css`, `static/inspection.js`, and `browser/contact.go`: integrate the capability into Prepare, preserve older-draft redirects, show value-free outcomes, bound editor width, and place ready facts behind disclosure.

New structs carry concrete approval snapshots, stored revisions, editor provenance or value-free outcomes needed by these implemented features. The narrow session capabilities follow the existing optional contact/resume pattern; no new ATS runner, general authorization framework, recovery engine or dependency was added.

Tests live in `cover_letter_transfer_test.go`, `application_answers_test.go`, `application_answer_transfer_test.go`, `reusable_answers_test.go`, the existing Chromium attachment/transport/integration tests, and the extended `contact_qualification_test.go`.

Pre-existing uncommitted work was preserved. No commits were created because the starting tree already mixed multiple completed changes in the same files.

## Verification

All final checks passed:

- `go test ./...`
- `go test -tags desktop ./...`
- `go test -race . ./internal/application/browser/chromium`
- `YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification . -run TestControlledContactJourney -v -count=1`
- `YOURMOVE_QUALIFY_BROWSER=1 go test ./internal/application/browser/chromium -run TestControlledApplicationHandoff -v -count=1`
- `git diff --check`
- `graft build`
- Native Wails executable build: passed.

Tests used `GOCACHE=/private/tmp/yourmove-go-build`. Default-cache access and local fixture listening were initially sandbox-restricted; using the writable cache and approved local test execution resolved those environment restrictions.

Pure/storage checks cover deterministic PDF parsing/pagination, content hashes, older-draft selection, stale letters/destinations, safe mode changes before selection, duplicate selection prevention, persistent answers, changed wording/options/sensitivity, qualified profile resolution, stale editor conflicts, partial transfer continuation, reusable revisions, different employers, application overrides, job deletion and cancellation while another session operation holds the lock.

Controlled real Chromium checks cover the normal Go UI-to-handler-to-session path, exact document content, empty/existing/ambiguous controls, native choice options, textarea rerendering, manual corrections, conditional questions, unsaved local editing during status transitions, continuation and no submission. The second-opportunity reuse UI check used an explicitly seeded copy of the controlled structural schema; it is not evidence of live cross-posting compatibility.

Wide (1280px) and narrow (390px) screenshots were captured and visually inspected. Dark-mode controlled layouts and the generated PDF were inspected. Native Wails was launched with isolated synthetic data; Opportunities → workspace → Prepare was observed, including saved-letter disclosure and question/reuse controls in light mode. Native edit/save attempts encountered repeated computer-use freshness interruptions; they are **not** counted as successful native save qualification. No new native browser-transfer claim is made.

No new live employer interaction was performed. The prior Increment A OnePay evidence was read and retained as prior evidence only. No application was submitted.

Exact logs and screenshots: `/private/tmp/yourmove-increment-b-evidence/` (`go-test.log`, `desktop-test.log`, `race-test.log`, `journey.log`, `cover-controls.log`, PNGs and synthetic sample PDF). The isolated native data and build also live under `/private/tmp`; normal application data was not used.

## Measured experience

Final controlled run; these workloads are not universal performance guarantees:

| Workload | Measured result |
|---|---|
| Existing contact/resume path | 1.588 s to observed transfer; 2.597 s to visible UI result; five UI actions |
| Letter-only authorization to observed PDF | 64 ms in the already-open synthetic session; two actions (approve, transfer) |
| Four prepared question answers | 1.602 s to observed transfer: textarea, native select, checkbox, radio |
| Separate cover primitive exercise | 371 ms for PDF selection, Unicode text rerender/readback, and manual-entry preservation |
| Question entry with optional memory | Three actions (enter, mark reusable, save); saved answer survived navigation |
| Conditional continuation | One additional question saved/transferred in the same browser; earlier fields/documents preserved |
| Reuse | Second qualified preparation displayed the original answer without retyping |

Measured unexpected overwrites, duplicate resume uploads and employer submissions: **zero**. The file primitive fixture observed one resume selection and one cover-letter selection, and blocked a repeated cover selection. Changed native options, ambiguous question identities and conflicting manual narratives were preserved without writes. No browser restart was needed for the successful letter/question/conditional continuation path.

The local fixture verifies File selection/content and, where instrumented, synthetic resume receipt. It does **not** establish real employer upload processing or server acceptance. UI outcomes explicitly retain that distinction.

## Remaining gaps and next work

- **Release qualification:** the full new journey needs native Wails save/transfer qualification and a freshly approved, posting-specific live Ashby exercise. Phase 0's native add/edit navigation, persistence and picker-cancellation debt remains outstanding.
- **Document limitation:** the small initial PDF exporter supports Windows-1252 text, including common smart punctuation and accented Latin characters. Other glyphs fail explicitly; the exact Unicode text-field path is supported. File-only forms requiring other glyphs or rejecting PDF still need a manually prepared accepted document. Employer upload processing/acceptance is unqualified.
- **Provider limitations:** custom dropdowns, ambiguous/positional identities, sensitive disclosures, embedded frames and unsupported controls remain manual. Reuse is deliberately restricted to exact questions at the same employer. Greenhouse/Lever transfer has not been qualified or expanded here.
- **UX/feature gaps:** the distinct resume/contact, letter and answer approvals have not yet been consolidated. There is no separate reusable-memory management/deletion surface; originals can be explicitly revised through the existing editor. A completed or uncertain letter upload cannot be automatically replaced in place. Browser loss does not invent recovered employer state.

**Single next step:** qualify this complete path in native Wails on one freshly authorized Ashby posting, including independent observation of employer document acceptance, before extending transfer to another provider.
