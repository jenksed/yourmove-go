# Application continuity — implementation and qualification

## Final native continuity qualification — BLOCKED (2026-10-20, 18:31 EDT)

**Native qualification:** BLOCKED. **Storage isolation:** Verified for the attempted executable launch; Gate A as a whole did not pass because reliable browser/window access was unavailable. **Normal-data incident:** Assessed but unresolved.

### Gate A evidence and stopping condition

Read `AGENTS.md`, this handoff, the temporary desktop/fixture/CDP overlays, and the relevant application/storage/continuation boundaries through Graft. No product defect was reproduced and no implementation change was made.

The exact executable was `/private/tmp/yourmove-continuity-review-native/YourMoveContinuityReview.app/Contents/MacOS/YourMoveContinuityReview`, SHA-256 `2d3c6ef3da1062c826162d4626ca31f4f749089d21fb42ad874acfb7a5f668bc`, bundle identifier `local.yourmove.continuity-review`. Its startup reported Wails v3.0.0-beta.32 and build tags `desktop,qualification`. This is the existing temporary executable, not a newly rebuilt/current-source qualification claim.

Inspection of the existing temporary `desktop.go` confirmed that its exact `YOURMOVE_DATA_DIR` guard precedes `applicationStorage()`, `openDB`, and `setupNativeContinuity`. Fresh executable checks with both an unset value and `/private/tmp/yourmove-wrong-isolation` exited 1 with `qualification requires its isolated synthetic YOURMOVE_DATA_DIR; refusing default storage`. The incorrect directory was not created. The allowed data directory and its parents resolved to the expected physical paths without symlinks.

The explicitly isolated launch used `YOURMOVE_DATA_DIR=/private/tmp/yourmove-continuity-review-native/data`, the existing uniquely named `YourMoveLocalEmployer.app` executable, and Wails PID **93482**. The process tree established its Chromium child **93484**, with a fresh `yourmove-contact-fixture-1663518885` profile and `--app=http://127.0.0.1:58333/native/job/application`. The UI tool listed the intended Wails bundle as running but did not list the uniquely named employer bundle. No window was selected or operated.

The isolated profile's `DevToolsActivePort` recorded **58348**, but a read-only `/json/list` request returned connection refused. The existing fixture observer repeatedly reported the same refusal. The child browser exited before shutdown inspection. This reproduces the existing qualification-environment blocker with the uniquely named browser; it does not demonstrate a production continuity failure or establish its cause. Other Chrome-for-Testing processes included live employer sessions, so using the shared browser identity could not establish safe window ownership. No fallback UI targeting or upload was attempted.

Stopped at Gate A. Only the positively identified Wails process from this attempt was terminated; unrelated applications, earlier orphan sessions, files, worktrees, and artifacts were preserved.

### Normal-data incident assessment

The protected after-incident snapshot passed SQLite `quick_check`. Read-only inspection confirmed the synthetic candidate profile, `native-master.pdf` (resume 4, master), `native-role.pdf` (resume 5), and job 1 selecting resume 5. These are after-incident facts, not authoritative prior values. No backup candidate was present in the normal storage directory; no authoritative pre-incident profile or selection values were established. No speculative recovery, deletion, or normal-storage write was performed.

A private SHA-256 manifest taken before guard checks and launch was compared after shutdown: **all six normal-storage files were byte-identical**, with zero added, removed, or changed files. This protects the normal candidate profile, selections, jobs, and application records against mutations by this attempt; it does not resolve the earlier incident.

### Observed workflow and transfer evidence

No native UI interaction occurred: no Prepare activation, Fill application request, manual correction, generation consent, generation, editing, saving, continuation, or workspace return. The browser endpoint was inaccessible, so fresh fixture operation counters and browser instance identity were **not observable**. Do not report counters as zero merely because no action was requested. The isolated database still had zero cover-letter and submission records; these database facts do not replace fixture counters. The previous **3 contacts / 1 resume / 0 letters / 0 consent changes / 0 submissions** remain historical evidence only.

Same-session continuity, retained manual correction, edited-letter identity, exact attachment selection, and workspace return remain freshly unqualified. No claim of employer-side acceptance or live ATS qualification is made.

### Changes, verification, and next action

The only repository file edited by this attempt is this handoff. No source, template, dependency, database, or harness source change was made. Fresh checks passed: unset/wrong-directory guard checks, snapshot integrity, normal-storage byte comparison, and `git diff --check`. Full Go, desktop, race, and controlled-browser suites were not rerun because no implementation changed and execution stopped at the required environmental gate; earlier suite results below are historical.

Private attempt evidence remains under `/private/tmp/yourmove-continuity-review-native/`: `qualification-final-launch.json`, `qualification-final-launch.log`, `qualification-final-process-evidence.txt`, `qualification-final-normal-before.json`, `qualification-final-storage-verification.json`, and `qualification-final-status-before.txt`. The normal-storage manifest and incident database stay outside repository artifacts. The launch log contains the isolated process's actual fixture-observer failures. No fresh UI screenshots were captured.

**Smallest next qualification action:** restore a working, uniquely targetable localhost Chromium fixture whose CDP endpoint remains available and whose native window maps positively to its isolated process. Then rerun Gate A and the complete fresh native journey through ordinary Wails controls. Do not reuse shared Chrome window targeting or treat this blocked run as a pass.

**Next recommended product task:** Prepare workspace design, application-question usability, and cover-letter editing improvements. Those changes were not started in this assignment.


## Fresh native qualification follow-up — blocked (2026-10-20)

The fresh native journey remains **Blocked**, not passed. The accepted continuity implementation was not changed. Rod remains removed; no dependency changes or module tidy were performed during this qualification.

### Observed native actions and fixture evidence

The isolated Wails build used the existing desktop handlers, deterministic generation seam, synthetic profile/documents, and localhost Chromium fixture. Prepare displayed `native-role.pdf`, the synthetic approved contact facts, and one **Fill application** action without a generic approval checkbox. The first automation interaction scrolled the button into view without a POST; the subsequent visible activation produced exactly one `POST /jobs/1/continue`, followed by the normal Prepare redirect. The native UI reported the selected resume and three completed contact fields.

Read-only fixture observations in `first-fill-window-selection.log` recorded:

- Three contact fills; one resume selection; zero cover-letter selections; zero employer consent changes; zero submissions.
- Browser instance `7a75dac6-04a7-4735-9903-4e7d4163c7e0`, localhost port 57880, application `/native/job/application`.
- Selected synthetic resume: 688 bytes, SHA-256 `06ffaf8d63b2eb670f22652ea5124f740e4321aea73e13d5e140051bd5d82d16`.
- No repeated resume selection was observed in that run.

This run did **not** reach manual employer correction, generation/edit/save, letter continuation, or final navigation persistence. Those steps and same-session continuity are not freshly qualified. Historical native and controlled Chromium evidence below remains distinct.

### Concrete blockers and local-storage incident

The UI tool selected an older unrelated Chrome process when given the managed Chrome app path. No actions were performed on that live form. A temporary uniquely named browser wrapper subsequently lost its debugging endpoint (connection refused). That observation is not a captured YourMove crash report. Retrying installed Chromium 1200 timed out; the original installed Chromium 1248 again served the localhost fixture successfully, initially with all counters zero. It was stopped without another transfer.

During recovery, requesting the Wails app state when no native window was available caused the UI tool to automatically launch another copy **without** `YOURMOVE_DATA_DIR`. The temporary harness then ran its startup setup against normal local storage. The unexpected non-fixture job in that window was inspected, with no form interaction, and the copy was stopped. Read-only inspection confirmed that the synthetic profile and two synthetic resumes were written to normal storage, changing the master-resume and job-1 resume selections. Prior candidate/profile and selection values are not established by the available evidence; no speculative restoration or deletion was attempted. This is a qualification-harness isolation failure, not evidence of a production continuity defect. It requires recovery review before further native qualification.

A protected local after-incident database snapshot was saved under `/private/tmp/yourmove-continuity-review-native/normal-storage-after-accidental-launch.db` for recovery. It contains private workspace data and must not be copied into the repository or published as synthetic evidence. No pre-incident backup was identified in this attempt.

The **temporary** `desktop.go` now refuses to start unless `YOURMOVE_DATA_DIR` exactly equals `/private/tmp/yourmove-continuity-review-native/data`, before opening storage. The qualification binary was rebuilt successfully and its unset-environment launch was checked: it exits with the isolation-required message. This guard does not change production source or application behavior. Both qualification processes were stopped; unrelated applications were left running.

### Verification and exact resumption point

Executed successfully: the temporary native `go build -tags desktop,qualification` overlay build, its subsequent fail-closed rebuild, and `git diff --check`. The Chromium-1200 native launch failed with `context deadline exceeded`; the original-Chromium launch produced successful read-only localhost observations but no additional native transfer. No full/race/desktop test suite was rerun because production implementation was unchanged. Earlier passing suites below are historical results.

Temporary overlay/build/evidence paths are under `/private/tmp/yourmove-continuity-review-native/`: `overlay-completion.json`, `fixture-completion.go`, `qualification-visible.go`, `desktop.go`, `first-fill-window-selection.log`, `completion-run.log`, `resumed-native.log`, `resumed-original.log`, and `evidence/01-prepare-before-fill.*` through `03-after-fill.*`.

Before resuming: review recovery of the normal local profile and resume selections, and establish reliable UI targeting of the isolated Wails and localhost employer processes. Do not reopen the temporary app through a UI tool while its explicit environment is absent. Once those blockers are resolved, launch:

```sh
YOURMOVE_CHROMIUM='/Users/jenksed/Library/Caches/ms-playwright/chromium-1248/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing' \
YOURMOVE_DATA_DIR=/private/tmp/yourmove-continuity-review-native/data \
/private/tmp/yourmove-continuity-review-native/YourMoveContinuityReview.app/Contents/MacOS/YourMoveContinuityReview \
> /private/tmp/yourmove-continuity-review-native/resume-after-recovery.log 2>&1
```

The original filled browser has been closed: do not claim its browser-only progress survived. Start a fresh synthetic browser journey, inspect Prepare scope, activate Fill once, then continue at manual correction → generation consent → generate/edit/save → verify zero premature letter selections → attach once → navigate away/back. Final upload counts, preservation of corrections, and zero submissions must still be observed.

The qualification work edited only this handoff document in the repository. Compared with the earlier file-hash snapshot, `go.mod` also differs following the intervening dependency work; it was not edited or reverted by this qualification, and the current module file has no Rod requirement. Final Git status contains 52 modified tracked files and 38 untracked status entries; its complete file list is saved at `/private/tmp/yourmove-continuity-review-native/final-git-status.txt`. Existing tracked modifications and untracked work, including pre-existing `go.mod`/`go.sum` changes, were preserved. No commit, merge, reset, discard, or cleanup was performed.

## Follow-up review: paused at the user's request

The checkout already contained the combined action implementation and stale-resume pointer guard described below. This follow-up preserved those changes and unrelated work rather than rebuilding the feature.

Fresh controlled runs used the normal generation/save/continuation handlers and real local Chromium. Both `TestControlledContactJourney` and `TestControlledCombinedContinuation` passed: one initial Fill application action, one late-letter or combined continuation action, no generic transfer checkbox, one resume upload, preserved manual corrections, conditional-answer continuation, and zero submissions. No live employer was contacted. The focused race run also passed, including the deterministic old-result/close/reopen regression.

The baseline exposed a real 390px regression after long public attachment filenames appeared: a 419px unbroken filename overflowed Prepare. A scoped `overflow-wrap:anywhere` and `min-width:0` fix on `.prepare-handoff` resolved it. The qualification screenshot helper now reports overflowing elements. Wide and narrow screenshots were captured and inspected; both complete controlled journeys subsequently passed.

A fresh isolated native Wails build launched with synthetic data and a localhost employer fixture. Native navigation reached Prepare and displayed the exact resume/contact scope and a single Fill application button without an approval checkbox. The user requested stopping before this new native run executed fill, generation, save, and continuation. Do not count this follow-up as fresh native end-to-end transfer verification; the older completed native evidence below remains historical evidence.

Fresh evidence: `/private/tmp/yourmove-continuity-review-before.log`, `/private/tmp/yourmove-continuity-review-after.log`, `/private/tmp/yourmove-continuity-review-after/`, `/private/tmp/yourmove-continuity-focused-race.log`, `/private/tmp/yourmove-continuity-review-tests.log`, `/private/tmp/yourmove-continuity-review-desktop-tests.log`. The isolated native overlay, app bundle, synthetic data, and run log are in `/private/tmp/yourmove-continuity-review-native/`.

Next: complete that synthetic native Fill application → generate/edit/save letter → Attach cover letter and continue journey, checking the fixture counters and the preserved browser. No new provider expansion or live-employer test is needed to finish this task.

## Product result

Prepare now presents one scoped POST action above optional writing and question editors:

- **Fill application** includes the displayed selected resume, approved contacts, selected saved letter, and pending supported answers when available.
- **Attach cover letter and continue** includes only the displayed saved letter in an active application.
- **Continue application** combines a pending letter and pending answers.
- **Fill saved answers and continue** includes only pending supported answers.

The employer/job, exact resume, letter text/version, and actual pending answer values appear beside the action. Ordinary transfer checkboxes were removed from Prepare and the inspection handoff. External generation consent, destructive restart/deletion consent, profile approval, and personal submission recording remain separate.

### Reproduced baseline

Before product changes, `TestControlledContactJourney` was changed to generate and edit/save a letter through the normal handlers and navigation. The generation backend was deterministic; no letter was inserted through SQLite. Local fixtures emulated Ashby controls in real Chromium.

The baseline succeeded technically: the original session survived, the letter was discovered, its transfer action was eligible, and existing resume status did not block letter transfer. The page did not require a restart, but its resume-oriented summary and low placement of the letter action obscured continuation.

| Measured interaction | Original | Revised |
| --- | --- | --- |
| Initial setup from unassigned resume | 5 actions: select, save, Prepare, checkbox, transfer | 4 actions: select, save, Prepare, Fill application |
| Initial transfer once material is displayed | Checkbox + button | One button |
| Saved late-letter transfer | Checkbox + button | One Attach cover letter and continue button |
| Saved-answer transfer | Checkbox + button | One button |
| Generic transfer approval checkboxes | One for each subsystem | Zero |

Generation still requires its distinct consent and Generate action. Editing and saving remain local actions. Those interactions were excluded from the transfer-only counts in both versions. UI automation sometimes needed a separate scroll-to-visible operation; these are not counted as application authorization actions.

### Demonstrated continuity

The final controlled Chromium run generated/saved a late letter, attached it in the original session, saved/transferred four answers, and then saved/transferred a newly exposed conditional answer. It observed one resume upload, preserved a manual last-name correction, and recorded zero submissions. Saving/generation left the employer letter and answer controls unchanged until an explicit transfer action.

A second controlled Chromium journey saved both a new letter and an answer, then transferred both through one **Continue application** action. It preserved earlier contacts, the resume and a manual correction, continued through conditional questions, and recorded zero submissions. The combined operation was observed in 492 milliseconds.

Final measured initial transfer: 1.585 seconds to observation, 2.568 seconds to visible result. Late letter: 64 milliseconds to observation in the open session. Four answers: 1.626 seconds. These are fixture measurements, not employer performance guarantees.

Users still inspect attachment acceptance, complete unsupported/sensitive/ambiguous controls, advance employer steps, and submit personally. An existing or uncertain attachment is inspected rather than automatically replaced. Browser loss explicitly warns about potentially lost browser-only work; reopening is distinct from continuation.

## Engineering quality and ownership

The implementation reuses `HandoffPresentation`, `summarizeHandoff`, `browserRunState`, `jobWorkspace`, `questionPreparation`, `resumeHandoff`, `coverLetterTransfer`, `applicationAnswerTransfer`, the existing watcher, and Chromium's serialized mutation path.

`application_continuation.go` adds local composition and a pure read-only preparation summary. Each existing transfer handler delegates to its material-specific command, which retains destination, identity, hash/revision, eligibility, session, and current-value checks. Only explicitly included operations run. Rejection notices identify the affected item; other eligible commands remain independent. The response does not imply atomic completion.

No named production type, interface, package, table, worker, state map, persistent approval record, or dependency was added. Existing presentation fields carry the action and displayed scope. Existing answer operations retain session-only attempted key/value snapshots and outcomes until close, preventing unchanged successful or uncertain fields from being replayed during later batches. Profile-derived contacts are excluded when the contact operation already owns them. Pending-answer hashes cover precisely the displayed pending subset, rather than all saved answers.

Two test-only types have distinct purposes: `delayedResumeBrowser` controls close/reopen result timing; `continuationBrowser` exercises the existing optional capabilities together with deterministic local failures.

The deterministic regression reproduced the audit's resume race in all four result paths: selected, waiting, preflight failure, and uncertain. `resumeResult` and `resumePreflightFailure` now require the original handoff pointer to match. Old watcher cleanup also cannot remove a replacement session's cancellation ownership. No concurrency framework or second browser writer was introduced.

An additional resume guard rejects an existing managed browser for an earlier destination. Wails' redirect allowlist now accepts canonical local Prepare URLs containing the selected draft and bounded per-item notices, retaining strict path, origin, query-key, and duplicate-key restrictions.

State ownership is unchanged. Durable preparation remains in SQLite; transfer authority and outcomes remain with the current session's existing operations; next actions are derived afresh. The added complexity is the small composition handler, pending-answer filtering, and narrowly extended native redirect handling. This replaces duplicated action selection in three UI sections without introducing a packet or approval subsystem.

## Regression evidence

- Combined initial scope contains visible exact materials and succeeds without `confirm=yes`.
- Stale letter and stale answer rejection leave unrelated included operations eligible.
- Omitted material is never armed, even when saved and ready.
- An uncertain letter outcome leaves answer success intact and is not replayed.
- Unchanged prior answers are excluded from later batches and their scope hashes.
- Foreign requests and wrong destinations are rejected.
- Deterministic old resume results cannot modify a replacement session.
- Closed/reopened and mismatched-browser destinations remain explicit boundaries.
- Canonical Wails continuation URLs work; foreign, duplicated, malformed, or unexpected query destinations are rejected.
- Existing Chromium tests retain document hashes, existing/manual values, ambiguity abstention, native choice identity, conditional questions, and no automatic submission.

## Verification

Passed using `GOCACHE=/private/tmp/yourmove-go-build`:

```sh
go test ./...
go test -tags desktop ./...
go test -race ./internal/app ./internal/application/browser/chromium
YOURMOVE_QUALIFY_BROWSER=1 go test -tags qualification ./internal/app -run 'TestControlled(ContactJourney|CombinedContinuation)' -v -count=1
YOURMOVE_QUALIFY_BROWSER=1 go test ./internal/application/browser/chromium -run TestControlledApplicationHandoff -v -count=1
git diff --check
graft build
```

Local fixture listeners and native launches required approved execution outside the filesystem/network sandbox. No external employer or AI provider was contacted. The document primitive test independently observed one resume selection, one cover selection, Unicode text rerender/readback, preserved manual content, and zero submissions.

### Native Wails evidence

An isolated macOS Wails app bundle used the real desktop mux, templates, POST handlers, and redirect adapter. A temporary build overlay supplied synthetic documents/profile/job, a localhost Chromium session adapter, and the existing deterministic generation test seam. Its database was precreated empty to avoid legacy-data migration. The harness was removed from the repository after the build; its source and overlay remain in `/private/tmp` for inspection.

Through native UI interaction, the run completed Prepare → Fill application → open writing → distinct generation consent → Generate → edit → Save my edits → Attach cover letter and continue. It then left for Opportunities and returned to Prepare. The edited draft and completed transfers remained visible. Read-only fixture counters ended at **3 contact fills, 1 resume selection, 1 cover selection, 0 submissions**; generation/save produced zero cover selections before the explicit continuation POST. No restart occurred.

A repeated automation click was initially rejected by automatic approval review as a possible uncertain upload retry. Inspection established that the first interaction only scrolled: no letter POST and zero cover selections. The subsequently permitted activation produced the single letter transfer. No uncertain upload was retried.

The native run qualifies the contact/resume and late-letter save/continuation path. Native answer editing/combined multi-item transfer, final minor summary wording, and subsequent pending-answer hash/destination hardening were covered by automated handler/Chromium tests rather than another full native run. Native 390px behavior was not claimed; the desktop window's minimum width is 700px. Controlled browser screenshots were inspected at 1280px and 390px in dark appearance.

Logs and screenshots:

- `/private/tmp/continuity-baseline.log`, `/private/tmp/yourmove-continuity-baseline/`
- `/private/tmp/continuity-all-final.log`, `/private/tmp/continuity-desktop-final.log`, `/private/tmp/continuity-race-final.log`
- `/private/tmp/continuity-revised-final.log`, `/private/tmp/continuity-combined-final.log`, `/private/tmp/continuity-documents-final.log`, `/private/tmp/yourmove-continuity-revised/`
- `/private/tmp/continuity-race-baseline.log` (deterministic failing race reproduction)
- `/private/tmp/continuity-native-clean.log`, `/private/tmp/continuity-native-complete.png`
- `/private/tmp/native_continuity_qualification.go`, `/private/tmp/continuity-overlay.json`, `/private/tmp/desktop-continuity.go`

### Limits

No new live-provider verification was authorized or performed. Browser File selection/content and local receipt do not prove employer-side upload acceptance. Unsupported controls and PDF glyph/format limitations remain manual. Real employer acceptance and universal Ashby compatibility remain unverified. No employer application was submitted.

The supplied architecture review, primitive catalog and CSV describe the pre-continuity snapshot; their relevant operation/presentation declarations matched the checkout. They are retained as audit artifacts rather than rewritten as a current inventory. All unrelated pre-existing uncommitted work remains in place; no commit was created.
