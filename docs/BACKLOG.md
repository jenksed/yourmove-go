# YourMove engineering backlog

> Repository-maintained queue of **unresolved work**, not an acceptance report or an instruction to implement everything. This file complements [release policy](RELEASE_POLICY.md), [release checklist](RELEASE_CHECKLIST.md), [roadmap](ROADMAP.md), and [Phase 2 acceptance](implementation/product-experience-implementation/ACCEPTANCE.md).
>
> **Snapshot:** October 23, 2026; [Phase 2 PR #7](https://github.com/jenksed/yourmove-go/pull/7) open/draft at `33c9e64`. Status and commit references below must be rechecked before planning work.

## How to use this backlog

- **Work is opt-in.** When asked to prioritize the backlog, inspect the current branch/PRs, this file, release gates and relevant code/tests; recommend the top **2–3 currently actionable items** with dependencies and reasons. Do not autonomously start an unrelated task.
- **Priority is a decision, not an automatic sort:** P0 = demonstrated data loss, unauthorized transmission, security, or broken core journey; P1 = required for the next explicitly selected milestone or a high-value maintainability blocker; P2 = valuable after P1; P3 = optional. An unverified test is **not** a confirmed defect. The stage, scope, user request, dependencies, and evidence can override numeric ordering; state why.
- **Stages:** `PR closeout`, `post-merge refactor`, `beta acceptance`, and `later`. When finishing PR #7, focus on PR closeout and do not expand into automated native testing. When asked to refactor after merge, select refactoring tasks first. When asked to prepare a beta, treat applicable release gates as mandatory.
- **Statuses:** `OPEN` = actionable but not started; `NEEDS VERIFICATION` = reported gap or code-review concern, not established failure; `BLOCKED` = prerequisite prevents execution; `DEFERRED` = deliberately not in present scope; `IN PROGRESS` and `DONE` require owner/commit/evidence. Change status only after verifying actual evidence.
- Before acting, read `AGENTS.md` and relevant repository design rules. Confirm the **current** branch and clean/dirty worktree; preserve unrelated uncommitted work. Keep each change narrow and testable. Never use real applicant data or submit employer applications for a fixture.
- On completion, update the backlog entry with status, commit/PR, tests, and remaining follow-ups. A Go/Chromium PASS does not turn a native Wails NOT VERIFIED into a PASS. Keep completed items briefly as history or move them to a dated archive after updating links.

## PR #7 closeout — only narrowly scoped work

### YM-001 — Verify salary fallback against false positives and neighboring context
- **Priority:** P1 · **Stage:** PR closeout · **Status:** DONE
- **Source:** `internal/platforms/compensation.go`, `internal/platforms/compensation_test.go`; fix commit `799528e`.
- **Why:** Description salary extraction can mistake unrelated currency + annual-period text for compensation. Conversely, the immediately preceding benefits sentence may suppress a complete, explicitly labeled salary sentence. These are code-review concerns not yet recorded as failing tests.
- **Acceptance:** Added focused conference registration and annual service cost abstentions and confirmed extraction for `Benefits include health insurance. Base salary: $100,000 per year`. Existing mixed salary + bonus, structured-salary precedence, literal provenance and manual-edit precedence regressions remain. Full ordinary and desktop suites pass.
- **Completion:** Commit `8aad7d5` on PR #7; `go test -count=1 ./...` and `go test -count=1 -tags desktop ./...` PASS (2026-10-23).
- **Boundary:** No LLM extraction or generalized salary parser in this PR.

### YM-002 — Exercise actual completion-dialog cancel, Escape and reuse
- **Priority:** P1 · **Stage:** PR closeout · **Status:** DONE
- **Source:** `static/ui.js`, `internal/app/submission_recording_qualification_test.go`; fix commit `02726ab`.
- **Why:** Closeout required direct exercised cancellation and Escape paths plus dialog reuse evidence.
- **Acceptance:** `TestControlledSubmissionRecordingWorkflow` activates **Cancel and stay** and Escape, checks exact dirty answer/letter/note values and focus return, confirms zero local history after both paths and after browser validation blocks an unchecked declaration, then checks ordinary navigation copy and performs the affirmative local record. Existing idempotency and no-employer-submit assertions remain. Targeted Chromium and required opt-in runs PASS (2026-10-23); native Wails remains NOT VERIFIED.
- **Completion:** Commit `8aad7d5` on PR #7; targeted and required opt-in controlled Chromium completion qualification PASS (2026-10-23).
- **Boundary:** Fix lifecycle cleanup if needed; do not introduce a modal framework.

### YM-003 — Minimum human Wails smoke and honest PR decision
- **Priority:** P1 · **Stage:** PR closeout · **Status:** IN PROGRESS
- **Source:** [PR #7](https://github.com/jenksed/yourmove-go/pull/7), [Phase 2 acceptance](implementation/product-experience-implementation/ACCEPTANCE.md), [native evidence](implementation/product-experience-implementation/evidence/native.json), [owner-fillable native smoke record](implementation/product-experience-implementation/NATIVE_SMOKE_RECORD.md).
- **Why:** The bundle starts and renders, but final-source dirty editing, local completion/History and restart were not exercised in actual Wails. macOS GUI automation failed before attachment with `-10827`; this is not a demonstrated YourMove defect.
- **Acceptance:** On a **fresh synthetic isolated workspace**, a human actually enters/saves an answer or letter, exercises dirty navigation cancel/leave, records a synthetic local completion, inspects History, quits/relaunches **the same executable with the same `YOURMOVE_DATA_DIR` (do not rerun the fixture-generating script)**, and checks durable associations. Follow YM-016. Record each observed result, host/build identity and failure; do not infer from Chromium. If a core interaction fails, repair before merge.
- **Completion so far:** Prepared the owner-fillable record with all steps set to NOT RUN. It now distinguishes the first seeded qualification workspace from relaunching the same executable with the same existing `YOURMOVE_DATA_DIR`; no native action has been inferred from Chromium. Owner execution and review remain pending.
- **Boundary:** Manual smoke can close the **source-merge** review; it does not replace complete beta acceptance or authorize public distribution.

### YM-004 — Close PR #7 with preserved native limitations
- **Priority:** P1 · **Stage:** PR closeout · **Status:** IN PROGRESS · **Depends on:** owner completion/review of YM-003
- **Acceptance:** Current PR/CI must be rechecked after push; pending native gates are documented as NOT VERIFIED with an owner-fillable record. Keep PR #7 open/draft for owner review. No agent merges, marks native acceptance PASS, or publishes a release without authorization.
- **Completion so far:** Fix commit `8aad7d5`, verification commit `55bb4b5`, and owner documentation commits `1139631` / `f6d327d` are on PR #7. Latest pre-fix run `40051051163` had Go failure (fixed test timing) and desktop PASS. Run `40051967937` on `f6d327d` passed Go and macOS desktop verification, including bundle validation. A docs-only backlog/evidence update will be the new head; check its triggered CI before owner review.
- **Boundary:** Avoid appending unrelated refactors/design work to the 123-file Phase 2 diff.

### YM-015 — Recoverable Candidate Profile validation in native Wails
- **Priority:** P1 · **Stage:** PR closeout · **Status:** IN PROGRESS (user-observed, source-confirmed; acceptance pending)
- **Observation:** While editing Candidate Profile in the real Wails qualification app, saving with profile approval selected and missing first name, last name or email displayed plain text `Profile was not saved: first name, last name and email are required for profile approval` with no visible way to return to editing. The user could not recover the entered profile values from that response.
- **Source:** `internal/app/inspection_handlers.go: updateCandidate` uses `http.Error(..., 400)` for any `saveCandidateProfile` validation failure; `internal/app/candidate_profile.go` performs approval-only mandatory-field validation; `templates/candidate.html` has no inline recovery presentation.
- **Impact:** A normal incomplete-approval attempt drops the user into an unstyled error page and interrupts a primary local editing workflow. The rejected form is **not saved**; this is not evidence of database corruption or post-save persistence failure.
- **Acceptance:** On invalid approval, render Candidate Profile with the **submitted values intact**, a visible accessible validation summary tied to the relevant field(s), and working navigation/actions; permit correction and retry. Without approval, allow incomplete profiles to be saved as drafts, preserving the existing separate approval boundary. Do not silently approve or transmit facts. Exercise invalid approval, valid approved save, invalid email, unapproved draft, keyboard/focus and no-JS behavior in controlled tests and a real Wails window. Ensure the Wails-compatible response path keeps the form usable after errors; do not return an unstyled plain-text body.
- **Boundary:** Small focused validation/error-rendering fix; preserve server authority, approved profile revision semantics, no mandatory onboarding, and existing field mappings.
- **Implementation so far (2026-10-25):** Invalid saves now re-render the submitted profile with field-specific errors and an accessible summary; the last confirmed approval/revision remains separate from the submitted form state. Changes are uncommitted. No regression tests, screenshots, or Wails acceptance were recorded for this implementation yet.

### YM-016 — Repeat native qualification against the **same** synthetic workspace
- **Priority:** P1 for completing YM-003 · **Stage:** PR closeout · **Status:** OPEN (procedure/documentation gap; no product persistence failure proven)
- **Observation:** The maintainer stopped `scripts/qualify-native-workflow.sh --launch` with Ctrl+C, then ran the **same script again** and saw previous changes missing.
- **Source/cause:** The script invokes `mktemp -d /private/tmp/yourmove-workflow-native-XXXXXX` **on every run**, creates a fresh `workspace`, and calls `TestPrepareNativeWorkflowFixture` to seed a fresh `jobs.db`. Restarting the script means a different database, not an app relaunch into the same workspace.
- **Acceptance:** Document the exact distinction between a **fresh qualification run** and a **persistence/restart check**. Capture the `Bundle:` and `Workspace:` paths printed by the first successful script execution. Fully stop that app process; confirm it exited; relaunch its **existing executable** with `YOURMOVE_DATA_DIR` set to the **same existing workspace**, without rerunning the seeding script. Verify a successfully saved profile/answer/letter, document selection, and one local History record persist correctly. Record actual source SHA, workspace path and observed results. Clarify that a failed 400 validation save is never persisted.
- **Safety:** Do not run two YourMove instances simultaneously against the same workspace, re-seed an existing DB, use real applicant data, or manually modify `jobs.db`. Do not add a generic production `--resume` or relaxed fixture safety check merely to simplify this test. A small script help/README note is sufficient if it prevents recurrence.
- **Boundary:** A new synthetic workspace on every new harness execution is intentional. This is primarily a test-operator documentation and repeatability correction.
- **Completion so far:** The owner smoke record now documents capturing and reusing the same bundle/workspace and launching the existing executable with the same data directory. Native execution and observed persistence evidence remain pending.

## Job Intelligence — M1 Ashby capability

### JI-025a — Close the Ashby Job Intelligence capability
- **Priority:** P1 · **Stage:** M1 capability closeout · **Status:** DONE
- **Source/evidence:** [Authoritative implementation plan](implementation/job-intelligence/IMPLEMENTATION_PLAN.md); [M1 closeout evidence](implementation/job-intelligence/evidence/m1-closeout.md); M0 evidence under `implementation/job-intelligence/evidence/m0-overview/`. Raster captures remain in the local review archive and are excluded from the source PR.
- **Outcome:** Ashby intake now validates selected posting identity, preserves bounded HTML/plain source and direct role facts, stores latest successful source alongside canonical job data, renders a safe structured Overview, and keeps applicant edits authoritative. Canonical copy/generation and saved question identities remain compatible in focused regression coverage.
- **Verification:** Full ordinary and `desktop`-tagged Go suites, `go vet`, JavaScript syntax and whitespace checks PASS. Focused M0/M1 and broader controlled Chromium checks PASS as recorded below. On 2026-10-24, the current-source `TestControlledNarrativeAnswerWorkbench` and `TestControlledScopeFirstWorkbench` both passed in Chrome Canary 158.0.8122.0, confirming answer/letter save-target focus and keyboard behavior. VoiceOver speech is not covered by these browser tests.
- **Native evidence:** Maintainer-reported PASS for `/private/tmp/yourmove-m1-manual-build/YourMove.app` with isolated workspace `/private/tmp/yourmove-m1-manual`; executable SHA-256 `8dbac25b28f2db81f74d482ceb0c01a5f0a027668cb7e3c2835985c5eb84b5b2`, source-manifest SHA-256 `3209c103c82c1488cbae643671fcffdf51d839035302c4b2ce6ed6c1a0927549`, covering Job Intelligence interaction and persistence across restart. The maintainer separately reports PASS for the refined native saved-status focus behavior, without an executable identity. Individually unenumerated viewport/theme/clipboard/safe-link/accessibility/artifact-retention/browser-authority checks remain NOT VERIFIED. VoiceOver speech is not claimed.
- **Remaining acceptance:** None for the accepted M1 capability. VoiceOver announcement and duplicate behavior remain NOT VERIFIED and are deferred to YM-017; they do not block M1. No employer form or transfer action was part of acceptance.
- **Dependencies / boundaries:** No M2/M3/M4 work, provider portability, release declaration or browser-transfer acceptance is included. Existing unrelated worktree changes remain untouched; no commit, merge, push or release was made.
- **Completion:** M1 PASS. JI-023a, JI-024a and JI-025a PASS for the accepted capability; full native detail and VoiceOver speech are not claimed. VoiceOver remains deferred under YM-017. No source merge or release is implied.

## Job Intelligence — M2 focused reading

- **Priority:** P1 · **Stage:** M2 implementation/acceptance · **Status:** PARTIAL
- **Outcome:** Make responsibilities, qualifications, compensation and hiring instructions quick to reach while preserving every unknown and secondary section in source order and keeping canonical copy unchanged.
- **Implemented:** Conservative exact-heading classifier and tests; native fragment navigation on the existing Overview; unknown content remains visible; no-JavaScript and keyboard paths work. No secondary sections are hidden and no deeper compensation facts were selected.
- **Verification:** `go test -count=1 ./internal/jobdocument`; focused app render tests; pinned Chrome Canary 158.0.8122.0 connected Ashby demonstration with 700×550 navigation, no-JavaScript keyboard reachability, canonical copy, 1280/390 light/dark viewports, and no horizontal overflow. Raster captures remain in the local review archive and are excluded from the source PR. JI-023b PASS for controlled Chromium scope.
- **Remaining acceptance:** Actual Wails interaction, keyboard/focus, accessible text, and same-workspace restart are NOT VERIFIED. JI-024b remains open; M2 is not PASS.
- **Source/evidence:** [Implementation plan](implementation/job-intelligence/IMPLEMENTATION_PLAN.md#ji-004); [M1 evidence](implementation/job-intelligence/evidence/m1-closeout.md) is unchanged and does not substitute for M2 native acceptance.

## Job Intelligence — M3 provider portability

- **Priority:** P1 · **Stage:** M3 implementation/acceptance · **Status:** PARTIAL
- **Outcome:** Keep Ashby, Greenhouse, and Lever reading consistent in Overview while retaining provider-specific source shapes and existing manual/canonical behavior.
- **Implemented:** Greenhouse and Lever adapters use the existing fetch/intake/source/persistence/Overview path. Lever identity is bound to global/EU region; no provider abstraction or application authority was added. App lifecycle verifies source persistence, manual-edit precedence across a second provider refresh, updated source/freshness, and SQLite restart for both providers.
- **Verification:** `go test -count=1 ./internal/platforms`; `TestJobIntelligenceM3ProvidersUseIntakeOverviewAndPersistAcrossRestart`; pinned Chrome Canary 158.0.8122.0 `TestJobIntelligenceM3ProviderOverviewDemo` passed each provider separately at 1280/390 in light/dark, with exact canonical copy, direct facts, Lever named-section navigation, and zero submissions. Raster captures remain in the local review archive and are excluded from the source PR. JI-023c PASS for controlled Chromium scope. Official provider docs and current public hosted pages were reviewed; raw live API JSON remains NOT VERIFIED.
- **Remaining acceptance:** Native Wails portability and same-workspace restart remain NOT VERIFIED under JI-024c. JI-015b/JI-025b are PARTIAL until that candidate is accepted; M3 is not PASS. Posting support does not imply application-form discovery or transfer.
- **Source/evidence:** [Implementation plan](implementation/job-intelligence/IMPLEMENTATION_PLAN.md#ji-020); [provider onboarding evidence](implementation/ats-platform-integration-notes.md#provider-specific-entry-status--m3-update-2026-10-24).

## Job Intelligence — Competitive ATS coverage follow-on

### YM-018 — Expand public Job Intelligence coverage across competitive ATS platforms
- **Priority:** P1 · **Stage:** competitive provider implementation and acceptance · **Status:** IN PROGRESS
- **Source/evidence:** [Competitive ATS coverage scorecard](implementation/job-intelligence/evidence/competitive-ats-coverage.md); source, identity, lifecycle, and controlled browser findings are recorded there. Raster captures remain local and are excluded from the source PR.
- **Why / user impact:** More employer-posting URLs can reach the same source-aware Job Intelligence Overview with preserved descriptions and facts, refresh, manual precedence, persistence, and canonical copy.
- **Current result:** Seven requested providers were investigated. Workday, SmartRecruiters, Workable, and Jobvite are source-qualified, adapter-qualified, and controlled-supported through production intake/source/Overview. The seven-provider Chromium portfolio now submits manual edits, refreshes changed public-source fixtures, checks canonical copy, and verifies all seven jobs after SQLite restart. iCIMS, Taleo, and Avature remain research-only because repeatable public applicant-facing detail content was not established. The wider 10-provider set has seven controlled-supported providers when Ashby, Greenhouse, and Lever are included. Native Wails qualification for the four new adapters remains NOT VERIFIED; Greenhouse/Lever M3 native acceptance remains pending under JI-024c.
- **Acceptance:** Keep the supported variants and dispositions current in the scorecard; require public source/identity evidence, bounded adapter tests, exact canonical association, production lifecycle coverage, controlled Chromium portfolio evidence, and then native acceptance against the exact candidate before claiming Level 4. Preserve blocked providers as research without URL-only support claims. Run the documented ordinary, desktop, provider, storage/refresh, browser-authority, and source-hygiene checks for each integrated wave.
- **Dependencies / boundaries:** Preserve the existing `Metadata`/`PostingSource` v1 contract and app/storage/document architecture; no autofill, employer submission, custom-domain scraper, authenticated API, anti-bot bypass, SaaS, browser extension, or native automation expansion. Keep M2/M3 native gates and YM-017 separate.
- **Completion:** On 2026-10-24, four new adapters reached controlled qualification. The seven-provider synthetic Chromium portfolio passed intake, manual edit precedence, changed-source refresh, freshness, canonical copy, and SQLite restart checks, with desktop/narrow screenshots. Workable title/company storage caps and the qualification gap found by independent review were fixed. Three investigated variants remain blocked and native Wails evidence is outstanding. Final local candidate identity is recorded in the scorecard. No commit, merge, push, or release was made.

### JI-019a — Correct false optionality labels

- **Priority:** P1 · **Stage:** M4 label correction · **Status:** DONE
- **Outcome:** Missing legacy required-state evidence now displays “Required state not verified”; a recorded true value remains “Required.”
- **Verification:** Focused label/brief tests pass, including field JSON, schema fingerprint, question key, saved-answer, and transfer-approval invariance.
- **Boundary:** This closes only JI-019a. No requirements evidence sidecar, schema-discovery change, or browser observation change was made.
- **Source/evidence:** [Implementation plan](implementation/job-intelligence/IMPLEMENTATION_PLAN.md#ji-019a).

## Post-merge maintainability — preferred next development track

### YM-005 — Split high-cognitive-load frontend behaviors by responsibility
- **Priority:** P1 · **Stage:** post-merge refactor · **Status:** DEFERRED pending PR #7 closeout
- **Source:** `static/ui.js`, `static/inspection.js`, `templates/job.html`.
- **Outcome:** Make dirty-form navigation, completion dialog, workspace activities, question/letter editors, and small UI enhancements independently understandable without introducing a new frontend framework or source of truth.
- **Acceptance:** Document current event ownership and invariants; refactor in small commits with clear names, narrow interfaces and equivalent progressive enhancement. Preserve selectors, no-JS behavior, per-material authorization, sessionStorage recovery boundaries, and Wails redirects. Existing Go, desktop and opt-in browser tests pass; demonstrate reduced coupling/cognitive load beyond merely moving lines.

### YM-006 — Simplify handler/view-model to template reasoning
- **Priority:** P1 · **Stage:** post-merge refactor · **Status:** DEFERRED pending PR #7 closeout
- **Source:** `internal/app/handlers.go`, `internal/app/application_handlers.go`, `internal/app/ui_pages.go`, `templates/job.html`.
- **Outcome:** Reduce navigation needed to understand one user action, its view state, POST owner and persistence boundary.
- **Acceptance:** Trace two representative workflows before/after; remove proven duplicated presentation decisions, split by responsibility **within existing packages** if helpful, avoid new state machines or schema migration, preserve response/redirect/authorization contracts, and retain regression coverage.

### YM-007 — Targeted Go file/function responsibility cleanup
- **Priority:** P2 · **Stage:** post-merge refactor · **Status:** DEFERRED
- **Source:** `internal/app/resumes.go`, `internal/app/browser_session.go`, `internal/app/application_answers.go`.
- **Acceptance:** Select hotspots based on measured reading/editing friction and caller tracing; use small same-package separations, explanatory invariants, and table-driven tests. No broad domain-package extraction, browser automation replacement, or generalized repository interfaces.

## Beta acceptance — release-specific work, not all source-merge prerequisites

### YM-008 — Native Wails interactions, document export and restart
- **Priority:** P1 when preparing beta · **Stage:** beta acceptance · **Status:** NEEDS VERIFICATION
- **Acceptance:** From a **frozen candidate**, perform actual Wails keyboard/dirty-editor/dialog/focus journeys including two dirty editors, Save/Cancel native export, Unicode document import/hash, History/Next, and full app relaunch/persistence. Record observed pass/fail separately from Chromium. Cross-reference [acceptance](implementation/product-experience-implementation/ACCEPTANCE.md).
- **Dependency:** Exact candidate source and synthetic workspace.

### YM-009 — Isolated native employer transfer fixture
- **Priority:** P1 if assisted employer filling is advertised · **Stage:** beta acceptance · **Status:** DEFERRED
- **Why:** Controlled Chromium fixture transfer passes; native Wails-to-managed-Chromium path has not been accepted.
- **Acceptance:** A test-only exact loopback destination/factory with no production trust bypass verifies authorization scopes, one resume selection, contacts/eligible saved answers, unsaved answer exclusion, preserved manual values, late letter, independent results, and zero employer submission requests. Never operate against a real employer URL for the fixture.
- **Boundary:** Do not migrate CDP, add a new ATS provider, or assume a navigation-only synthetic posting is safe to fill.

### YM-010 — Accessibility and native viewport evidence
- **Priority:** P1 when preparing beta · **Stage:** beta acceptance · **Status:** NEEDS VERIFICATION
- **Acceptance:** Actual VoiceOver navigation/labels, keyboard/focus, 200% effective zoom/text scaling, reduced motion, light/dark, and 700×550 window behavior. Capture WebView `innerWidth/innerHeight` directly if feasible; AX scroll-area measurements are **not** JavaScript inner dimensions. Classify unexercised tests as NOT VERIFIED.
- **Boundary:** A test-only WebView DOM probe may supplement, but cannot prove real keyboard or VoiceOver behavior.

### YM-011 — Freeze candidate, qualify scope, and choose distribution path
- **Priority:** P1 when preparing beta · **Stage:** beta acceptance · **Status:** OPEN · **Depends on:** applicable YM-008–010
- **Source:** [release policy](RELEASE_POLICY.md), [release checklist](RELEASE_CHECKLIST.md), [release runbook](RELEASE_PROCESS.md).
- **Acceptance:** Freeze commit/build/hash, test two distinct supported preparation journeys and recovery, review Chrome/AI absence, verify privacy and no automatic submission, then select **technical alpha** with accurate limitations or **normal public installation** with Developer ID signing, notarization, Gatekeeper and a clean browser-downloaded installation test. Record owner SHIP / SHIP RESTRICTED / HOLD. Source merge is not release authorization.

## Later / deliberately separated work

### YM-012 — Investigate host-wide macOS UI automation failure
- **Priority:** P2 · **Stage:** later · **Status:** DEFERRED
- **Evidence:** [native.json](implementation/product-experience-implementation/evidence/native.json) reports `osascript` / System Events `-10827`, `kLSNoExecutableErr`, and unrelated GUI app LaunchServices failures while direct YourMove launch succeeded.
- **Acceptance:** Reproduce in a normal interactive GUI session vs failing automation environment, identify the exact failing application/invocation and repair environmental cause if worthwhile. Never claim an app defect or weaken signing/security based solely on this error.

### YM-013 — Optional test-only Wails WebView probe
- **Priority:** P2 · **Stage:** later · **Status:** DEFERRED
- **Acceptance:** If manual testing becomes repeatedly expensive, investigate opt-in restricted Wails-side DOM/JS assertions for viewport, active editor and dialog state. Prove absence from production builds and absence of arbitrary script execution or real-user data exposure. Never equate synthetic JS events with actual native keyboard accessibility acceptance.

### YM-014 — Prepare/Answers hierarchy improvement
- **Priority:** P2 · **Stage:** later / separate UX PR · **Status:** DEFERRED
- **Source:** [Direction A design](implementation/2026-10-22-product-experience/DESIGN_SPEC.md), `docs/design/DESIGN_SYSTEM.md`, `docs/design/RULES.md`.
- **Acceptance:** Make the immediate writing/preparation decision clearer based on real usage evidence; preserve the existing five job activities, one authoritative editor, local save and explicit transfer review. Separate PR after Phase 2 closeout, not part of native infrastructure work.

### YM-017 — Diagnose VoiceOver save announcements in native Wails
- **Priority:** P2 · **Stage:** later · **Status:** DEFERRED
- **Evidence:** During M1 closeout, the maintainer reported opening VoiceOver Utility but VoiceOver was not working with YourMove. No exact announcement, duplicate count, focused element, or reproduction steps were provided. The agent's automation process could not launch or query the VoiceOver runtime; this does not establish a YourMove defect.
- **Acceptance:** In an interactive macOS session with VoiceOver running, reproduce cover-letter and selected-answer local saves on an isolated synthetic workspace and exact candidate. Record the spoken phrase/count, VoiceOver focus, keyboard continuation, and executable/source identity. Distinguish a VoiceOver/runtime/permission setup issue from a Wails/WebKit status announcement or focus issue; document findings and the smallest justified follow-up.
- **Boundary:** Investigation only. No accessibility framework, success indicator, focus redesign, employer form, transfer, or submission. VoiceOver findings remain separate and do not reopen or block accepted M1.

## Backlog item template

```md
### YM-NNN — Specific outcome
- **Priority:** P1/P2/P3 · **Stage:** PR closeout/post-merge refactor/beta acceptance/later
- **Status:** OPEN / NEEDS VERIFICATION / BLOCKED / DEFERRED / IN PROGRESS / DONE
- **Source/evidence:** path, issue or PR, failing test/observation (if any)
- **Why / user impact:** concrete impact
- **Acceptance:** observable behavior, necessary tests, evidence
- **Dependencies / boundaries:** what must happen first and what not to expand
- **Completion:** commit/PR, date, exact evidence; remaining work if any
```

**Rule:** An agent asked "what next?" should not blindly choose a P1 release-gate automation task when the current requested goal is *refactoring*. Choose the smallest valuable actionable item aligned to the stated milestone while preserving safety and release obligations.

## Universal Job Recovery extension — authorized 2026-10-24

### YM-018 — Save and recover jobs across unsupported platforms
- **Priority:** P1 · **Stage:** recovery product acceptance · **Status:** IMPLEMENTED / NEEDS NATIVE VERIFICATION
- **Outcome:** First-party intake saves immediately offline and opens the description editor; pasted headings/bullets support reading navigation; manual corrections and last-success sources survive refresh/restart. Prepare retains resume selection, profile/Answer Library copying, explicit employer opening, manual completion and Next. Manual cover letters use the existing draft lifecycle without AI or fabricated source evidence.
- **Evidence/tests:** [Integrated report](implementation/job-intelligence/evidence/universal-recovery/README.md); dedicated production-handler/reopen/privacy/migration tests, controlled Chromium at 1280/700/390 light/dark with keyboard/no-JavaScript paths. No commit/PR; current branch has no PR according to live `gh pr view` lookup. No source merge/release claim.
- **Remaining acceptance:** Actual frozen Wails interaction/export/default-browser return and same-directory restart; unknown-page capture remains unavailable. Arbitrary new per-job answer questions and rich clipboard emphasis/links remain outside implemented fallback; existing saved writing is manually reusable. YM-017 remains deferred.

### YM-019 — Reviewed recovery diagnostics and evidence-to-adapter completion
- **Priority:** P1 · **Stage:** bounded learning workflow · **Status:** PARTIAL
- **Implemented:** Local metadata preview and explicitly confirmed download; hostname plus value-free URL shape, platform family, coarse retrieval outcome and recovery/origin flags. No raw errors, page samples, applicant content or automatic uploads. Dated classic iCIMS failure-to-public-variant/fixture/live-source investigation documented.
- **Remaining:** Opt-in, inspected public source sample/exact public identity package and proof that applicant-approved evidence generates an engineering fixture. Metadata-only export is useful but does not close this gate. Distinguish unknown/transient/authentication causes only when observed; never infer them from a failed request alone.
- **Evidence:** [Recovery report](implementation/job-intelligence/evidence/universal-recovery/README.md), [provider evidence](implementation/job-intelligence/evidence/universal-recovery-provider-expansion.md).

### YM-020 — Six-platform expansion qualification
- **Priority:** P2 · **Stage:** recovery-driven provider expansion · **Status:** FOUR CONTROLLED-SUPPORTED / TWO UNQUALIFIED
- **Completed:** Personio published XML, Teamtailor vendor RSS, Recruitee exempt public XML, and classic iCIMS identity-bound public JSON-LD. Eight live postings across two employers each and eleven-provider integrated controlled Overview lifecycle. Eleven total implemented controlled-supported providers; zero newly native-accepted providers.
- **Deferred/blocked:** Taleo CONDITIONAL research with one observed encoded career-section variant; Avature BLOCKED repeatable source qualification. Branded/custom domains, unavailable feeds and unqualified variants use recovery. Do not count fixture-only or research-only support.
- **Evidence/variants:** [Six-platform report](implementation/job-intelligence/evidence/universal-recovery-provider-expansion.md). Native qualification and the primary recovery gate take precedence over increasing this count.

### YM-021 — Align answer and cover-letter mutation with posting ownership
- **Priority:** P1 · **Stage:** beta acceptance · **Status:** DONE (source fix; release candidate still unqualified)
- **Outcome:** Answer and text-mode cover-letter writes recheck the existing provider-specific posting ownership policy at mutation time. Ashby descendants remain supported; Lever permits only `/apply`; Greenhouse requires the exact posting route.
- **Evidence/tests:** `internal/application/browser/chromium/mutation_ownership_qualification_test.go` checks supported and rejected routes, preserves manual values, and observes zero submit events. Selected controlled release-gate tests passed with `-tags qualification`: `TestUniversalRecoveryBrowserJourney`, `TestControlledContactJourney`, `TestControlledCombinedContinuation`, `TestControlledApplicationHandoff`, `TestHostedPlatformForms`, `TestControlledScopeFirstWorkbench`, `TestControlledSubmissionRecordingWorkflow`, and `TestContactResolverOwnershipMatchesPostingPolicy`. Full report: `$PROJECTS/yourmove-review-artifacts/v0.1.0-preaudit/run-20261024-230550/BROWSER_AUTHORITY_AUDIT_ADDENDUM.md`.
- **Boundary:** Production ownership policy was not widened; the handoff integration test now supplies its synthetic Ashby fixture identity explicitly. No commit/PR created; the change remains in the existing dirty worktree pending source review. This does not replace native Wails acceptance or qualify a frozen release artifact.

## Sol 6.1 product-experience implementation follow-up — 2026-10-25

**Baseline:** The implementation checkout matched the preserved source manifest `91bef4fba4a77b1003ca9f54327498de46cb6680617115b077ef07dae7b83519` before these edits (470 tracked and 138 untracked file hashes). Changes remain uncommitted on `feature/ashby-provider-finalization`; the source manifest must be regenerated before a reviewed checkpoint.

### UX-02 — Lead Prepare with the selected material and next boundary
- **Status:** IN PROGRESS · **Evidence:** `templates/job.html`, `static/styles.css`
- **Implemented so far:** Selected resume filename, version, Change and Download now lead Prepare. The repeated readiness grid and local transfer-scope hop were removed; exact transfer scope and its consent remain. Partial inspection and employer-side work counts remain beside the answer entry.
- **Acceptance pending:** Supported/manual/disconnected/applied/closed state captures; 700×550 and 1100×800 layout; dirty answer and letter preservation; no-JavaScript and native Wails redirect/focus checks.

### UX-04 — Use one first-job entry point
- **Status:** IN PROGRESS · **Evidence:** `templates/jobs.html`, `internal/app/handlers.go`, `internal/app/ui_pages.go`
- **Implemented so far:** Empty workspaces show one open URL-saving form with navigation to Documents. Empty-only progress and list filters are hidden. Invalid URL/title submissions re-render the form values with field errors. Existing filtered-empty controls remain for non-empty workspaces.
- **Acceptance pending:** Fresh workspace and filtered-empty captures, invalid input/correction path, batch import, and no-JavaScript/Wails behavior.

### UX-06 — Make the safe dialog choice visually dominant
- **Status:** IN PROGRESS · **Evidence:** `static/ui.js`
- **Implemented so far:** “Stay and edit” now uses the primary treatment and “Leave without saving” remains secondary; DOM order and focus handling are unchanged.
- **Acceptance pending:** Both themes, Stay, Escape, explicit Leave, Enter behavior, and native focus return.

**Verification boundary:** No tests or after-change screenshots were produced in this implementation turn. Native Wails acceptance remains NOT VERIFIED; UX-01 remains the release blocker until its specified recovery path is accepted.

## Product experience finalization — October 25, 2026

**Status: implementation and controlled-browser qualification complete; integrated native acceptance OPEN / UX HOLD.** This supersedes the unverified status of the four Luna corrections only within the executed browser/handler scope. No commit, PR, merge, tag or distribution was created. Existing branch work and AGENTS.md were preserved in a verified 608-file snapshot.

- Shared Quiet Ink palette: neutral light/graphite dark, semantic action/selection/success/error/dirty/focus roles; system typography retained. Real two-direction concepts, all 19 principal production surfaces at three browser sizes/both themes, source-pinned before/after report.
- UX01/02/04/06 revalidated and refined; UX03/05/07/08/09/10/11 implemented bounded composition/feedback improvements; UX12 canonical numeric return-to-job through Profile/Documents; UX13 bounded two-job reuse demonstrated without new matching or silent selection.
- Confirmed first-manual-letter unsaved-departure gap fixed in the existing editor guard. Stay, Escape, initial Enter, explicit Leave and same-node activity switching qualified. Approved profile revision preservation and input/consent recovery qualified.
- Final checks PASS: `go test -count=1 ./...`, desktop-tagged equivalent, both `go vet` commands, `node --check static/ui.js`, `git diff --check`; 13 selected integrated Chromium suites PASS (72.675s). Full exact commands and logs live in the finalization package.
- Remaining gates: actual native Wails Profile correction/import/export/cancel/focus/keyboard/browser return/restart; native wait responsiveness and candidate-specific download/install/CI. Diagnostic startup is not native interaction acceptance. Long-running generation probe was inconclusive; no background-response claim is made. Broader two-employer native journeys and VoiceOver remain unverified.

Evidence: the local finalization review archive contains `EXECUTIVE_REVIEW.md`, `UX_FINDINGS_FINAL.md`, `USER_JOURNEY_ACCEPTANCE.md`, `NATIVE_ACCEPTANCE.md`, and `TEST_RESULTS.md`; generated logs, screenshots, and binaries are excluded from this source PR. Source identity and a task-only diff separate this work from the inherited dirty branch. Deferred automation, arbitrary unsupported-form questions, new matching and direct unknown-page capture remain deferred.
