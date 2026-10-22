# Narrative answer implementation and qualification

Implemented against `be24335d3e8d6d28e49dfec359c15bcf10447958`, confirmed against fetched `origin/main` before editing. The supplied authoritative design document was preserved. This report describes implementation and measured qualification, not a new architecture proposal.

Checkpoint 2 follow-up: see [`2026-10-22-answer-assistance-refinement.md`](2026-10-22-answer-assistance-refinement.md) for per-question lazy loading, the unified Help me answer composition, visual captures, qualification, and delivery status.

## Ownership and boundaries

- Application-owned deterministic policy admits only active, current, unambiguous, editable narrative questions already accepted by Answer Library eligibility. Unsupported questions retain their ordinary manual workflow.
- Selected reviewed Career Evidence and explicitly selected integrity-checked resume excerpts supply factual input. At least one is required. Answer Library is writing reference only; employer context is requirements only. Profile/demographic data and unselected sources are excluded.
- Review, generation, and adoption checks never write application answers. The existing answer form saves; the existing separately authorized transfer sends saved material to the employer. Submission remains manual. No migration, generated-answer table, dependencies, frontend framework, or orchestration framework was added.

## Exact limits

All limits are server-side byte limits, including source labels/serialization overhead. UTF-8 boundaries are preserved.

| Material | Maximum |
| --- | --- |
| Exact observed employer label | 4 KiB, rejected if oversized |
| Career Evidence | 2 records; 12 KiB each; 20 KiB combined; rejected rather than truncated |
| Previous writing and provenance | 1 entry; 4 KiB |
| Resume evidence | 2 distinct documents; 4 KiB each; 6 KiB combined |
| Job context | 3 KiB; off by default |
| Editorial instructions | 8 KiB |
| Complete prompt plus instructions | 48 KiB |
| Proposed answer | 6,000 bytes and 600 whitespace-delimited words |
| Review request body | 8 KiB |
| Other drafting request bodies | 2 KiB |
| Tickets | 16 per App; 256-bit random opaque identity |
| Lifetimes | Review 10 minutes; completed preview 30 minutes; execution 120 seconds |

Previous writing, resumes, and employer descriptions use deterministic leading excerpts with visible truncation markers. Previous-writing excerpts shorten only the example; the complete original question and employer/job/host provenance remain intact. Entries whose provenance alone cannot fit 4 KiB are rejected. Job context likewise preserves title/employer and shortens only the description. Regression tests verify valid structured serialization and intact provenance at the budget boundary. Two selected distinct resumes receive at most 3 KiB each to fit their aggregate budget without silently omitting either. After local review, resume excerpts can be shortened by character count; the browser submits a UTF-8 byte boundary and the server re-extracts and validates the contiguous leading slice. Career Evidence retains title, situation, actions, employer/role context, technologies, period, and outcome state. Only recorded outcomes include outcome text; original writing, source question, and source notes are excluded.

## Consent, execution, and lifecycle

The exact question, provider/model, source identities, full projected payload, and trusted instructions are available before authorization. Both generation permission and possible external-transmission acknowledgment are fresh for each ticket. Consent is independent of cover letters and transfer authorization. Loopback connectivity is described as transport locality, never proof of local inference. Ollama can use cloud models; compatible services may forward requests; Codex/Claude CLIs use vendor services.

`reviewed → running → complete / insufficient / failed / canceled` is irreversible for provider invocation. Reservation is atomic. Duplicate requests return the existing state/result. A canceled but still-returning provider retains the job's in-flight reservation. No lock or database transaction is held during execution. There are no application retries, fallbacks, or repair calls. Vendor CLI internal behavior is outside YourMove's control.

Snapshots include exact question identity, saved revision, resolved baseline hash, source identities/revisions/creation identities, resume file/excerpt hashes, job-context hash, canonical payload/hash, provider/model, and instruction/policy hashes. Reconstruction occurs before transmission, after generation, and before adoption. Changes invalidate adoption; a completed stale proposal remains copyable. Known incomplete provider completions, invalid output, mixed insufficient-evidence markers, reasoning/process artifacts, tools, and oversized responses fail without rewriting.

Ollama and Anthropic/chat profiles allow 1,024 output tokens; OpenAI Responses allows 1,600. Narrative generation does not inherit the MiniMax cover-letter exception or the cover-letter 30-byte minimum/letter wrapper. Codex output-file reads are bounded. Existing endpoint restrictions, credential configuration, stdin privacy, environment scrubbing, isolated CLI work directories, session protections, and redirect rejection remain in use.

Live previews retain an accessible disclosure of their exact reviewed source payload in the DOM; browser recovery stores only bounded source references, not that payload. Preview never alters the editor. Explicit adoption checks server state, editor identity/revision, and textarea contents around the asynchronous request and replacement confirmation. It inserts unsaved text and dispatches input without updating hidden revisions. Ordinary dirty-form recovery then owns adopted text. Bounded per-tab sessionStorage retains at most 16 previews, each at most 22,000 serialized characters, without source payloads. Older recovery entries can be removed when the cap is reached. Recovery requires a fresh server check for adoption; expired or missing tickets permit copying only. Storage failure retains the DOM preview with navigation protection. Recovery is not promised after shutdown, tab closure, or storage clearing.

## Verification evidence

No paid services, real applicant data, normal application data directory, or live employer submission were used. Fixtures use temporary SQLite/resume storage, loopback servers, injected generators, and fake vendor CLIs.

- Full standard and desktop-tag Go regression suites pass.
- Standard build and desktop production build pass; the latter writes `/private/tmp/yourmove-go-narrative-ai`.
- `node --check static/ui.js` and `git diff --check` pass.
- Focused narrative app/provider race tests pass. They cover single-use duplicate generation, per-job exclusion, cancellation, shutdown, source changes, safe errors, budgets, independent resources, integrity checking, and no automatic persistence.
- Provider tests exercise all five HTTP adapters and both CLI adapters with narrative profiles and short prose. Existing cover-letter regressions remain passing.
- DesktopHandler HTTP tests use Wails origins and verify JSON/no-store responses without navigation redirects and rejection of foreign origins.
- Controlled Chromium exercises selection, local review without provider execution, current authorization, read-only preview, session recovery without generation on reload, canceled replacement, typing during generation/adoption, duplicate clicks, provider failure, insufficient evidence, cancellation, stale evidence, keyboard source/consent/generation/adoption controls, post-confirmation rechecks, storage-failure navigation protection, 390px light/dark screenshots without overflow, explicit unsaved adoption, editing, normal save, script-free saving, and SQLite restart persistence. The fixture verifies no automatic employer browser opening or submission.
- Existing controlled Question Workbench, Career Evidence Workbench, independent cover-letter, Contact Journey, and Combined Continuation fixtures also pass. They exercise dirty navigation, answer conflicts, no-JavaScript saving, and independently authorized employer transfer with zero submissions.

## Limitations and remaining native qualification

Syntactic checks and restricted sources cannot prove factual accuracy, relevance, or complete resistance to prompt injection. Applicant review remains required. Conservative question classification deliberately leaves some legitimate narrative fields unsupported. Additional source choices are bounded to suggestions plus 30 recent records, rather than a full paginated archive browser. Preview storage is ephemeral and private to the tab; it is not durable answer persistence.

Controlled Chromium and Wails-origin HTTP tests do not establish actual native Wails WebView behavior. The native application window was not exercised. Remaining native checks: source/review/consent controls, provider/model control parity, keyboard focus, copy behavior, asynchronous adoption and replacement dialogs, navigation protection, light/dark appearance, and redirect/recovery behavior. The desktop window's configured minimum width is 700px; 390px testing describes the shared browser UI, not a resized native window.

GitHub Actions results and final commit identities are reported with the implementation handoff after the normal push to main.

## Changed-file inventory

- Embedded resources and constructors: `assets.go`, `main.go`, `desktop.go`, `internal/app/app.go`, `internal/app/resources_test.go`.
- Application backend and contracts: `internal/app/app_http.go`, `internal/app/desktop_redirect.go`, new `internal/app/application_answer_ai.go`, `internal/app/application_answer_ai_state.go`, and `internal/app/application_answer_ai_test.go`.
- Workbench presentation and browser qualification: `internal/app/application_answers.go`, `internal/app/handlers.go`, new `internal/app/application_answer_ai_presentation.go`, `internal/app/application_answer_ai_presentation_test.go`, `internal/app/application_answer_ai_qualification_test.go`, `templates/job.html`, `static/ui.js`, and `static/styles.css`.
- Shared provider implementation and tests: `internal/coverletter/provider/providers.go`, `internal/coverletter/provider/subscription.go`, and new `internal/coverletter/provider/narrative_test.go`.
- Editorial instructions and handoff: new `prompts/application_answer.md`, `prompts/README.md`, `docs/CODEBASE_GUIDE.md`, this qualification report, and the supplied `docs/implementation/2026-10-22-application-answer-ai-design.md` retained unchanged.
