# YourMove Go — Codebase Field Guide and Repository Cleanup Plan

**Source basis:** verified against baseline commit `2345d3157eb61e8ed7633a7ad8d8d5c318b50227` on October 21, 2026. Paths and architecture below were checked against that checkout. This is not a claim of universal employer-site compatibility or release qualification; dated reports in `docs/implementation/` remain historical.

## What this guide is for

Find the smallest correct set of files for a change; understand how a user action reaches data or Chromium; make and test changes manually; and improve the public repository without dragging an active product through a speculative architectural rewrite.

## The mental model

```text
Browser main.go ────────┐
                       ├── App + routes (app_http.go)
macOS Wails desktop.go ─┘           │
                         HTTP handlers + HTML templates
                                    │
                       feature/domain operations
                         /                  \
                SQLite + files      sessionHost (memory)
                                           │
                                  Chromium Session (CDP)
                                           │
                                     Employer form
```

- `main.go` (`!desktop`) runs a local HTTP server on `127.0.0.1:3000`.
- `desktop.go` (`desktop && darwin`) embeds that same HTTP application in Wails and supplies native file dialogs and browser launching.
- `app_http.go` defines `App`, routes, and local request-host/origin checks.
- The top-level `package main` implements most application logic with feature-oriented files. This flat package is not inherently unidiomatic; it makes internal access straightforward but needs clear naming and navigation.
- `templates/*.html` are **server-rendered**; `static/ui.js` and `static/inspection.js` add enhancement and polling. Go POST handlers remain the mutation authority.
- SQLite and document bytes are durable; an open employer `sessionHost`/Chromium state is not durable across process exit.
- `internal/platforms` fetches *job-posting metadata*. `internal/application/providers` retrieves *public application requirements*. `internal/application/browser` observes the *live employer page*. `internal/application/browser/chromium` launches Chromium and performs CDP reads/writes. These are four different purposes.

## One request, end to end

### Opening the Prepare workspace

1. `GET /jobs/{id}` is registered in `app_http.go`.
2. `handlers.go: jobDetail` loads the job and related records for the workspace.
3. `application_handlers.go` defines the `jobWorkspace` view data; `application_continuation.go: summarizePreparation` derives the next action without transferring anything.
4. `templates/job.html` renders the existing state, including exact transfer scope and the appropriate form.
5. `static/ui.js` controls tabs, question focus, dirty editing and other browser-side conveniences; `static/inspection.js` polls for new observations without silently discarding unsaved changes.

### Clicking Fill application / Continue application

1. Form POST to `/jobs/{id}/continue` → `application_continuation.go: continueApplication`.
2. It validates the saved destination, parses the displayed `include_resume`, `include_letter` and `include_answers` flags, and independently calls:
   - `browser_handlers.go: resumeHandoffCommand`;
   - `cover_letter_transfer.go: coverLetterCommand`;
   - `application_answer_transfer.go: answerTransferCommand`.
3. Each subcommand rechecks its own material identities, hashes/revisions, destination, current eligibility and active session; it **arms** its operation, not a blanket application submission.
4. `browser_session.go: sessionHost` starts/owns the browser and its polling watcher. The watcher invokes `tryResume`, `tryContacts`, `tryCoverLetter` and `tryAnswers` as eligible.
5. `internal/application/browser/chromium/*` evaluates the current employer page and performs the actual CDP operation while preserving manual values and avoiding unsafe replay.
6. Individual observed results flow back to `browserRunState` and the Prepare UI. **Selecting a document or filling a field is not proof the employer accepted or submitted the application.**
7. `POST /jobs/{id}/submit` is a distinct user-confirmed *local record*; no YourMove operation automatically submits the employer application.

### Persistence and restarting

- `storage.go: applicationStorage` selects an absolute workspace location, normally `~/Library/Application Support/YourMove` on macOS, or the `YOURMOVE_DATA_DIR` override.
- `database.go: openDB` uses one SQLite connection and creates/upgrades the schema in `initJobsSchema`.
- `jobs.go`, `resumes.go`, `candidate_profile.go`, `cover_letters.go`, `application_answers.go`, `reusable_answers.go`, and `application_inspections.go` read/write their durable records.
- Stored resumes live in a separate `resumes/` directory within the workspace. Back up the database **and** that directory together, with the app stopped.

The main persisted records map to these tables and files:

| Data | Persistence location |
| --- | --- |
| Opportunities and selected resume | `jobs` table in SQLite |
| Resume metadata and bytes | `resumes` table plus files in the workspace `resumes/` directory; bytes are checked by SHA-256 |
| Candidate information and approval | The single `candidate_profile` row in SQLite |
| Cover-letter drafts | `cover_letters` table, including draft text and source resume/job hashes |
| Observed application form | `application_inspections`, `application_fields` and `application_steps` store observed structure, not a live browser session |
| Employer answers | `application_answers` stores job/destination/provider-specific answers; `reusable_answers` stores separately approved employer-scoped answers |

- An app restart does not restore an in-memory employer browser session, its unsubmitted form changes or its last transient transfer statuses. Never make a UI promise to the contrary.

## What to edit, by goal

| You want to change… | First file to inspect | Follow into… | Minimal verification |
| --- | --- | --- | --- |
| Opportunity list appearance | `templates/jobs.html` | `ui_pages.go`, `static/styles.css`, `static/ui.js` | render test + open `/jobs` |
| A field on a job | `templates/job.html` | `handlers.go: editJob`, `jobs.go`, `database.go` if persisted | add/edit/restart on isolated data |
| Batch URL import limits/duplicates | `intake.go` | `intake_handlers.go`, `intake_test.go` | repeated/invalid URL import |
| Posting title/company/description fetch | `internal/platforms/*` | `handlers.go: fetchPostingDetails`, `enrichment.go` | platform tests + actual supported posting if safe |
| Queue / next-job ordering | `triage.go` | `ui_pages.go`, `application_continuation.go: nextApplication` | synthetic mix of ready/closed/applied jobs |
| Role-family suggestion | `role_families.go: matchResume` | `family_handlers.go`, `templates/families.html` | family match tests; no silent resume reassignment |
| Resume import/change | `resumes.go` | `application_handlers.go`, `templates/resumes.html`, `templates/job.html` | stored bytes/hash, selection and restart |
| Candidate facts / approval | `candidate_profile.go` | `inspection_handlers.go`, `application_answers.go` | approval and resolution tests |
| Cover-letter prompt/provider | `prompts/cover_letter.md`, `cover_letters.go`, `cover_providers.go` | `cover_subscription.go`, `templates/job.html` | provider test + saved draft untouched on error |
| Letter editing or save behavior | `cover_letters.go` | `templates/job.html`, `static/ui.js`, `desktop_redirect.go` | save/reopen/Unicode/error recovery |
| Letter attachment/text fill | `cover_letter_transfer.go` | `browser_session.go`, `chromium/cover_letter.go`, `chromium/attachment.go` | controlled fixture checks exact content and repeat-upload suppression |
| Employer question preparation | `application_answers.go` | `reusable_answers.go`, `templates/job.html`, `static/ui.js` | identity/revision/option/explicit override checks |
| Employer answer transfer | `application_answer_transfer.go` | `browser_session.go`, `chromium/answer.go` | controlled fixture controls, manual value preservation |
| How one-click continuation is composed | `application_continuation.go` | `handoff_state.go`, `application_handlers.go`, `templates/job.html` | exact flags and independent partial failures |
| Browser start/stop/recovery | `browser_handlers.go` | `browser_session.go`, `chromium/chromium.go` | isolated browser session and recovery checks |
| Employer field detection | `internal/application/browser/inspect.js` | `observation.go`, `application_inspections.go`, `schema/schema.go` | browser observation fixtures |
| ATS field-mutation logic | `internal/application/browser/chromium/answer.go`, `contact.go`, `attachment.go` | `chromium.go`, `posting.go` | controlled browser fixture; never use a live submission as a test |
| Desktop-only navigation or origin handling | `desktop_redirect.go` | `desktop.go`, `app_http.go` | desktop-tagged tests + macOS manual route |
| Native file save/export | `desktop_export.go` | `attachment_downloads.go`, `application_handlers.go`, `desktop.go` | native Save/Cancel and exact exported bytes |
| App data directory / legacy import | `storage.go` | `database.go`, `resumes.go` | synthetic workspace; confirm normal data untouched |

## How to make one manual change without getting lost

1. **Choose an observable behavior.** Example: after saving a resume, stay on Prepare.
2. **Find the UI control.** Search `templates/job.html` for `action="/jobs/` or `Save resume`; identify the exact HTTP method/path and field `name=` attributes.
3. **Find its registration.** Search `app_http.go` for that route; open the handler, not the entire repository.
4. **Find where the decision is made.** For a redirect, start in `application_handlers.go: setJobResume`. For a presentation label, start in `application_continuation.go` or `handoff_state.go` rather than modifying browser logic.
5. **Check the state owner.** Does it change SQLite data, a temporary `sessionHost` operation, or a view-only projection? Make the change in the owner, not in three duplicated displays.
6. **Make the smallest edit; preserve posted field names and identity/revision checks.** Native Wails GET redirects are specially adapted; changing redirect destinations can affect `desktop_redirect.go`.
7. **Run targeted tests, then broader tests if needed.** Exercise the real browser/desktop path for visual or host-sensitive behavior, with isolated data.
8. **Read the diff.** Confirm no incidental changes, private data or build artifacts. Commit only the intended unit.

Useful commands from the repo root:

```sh
rg -n 'Save resume|/jobs/\{id\}/resume' templates app_http.go application_handlers.go
rg -n '^func .*setJobResume|^func .*jobDetail' --glob '*.go'
git diff --check
git diff --stat
go test ./...
go test -tags desktop ./...   # on supported macOS toolchain
```

### A persistent, isolated development environment on macOS

A normal `go run .` uses the real default YourMove workspace, and a first launch can consider a legacy `jobs.db` in the current working directory. For manual practice, build once, then run from a clean runner directory with a **different persistent data directory**:

```sh
mkdir -p /tmp/yourmove-go-manual-runner
go build -o /tmp/yourmove-go-manual-runner/yourmove .
(cd /tmp/yourmove-go-manual-runner && \
  YOURMOVE_DATA_DIR="$HOME/Library/Application Support/YourMove-Manual-Dev" \
  ./yourmove)
```

This intentionally does **not** start the Wails window; it opens local browser mode at `http://127.0.0.1:3000`. Use the same `YOURMOVE_DATA_DIR` on subsequent runs to retain experiments. Do not use that path for automated qualification involving destructive data fixtures. Run the app under the exact Go toolchain declared in `go.mod`; the snapshot specifies Go 1.27.2. Do not run two instances against the same managed Chromium/session workflow.

For a native-only change, verify `go test -tags desktop ./...` and `go run -tags desktop .` on macOS with an explicitly isolated workspace. Wails behavior is not fully proven by an ordinary browser request.

## The most important boundaries to preserve

- **UI rendering ≠ authority.** A hidden input/value in a form isn't sufficient: the Go POST handler must validate the current job, destination, content identity and revision.
- **Local save ≠ employer transmission.** Editing a letter or answering a question in YourMove must not automatically send it to an employer.
- **Browser-selected file ≠ employer acceptance.** Keep the observed outcome language honest.
- **Employer completion ≠ local recording.** The user submits in the employer browser; YourMove records the user's confirmation separately.
- **Persistent record ≠ active browser session.** An SQLite answer can survive restart; the same Chromium tab and in-memory operation cannot be assumed to.
- **Manual correction takes precedence.** Browser mutation should preserve existing values when ambiguous or manually filled.

## Repository cleanup: recommended order

### Pass 0: Protect and inspect before changing anything

```sh
git status --short
git branch --show-current
git ls-files > /tmp/yourmove-tracked-files.txt
git diff --stat
git diff --check
```

Back up the **real application workspace** separately from Git before storage/migration experiments. Do not `git clean -fdx`, reset, squash, or delete worktrees while any pending branch/data recovery matters. Treat history and private evidence as separate concerns.

Check for accidentally tracked `.db`, `.db-wal`, document files, cached browser profiles, `.env`, logs, screenshots, provider requests/responses, or applicant content. Keep local evaluation evidence and machine-specific agent/editor settings out of commits unless they are deliberate, sanitized repository fixtures. `.gitignore` does not untrack already committed files; if any sensitive data was ever committed, deleting it from HEAD alone does not remove it from history.

### Pass 1: Make the public entry point truthful

1. Keep the supplied marketing illustration as the README's primary visual. Add an application screenshot only when it is a real capture with synthetic or redacted data, and label it clearly.
2. State *exactly* what users can run/download. Do not imply a signed installer exists if one has not been published.
3. Describe supported employer workflows as qualified per feature; do not equate page metadata support with safe form-filling support.
4. Provide simple developer orientation: where routes are, how to run tests, where data is saved, and how to build safely.
5. Add a clear `LICENSE` **only after choosing the intended grant of rights**; don't call an unlicensed repo open source.
6. Fix the placeholder module identifier: `module example.com/yourmove-lite` and imports in approximately 50 Go files. Prefer a single scoped change to `github.com/jenksed/yourmove-go` **if that is the canonical repo path**. Use `go mod edit -module=...`, update local imports, `gofmt`, and test. This improves public consistency, not behavior.

### Pass 2: Reduce contradictory documentation

- `WAILS_COMPAT.md` at repository root includes historical claims about repository-relative templates, `go.mod tidy`, and missing packaging/export features. **Archive as a historical spike or remove after verifying every still-relevant instruction migrated to an accurate developer guide.**
- `UI_WORKBENCH.md` is a blend of history, layout rules, and outdated patch-application workflow. Move historical parts to `docs/history/`, retaining active guidance in `docs/design/` or the developer guide.
- `docs/design/application-workbench-v1.md` is a proposal, not an implementation contract. Keep that status visible if the design is retained.
- Keep date-stamped implementation and qualification reports under `docs/implementation/` as historical records. Use this guide and `docs/RELEASE_CHECKLIST.md` for current architecture and release status; do not treat an old report as a current site-compatibility claim.
- Keep current-user and developer entry points clear: `README.md`, `docs/TRY_IT.md`, `docs/FEEDBACK.md`, `docs/RELEASE_CHECKLIST.md`, and this guide. Keep dated implementation records separate from current instructions.
- Graft is an optional navigation accelerator. This guide, ordinary Go tooling, `rg`, and direct source inspection are sufficient for development without it.

### Pass 3: Improve file hygiene, not package count

- Add focused ignores for local `.env` files, SQLite sidecars/DBs, private resume data directories, platform junk, logs, temporary exports, test output and `.app` build artifacts. Don't blanket-ignore test fixtures, deliberately checked-in assets, PDFs or DOCX files that tests need.
- Verify `.github/ISSUE_TEMPLATE` privacy warnings and add a minimal issue/feedback entry point if the repository expects reports.
- Add a small CI job running `go test ./...` when the declared Go version is available on the runner. Run desktop-tagged tests on an appropriate macOS runner only if it is a maintained supported path. The goal is a reliable signal, not a large matrix.
- Leave the flat `package main` alone unless you encounter **specific edit friction** that would be fixed by extracting a small focused package. Existing `internal/*` packages already provide real abstraction boundaries.
- If you do split a large file, split **by responsibility and caller workflow**, initially within the same package. Example: `resumes.go` currently mixes document parsing/identity, registry operations and confirmed submission history. `submissions.go` is a credible low-risk separation within `package main`, but only if you're actually editing that area. Avoid broad file moves while stabilizing application throughput.

### Pass 4: Validate the public result

```sh
gofmt -w <changed-go-files>
go test ./...
go test -tags desktop ./...   # supported macOS host
git diff --check
git status --short
```

Review rendered README links, issue templates, sample commands, release claims, and a clean checkout / clean data-dir startup. Check tracked files rather than trusting `.gitignore` alone. Keep repo-document cleanup in separate commits from runtime behavior and Go module rename.

## What not to do

- Do not move 68 production Go files into a new `internal/app/*` hierarchy just to shorten the root directory.
- Do not rename route paths, form `name=` attributes or Wails redirect targets during cosmetic cleanup.
- Do not merge `internal/platforms` and `internal/application/providers`; they serve different information boundaries.
- Do not replace SQLite with a repository abstraction or introduce a general state machine because there are several status fields.
- Do not remove regression fixtures or tests solely to reduce file count.
- Do not delete old docs before checking what is linked from the README and GitHub issues.
- Do not conflate a passing Go unit suite with a native Wails or real employer acceptance test.

## Practical control trace: save an employer answer

Use this path to follow one visible control through the application:

1. Find the rendered answer form in `templates/job.html` and its `POST /jobs/{id}/answers/save` action.
2. Find that route in `app_http.go` and the handler `application_answers.go: saveApplicationAnswer`.
3. Follow answer preparation and source selection through `prepareQuestions` and `resolveApplicationAnswer` in the same file, with approved profile fields in `candidate_profile.go` and reusable answers in `reusable_answers.go`.
4. Confirm the per-job answer is written to `application_answers` in SQLite. Employer transfer is a separate action handled by `application_answer_transfer.go` and the browser session.
5. Use `application_answers_test.go` or `question_workbench_test.go` for focused local verification, then run the broader suite when appropriate.

---

## File index for the verified baseline

These descriptions are navigation hints checked against the baseline commit above; refer to source before editing. Files suffixed `_test.go` verify behavior and should normally be changed only when the intended behavior or fixtures change.

### Root Go application files (43 files)

| File | Role |
| --- | --- |
| `ai_handlers.go` | AI connection/setup presentation |
| `app_http.go` | App composition, HTTP routes and local-request defenses |
| `application_answer_transfer.go` | Scoped prepared-answer authorization and session transfer |
| `application_answers.go` | Observed question identity, answer resolution, local editor and saving |
| `application_continuation.go` | Exact scoped continuation composition and next-application action |
| `application_handlers.go` | Job workspace view structs; documents, resume selection and submission endpoints |
| `application_inspections.go` | Persistent definitions-only application inspection records |
| `assets.go` | Embed templates and static files in the binary |
| `attachment_downloads.go` | Saved cover-letter HTTP download |
| `attachment_names.go` | File naming for saved downloaded documents |
| `browser_handlers.go` | Managed browser actions, status, start/recheck/close and resume command |
| `browser_session.go` | Active browser ownership, session watcher and transfer execution |
| `candidate_profile.go` | Candidate profile model and persistence |
| `cover_letter_document.go` | Saved-letter PDF creation and content hash |
| `cover_letter_transfer.go` | Letter transfer authorization, staging and session execution |
| `cover_letters.go` | Cover-letter draft persistence, source evidence, generate/save handlers |
| `cover_providers.go` | Cover-letter provider configuration and API adapters |
| `cover_subscription.go` | CLI-based subscription provider connection and execution |
| `database.go` | SQLite open, connection policy and schema upgrades |
| `deletion.go` | Job deletion with history integrity |
| `deletion_handlers.go` | Single and batch delete HTTP actions |
| `desktop.go` | macOS/Wails desktop entry, native window and file dialogs |
| `desktop_export.go` | Trusted Wails Save dialog and exact file export |
| `desktop_probe.go` | Opt-in diagnostic probe for Wails request bodies |
| `desktop_redirect.go` | Trusted Wails request origin and safe POST-to-GET redirect adapter |
| `enrichment.go` | Background (in-process) posting-metadata enrichment worker |
| `enrichment_handlers.go` | Enrichment page and actions |
| `family_handlers.go` | Role-family and utilities HTTP endpoints; local form helper |
| `handlers.go` | Jobs pages, job workspace assembly, job edits and posting refresh |
| `handoff_state.go` | Browser result summary for presentation |
| `inspection_handlers.go` | Candidate page plus inspection page/HTTP actions |
| `intake.go` | URL normalization, deduplication and batch import logic |
| `intake_handlers.go` | HTTP entry for batch URL import |
| `jobs.go` | Job model, stage/priority labels and SQLite queries/updates |
| `main.go` | Non-desktop entry point and localhost HTTP server |
| `posting_open.go` | Native/HTTP action to open an employer posting |
| `requirements_handlers.go` | Read-only application requirements summary |
| `resumes.go` | Resume import/verification/extraction, assignment, review tokens and submissions |
| `reusable_answers.go` | Exact employer/question-scoped approved answer reuse |
| `role_families.go` | Role-family storage and deterministic resume suggestion |
| `storage.go` | Data-directory selection and legacy database migration |
| `triage.go` | Queue next-action and next-job recommendation |
| `ui_pages.go` | Opportunity dashboard summary and recorded-applications page |

### Internal Go packages and browser assets (46 files)

| File | Role |
| --- | --- |
| `internal/application/browser/chromium/answer.go` | Live employer question-answer mutations and readback |
| `internal/application/browser/chromium/attachment.go` | Resume/letter file-input selection and exact attachment |
| `internal/application/browser/chromium/attachment_test.go` | Tests for attachment |
| `internal/application/browser/chromium/chromium.go` | Managed Chromium process, CDP endpoint, observation and serialized work |
| `internal/application/browser/chromium/chromium_test.go` | Tests for chromium |
| `internal/application/browser/chromium/contact.go` | Employer contact-field mutations and outcomes |
| `internal/application/browser/chromium/cover_letter.go` | Employer letter text-field filling |
| `internal/application/browser/chromium/executable.go` | Locate supported Chromium executable |
| `internal/application/browser/chromium/executable_test.go` | Tests for executable |
| `internal/application/browser/chromium/handoff_integration_test.go` | Tests for handoff integration |
| `internal/application/browser/chromium/platform_forms_test.go` | Tests for platform forms |
| `internal/application/browser/chromium/qualification.go` | Test/qualification-only Chromium fixture helpers |
| `internal/application/browser/chromium/trial_run_test.go` | Controlled local Chromium fixture test |
| `internal/application/browser/chromium/transport.go` | CDP WebSocket dial helper |
| `internal/application/browser/chromium/transport_test.go` | Tests for transport |
| `internal/application/browser/contact.go` | Contact/answer values and field outcome types |
| `internal/application/browser/inspect.js` | JavaScript payload injected for read-only employer form observation |
| `internal/application/browser/observation.go` | Live structural observation representation and schema conversion |
| `internal/application/browser/observation_test.go` | Tests for observation |
| `internal/application/browser/posting.go` | Supported posting detection and owned-URL checks |
| `internal/application/browser/posting_test.go` | Tests for posting |
| `internal/application/browser/script.go` | Embed the inspection script |
| `internal/application/discovery/discovery.go` | Read-only application requirements dispatcher |
| `internal/application/discovery/discovery_test.go` | Tests for discovery |
| `internal/application/flow/flow.go` | Conservative classification of observed actions |
| `internal/application/flow/flow_test.go` | Tests for flow |
| `internal/application/providers/ashby/ashby.go` | Public Ashby application field inspection |
| `internal/application/providers/ashby/ashby_test.go` | Tests for ashby |
| `internal/application/providers/ashby/destination.go` | Qualified Ashby application destination resolution |
| `internal/application/providers/greenhouse/greenhouse.go` | Public Greenhouse application requirements inspection |
| `internal/application/providers/greenhouse/greenhouse_test.go` | Tests for greenhouse |
| `internal/application/schema/schema.go` | Provider-neutral application field/result schema and fingerprint |
| `internal/application/schema/schema_test.go` | Tests for schema |
| `internal/platforms/api.go` | Bounded provider HTTP fetch and parsing helpers |
| `internal/platforms/api_test.go` | Tests for api |
| `internal/platforms/ashby.go` | Ashby job metadata retrieval |
| `internal/platforms/ashby_test.go` | Tests for ashby |
| `internal/platforms/closed_test.go` | Tests for closed |
| `internal/platforms/description.go` | Posting-description processing helpers |
| `internal/platforms/greenhouse.go` | Greenhouse job metadata retrieval |
| `internal/platforms/greenhouse_test.go` | Tests for greenhouse |
| `internal/platforms/lever.go` | Lever job metadata retrieval |
| `internal/platforms/lever_test.go` | Tests for lever |
| `internal/platforms/paylocity.go` | Other supported posting-metadata path |
| `internal/platforms/paylocity_test.go` | Tests for paylocity |
| `internal/platforms/platforms.go` | Job-posting metadata dispatcher and errors |

### HTML templates (13 files)

| File | Role |
| --- | --- |
| `templates/ai.html` | AI generation connection settings/help |
| `templates/applications.html` | Manually recorded application list |
| `templates/candidate.html` | Profile editing/approval |
| `templates/enrichment.html` | Posting refresh/enrichment screen |
| `templates/families.html` | Role-family matching controls |
| `templates/import_results.html` | Batch import results |
| `templates/inspection.html` | Browser and requirements inspection page |
| `templates/job.html` | Per-job Overview/Prepare/History workspace and action forms |
| `templates/jobs.html` | Opportunities/queue list and import/add forms |
| `templates/partials.html` | Common HTML shell and shared rendering fragments |
| `templates/requirements.html` | Read-only discovered application requirements |
| `templates/resumes.html` | Document library and import/management forms |
| `templates/utilities.html` | Utilities navigation |

### Static assets and browser-side enhancement (7 files)

| File | Role |
| --- | --- |
| `static/inspection.js` | Browser observation/status polling and dirty-editor refresh guard |
| `static/styles.css` | Global design tokens, components and responsive styling |
| `static/ui.js` | Client-side enhancement: tabs, question focus, dirty forms and UX |
| `static/yourmove-symbol-primary.svg` | Primary YourMove mark |
| `static/yourmove-wordmark-primary.svg` | Primary YourMove wordmark |
| `static/yourmove-logo.png` | Raster application logo asset |
| `static/yourmove-wordmark-reverse.svg` | Reverse YourMove wordmark |

### Root-level tests / qualification (45 files)

| File | Role |
| --- | --- |
| `application_answer_transfer_test.go` | Tests for application answer transfer |
| `application_answers_test.go` | Tests for application answers |
| `application_continuation_test.go` | Tests for application continuation |
| `application_runtime_qualification_test.go` | Opt-in/controlled qualification for application runtime |
| `application_test.go` | Tests for application |
| `application_throughput_test.go` | Tests for application throughput |
| `attachment_names_test.go` | Tests for attachment names |
| `browser_handlers_test.go` | Tests for browser handlers |
| `browser_preflight_test.go` | Tests for browser preflight |
| `browser_recovery_test.go` | Tests for browser recovery |
| `candidate_profile_autofill_test.go` | Tests for candidate profile autofill |
| `closed_posting_test.go` | Tests for closed posting |
| `contact_qualification_test.go` | Opt-in/controlled qualification for contact |
| `cover_letter_independence_qualification_test.go` | Opt-in/controlled qualification for cover letter independence |
| `cover_letter_transfer_test.go` | Tests for cover letter transfer |
| `cover_letter_workbench_qualification_test.go` | Opt-in/controlled qualification for cover letter workbench |
| `cover_letter_workbench_test.go` | Tests for cover letter workbench |
| `cover_letters_test.go` | Tests for cover letters |
| `cover_source_errors_test.go` | Tests for cover source errors |
| `cover_subscription_test.go` | Tests for cover subscription |
| `database_test.go` | Tests for database |
| `deletion_test.go` | Tests for deletion |
| `desktop_export_test.go` | Tests for desktop export |
| `desktop_http_test.go` | Tests for desktop http |
| `desktop_redirect_test.go` | Tests for desktop redirect |
| `enrichment_test.go` | Tests for enrichment |
| `handlers_test.go` | Tests for handlers |
| `handoff_state_test.go` | Tests for handoff state |
| `inspection_markup_test.go` | Tests for inspection markup |
| `inspection_test.go` | Tests for inspection |
| `intake_test.go` | Tests for intake |
| `job_brief_test.go` | Tests for job brief |
| `opportunity_queue_qualification_test.go` | Opt-in/controlled qualification for opportunity queue |
| `posting_open_test.go` | Tests for posting open |
| `profile_completion_qualification_test.go` | Opt-in/controlled qualification for profile completion |
| `profile_question_presentation_test.go` | Tests for profile question presentation |
| `question_workbench_qualification_test.go` | Opt-in/controlled qualification for question workbench |
| `question_workbench_test.go` | Tests for question workbench |
| `release_data_test.go` | Tests for release data |
| `resume_continuity_test.go` | Tests for resume continuity |
| `resume_selector_test.go` | Tests for resume selector |
| `reusable_answers_test.go` | Tests for reusable answers |
| `trial_profile_test.go` | Tests for approved profile field mapping |
| `role_families_test.go` | Tests for role families |
| `ui_pages_test.go` | Tests for ui pages |

### Repository root configuration and documentation (9 files)

| File | Role |
| --- | --- |
| `.gitignore` | Git ignore rules for local files/artifacts |
| `.ignore` | Ripgrep search exceptions for ignored graph |
| `AGENTS.md` | Agent-specific navigation and design rules |
| `README.md` | Public project introduction and source-run instructions |
| `UI_WORKBENCH.md` | Historical/current mixed UI design and verification notes |
| `WAILS_COMPAT.md` | Historical Wails compatibility spike (partly stale) |
| `go.mod` | Go module identity, language/toolchain and dependencies |
| `go.sum` | Go dependency checksums |
| `opencode.json` | Local developer-agent Graft MCP configuration |

### Embedded writing instructions (2 files)

| File | Role |
| --- | --- |
| `prompts/README.md` | Contributor workflow for the cover-letter instruction source |
| `prompts/cover_letter.md` | Version-controlled instructions embedded by `cover_letters.go` |

### Repository presentation asset (1 file)

| File | Role |
| --- | --- |
| `docs/images/yourmove-hero.png` | README marketing illustration; not an application screenshot |

### Current and reference documentation (8 files)

| File | Role |
| --- | --- |
| `docs/FEEDBACK.md` | How to report user experience safely |
| `docs/RELEASE_CHECKLIST.md` | Current/historical release readiness checklist |
| `docs/TRY_IT.md` | User-friendly source preview walkthrough and storage warnings |
| `docs/VISUALS.md` | Screenshot capture/art direction notes |
| `docs/design/DESIGN_SYSTEM.md` | UI design foundations |
| `docs/design/RULES.md` | Additional UI composition rules |
| `docs/design/application-workbench-v1.md` | Earlier proposed workbench design; not the implementation contract |
| `docs/CODEBASE_GUIDE.md` | Verified route, persistence, browser, and developer map |

### Dated implementation reports (7 files)

| File | Role |
| --- | --- |
| `docs/implementation/2026-10-20-application-continuity.md` | Dated engineering handoff or qualification notes; not necessarily current |
| `docs/implementation/2026-10-20-application-experience.md` | Dated engineering handoff or qualification notes; not necessarily current |
| `docs/implementation/2026-10-20-question-workbench.md` | Dated engineering handoff or qualification notes; not necessarily current |
| `docs/implementation/2026-10-20-v01-critical-path-implementation.md` | Dated engineering handoff or qualification notes; not necessarily current |
| `docs/implementation/2026-10-20-v01-release-readiness.md` | Dated engineering handoff or qualification notes; not necessarily current |
| `docs/implementation/2026-10-21-trial-run-fixes.md` | Dated local-fixture trial notes; not universal employer-site qualification |
| `docs/implementation/ats-platform-integration-notes.md` | Dated engineering handoff or qualification notes; not necessarily current |

### Historical documents (11 files)

| File | Role |
| --- | --- |
| `docs/history/G1_APPLICATIONS.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G2_BULK_TRIAGE.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G3_ENRICH_DELETE.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G4_DOCUMENT_ROUTING.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_2_BROWSER_INSPECTION.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_3_WAILS_INSPECTION.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_5_RESUME_HANDOFF.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_6_BROWSER_RECOVERY.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_7_TRANSPORT_RELIABILITY.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_9_HANDOFF_QUALIFICATION.md` | Superseded milestone/history note; preserve only as reference |
| `docs/history/G5_INSPECTION.md` | Superseded milestone/history note; preserve only as reference |

### Release tooling (5 files)

| File | Role |
| --- | --- |
| `release/macos/Info.plist` | macOS app bundle metadata |
| `release/macos/README.md` | macOS build/distribution/release instructions |
| `release/macos/build.sh` | macOS package build |
| `release/macos/distribute.sh` | Signing/notarization distribution procedure |
| `release/macos/source_manifest.py` | Release source-manifest metadata generation |

### GitHub repository templates (2 files)

| File | Role |
| --- | --- |
| `.github/ISSUE_TEMPLATE/bug-report.md` | Public bug report template |
| `.github/ISSUE_TEMPLATE/experience-feedback.md` | Public experience-feedback template |


---

**Maintenance:** This guide was checked against the baseline commit identified above. Reconfirm source and route signatures after code changes; dated implementation notes are historical evidence, not current release or compatibility claims.
