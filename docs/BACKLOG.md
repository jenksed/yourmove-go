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
- **Priority:** P1 · **Stage:** PR closeout · **Status:** OPEN (user-observed, source-confirmed)
- **Observation:** While editing Candidate Profile in the real Wails qualification app, saving with profile approval selected and missing first name, last name or email displayed plain text `Profile was not saved: first name, last name and email are required for profile approval` with no visible way to return to editing. The user could not recover the entered profile values from that response.
- **Source:** `internal/app/inspection_handlers.go: updateCandidate` uses `http.Error(..., 400)` for any `saveCandidateProfile` validation failure; `internal/app/candidate_profile.go` performs approval-only mandatory-field validation; `templates/candidate.html` has no inline recovery presentation.
- **Impact:** A normal incomplete-approval attempt drops the user into an unstyled error page and interrupts a primary local editing workflow. The rejected form is **not saved**; this is not evidence of database corruption or post-save persistence failure.
- **Acceptance:** On invalid approval, render Candidate Profile with the **submitted values intact**, a visible accessible validation summary tied to the relevant field(s), and working navigation/actions; permit correction and retry. Without approval, allow incomplete profiles to be saved as drafts, preserving the existing separate approval boundary. Do not silently approve or transmit facts. Exercise invalid approval, valid approved save, invalid email, unapproved draft, keyboard/focus and no-JS behavior in controlled tests and a real Wails window. Ensure the Wails-compatible response path keeps the form usable after errors; do not return an unstyled plain-text body.
- **Boundary:** Small focused validation/error-rendering fix; preserve server authority, approved profile revision semantics, no mandatory onboarding, and existing field mappings.

### YM-016 — Repeat native qualification against the **same** synthetic workspace
- **Priority:** P1 for completing YM-003 · **Stage:** PR closeout · **Status:** OPEN (procedure/documentation gap; no product persistence failure proven)
- **Observation:** The maintainer stopped `scripts/qualify-native-workflow.sh --launch` with Ctrl+C, then ran the **same script again** and saw previous changes missing.
- **Source/cause:** The script invokes `mktemp -d /private/tmp/yourmove-workflow-native-XXXXXX` **on every run**, creates a fresh `workspace`, and calls `TestPrepareNativeWorkflowFixture` to seed a fresh `jobs.db`. Restarting the script means a different database, not an app relaunch into the same workspace.
- **Acceptance:** Document the exact distinction between a **fresh qualification run** and a **persistence/restart check**. Capture the `Bundle:` and `Workspace:` paths printed by the first successful script execution. Fully stop that app process; confirm it exited; relaunch its **existing executable** with `YOURMOVE_DATA_DIR` set to the **same existing workspace**, without rerunning the seeding script. Verify a successfully saved profile/answer/letter, document selection, and one local History record persist correctly. Record actual source SHA, workspace path and observed results. Clarify that a failed 400 validation save is never persisted.
- **Safety:** Do not run two YourMove instances simultaneously against the same workspace, re-seed an existing DB, use real applicant data, or manually modify `jobs.db`. Do not add a generic production `--resume` or relaxed fixture safety check merely to simplify this test. A small script help/README note is sufficient if it prevents recurrence.
- **Boundary:** A new synthetic workspace on every new harness execution is intentional. This is primarily a test-operator documentation and repeatability correction.
- **Completion so far:** The owner smoke record now documents capturing and reusing the same bundle/workspace and launching the existing executable with the same data directory. Native execution and observed persistence evidence remain pending.

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
